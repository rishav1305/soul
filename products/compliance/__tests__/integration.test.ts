/**
 * Integration tests for @soul/compliance
 *
 * These tests exercise the full compliance pipeline end-to-end:
 *   1. Scan vulnerable fixture  → assert findings
 *   2. Scan compliant fixture   → assert minimal/clean results
 *   3. Generate JSON report     → parse and validate structure
 *   4. Generate badge SVG       → check SVG validity
 *   5. Generate fix patches     → verify patches exist (dry-run)
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { mkdtempSync, rmSync, cpSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';

import { runScan } from '../src/tools/scan.js';
import { formatJSON } from '../src/reporters/json.js';
import { formatTerminal } from '../src/reporters/terminal.js';
import { generateBadge, calculateScore } from '../src/reporters/badge.js';
import { generateHtml } from '../src/reporters/html.js';
import { loadRules } from '../src/rules/index.js';
import { runFix } from '../src/tools/fix.js';
import type { ScanResult, Finding } from '../src/types.js';

const fixturesDir = join(import.meta.dirname, 'fixtures');
const vulnerableDir = join(fixturesDir, 'vulnerable-app');
const compliantDir = join(fixturesDir, 'compliant-app');

// ── 1. Full scan on vulnerable fixture ──────────────────────────────────

describe('Integration: vulnerable app scan', () => {
  let result: ScanResult;

  beforeEach(async () => {
    result = await runScan({ directory: vulnerableDir });
  });

  it('produces findings for the vulnerable app', () => {
    expect(result.findings.length).toBeGreaterThan(0);
    expect(result.summary.total).toBe(result.findings.length);
  });

  it('detects critical severity findings', () => {
    expect(result.summary.bySeverity.critical).toBeGreaterThan(0);
  });

  it('runs all 5 analyzers successfully', () => {
    expect(result.metadata.analyzersRun.length).toBe(5);
    expect(result.metadata.analyzersRun).toContain('secret-scanner');
    expect(result.metadata.analyzersRun).toContain('config-checker');
    expect(result.metadata.analyzersRun).toContain('ast-analyzer');
    expect(result.metadata.analyzersRun).toContain('git-analyzer');
    expect(result.metadata.analyzersRun).toContain('dep-auditor');
  });

  it('populates metadata correctly', () => {
    expect(result.metadata.directory).toBe(vulnerableDir);
    expect(result.metadata.duration).toBeGreaterThanOrEqual(0);
    expect(result.metadata.frameworks).toEqual(['soc2', 'hipaa', 'gdpr']);
    expect(result.metadata.timestamp).toBeTruthy();
    // Timestamp should be a valid ISO date
    expect(new Date(result.metadata.timestamp).getTime()).not.toBeNaN();
  });

  it('has consistent summary counts', () => {
    const { bySeverity, byFramework, byAnalyzer, fixable, total } = result.summary;

    // Sum of bySeverity must equal total
    const severitySum =
      bySeverity.critical + bySeverity.high + bySeverity.medium + bySeverity.low + bySeverity.info;
    expect(severitySum).toBe(total);

    // fixable must be <= total
    expect(fixable).toBeLessThanOrEqual(total);

    // byAnalyzer sum must equal total
    const analyzerSum = Object.values(byAnalyzer).reduce((a, b) => a + b, 0);
    expect(analyzerSum).toBe(total);
  });

  it('each finding has required fields', () => {
    for (const finding of result.findings) {
      expect(finding.id).toBeTruthy();
      expect(finding.title).toBeTruthy();
      expect(finding.description).toBeTruthy();
      expect(['critical', 'high', 'medium', 'low', 'info']).toContain(finding.severity);
      expect(finding.framework.length).toBeGreaterThan(0);
      expect(finding.analyzer).toBeTruthy();
      expect(typeof finding.fixable).toBe('boolean');
    }
  });

  it('findings are deduplicated (unique file:line:id)', () => {
    const keys = result.findings.map(
      (f) => `${f.file ?? ''}:${f.line ?? 0}:${f.id}`,
    );
    expect(new Set(keys).size).toBe(keys.length);
  });

  it('detects hardcoded secrets', () => {
    const secretFindings = result.findings.filter((f) => f.analyzer === 'secret-scanner');
    expect(secretFindings.length).toBeGreaterThan(0);
  });

  it('detects weak cryptography', () => {
    const cryptoFindings = result.findings.filter(
      (f) => f.analyzer === 'ast-analyzer' && f.title.toLowerCase().includes('hash'),
    );
    expect(cryptoFindings.length).toBeGreaterThan(0);
  });
});

// ── 2. Full scan on compliant fixture ───────────────────────────────────

describe('Integration: compliant app scan', () => {
  let result: ScanResult;

  beforeEach(async () => {
    result = await runScan({ directory: compliantDir });
  });

  it('produces no critical findings', () => {
    const criticals = result.findings.filter((f) => f.severity === 'critical');
    expect(criticals.length).toBe(0);
  });

  it('runs all 5 analyzers', () => {
    expect(result.metadata.analyzersRun.length).toBe(5);
  });

  it('has consistent summary', () => {
    expect(result.summary.total).toBe(result.findings.length);
  });
});

// ── 3. JSON report generation and validation ────────────────────────────

describe('Integration: JSON report from scan', () => {
  it('produces valid, round-trippable JSON from a real scan', async () => {
    const result = await runScan({ directory: vulnerableDir });
    const json = formatJSON(result);

    // Should be valid JSON
    const parsed: ScanResult = JSON.parse(json);

    // Top-level structure
    expect(parsed).toHaveProperty('findings');
    expect(parsed).toHaveProperty('summary');
    expect(parsed).toHaveProperty('metadata');

    // Counts match
    expect(parsed.findings.length).toBe(result.findings.length);
    expect(parsed.summary.total).toBe(result.summary.total);
    expect(parsed.metadata.directory).toBe(vulnerableDir);

    // Deep equality
    expect(parsed).toEqual(result);
  });

  it('produces pretty-printed JSON with indentation', async () => {
    const result = await runScan({ directory: vulnerableDir });
    const json = formatJSON(result);

    expect(json).toContain('\n');
    expect(json).toContain('  ');
    expect(json.split('\n').length).toBeGreaterThan(10);
  });

  it('preserves all finding fields through serialization', async () => {
    const result = await runScan({ directory: vulnerableDir });
    const json = formatJSON(result);
    const parsed: ScanResult = JSON.parse(json);

    for (const finding of parsed.findings) {
      expect(finding.id).toBeTruthy();
      expect(finding.title).toBeTruthy();
      expect(finding.severity).toBeTruthy();
      expect(finding.framework).toBeInstanceOf(Array);
      expect(finding.analyzer).toBeTruthy();
    }
  });
});

// ── 4. Badge generation from real scan ──────────────────────────────────

describe('Integration: badge generation from scan', () => {
  it('generates valid SVG from vulnerable app scan', async () => {
    const result = await runScan({ directory: vulnerableDir });
    const totalRules = loadRules().length;
    const svg = generateBadge(result, totalRules);

    // Must be valid SVG
    expect(svg).toContain('<svg');
    expect(svg).toContain('</svg>');
    expect(svg).toContain('xmlns="http://www.w3.org/2000/svg"');

    // Must contain score percentage
    const score = calculateScore(result.summary.total, totalRules);
    expect(svg).toContain(`${score}%`);

    // Must contain framework labels
    expect(svg).toContain('SOC2');
    expect(svg).toContain('HIPAA');
    expect(svg).toContain('GDPR');
  });

  it('generates valid SVG from compliant app scan', async () => {
    const result = await runScan({ directory: compliantDir });
    const totalRules = loadRules().length;
    const svg = generateBadge(result, totalRules);

    expect(svg).toContain('<svg');
    expect(svg).toContain('</svg>');

    // Compliant app should have a higher score
    const score = calculateScore(result.summary.total, totalRules);
    const vulnerableResult = await runScan({ directory: vulnerableDir });
    const vulnerableScore = calculateScore(vulnerableResult.summary.total, totalRules);
    expect(score).toBeGreaterThanOrEqual(vulnerableScore);
  });

  it('badge color reflects score thresholds', async () => {
    const result = await runScan({ directory: vulnerableDir });
    const totalRules = loadRules().length;
    const score = calculateScore(result.summary.total, totalRules);
    const svg = generateBadge(result, totalRules);

    if (score > 80) {
      expect(svg).toContain('#4c1'); // green
    } else if (score >= 60) {
      expect(svg).toContain('#dfb317'); // yellow
    } else {
      expect(svg).toContain('#e05d44'); // red
    }
  });
});

// ── 5. Terminal reporter from real scan ─────────────────────────────────

describe('Integration: terminal reporter from scan', () => {
  it('formats vulnerable scan results with severity sections', async () => {
    const result = await runScan({ directory: vulnerableDir });
    const output = formatTerminal(result);

    // Should contain at least one severity section
    expect(output).toMatch(/CRITICAL|HIGH|MEDIUM|LOW|INFO/);

    // Should contain the directory
    expect(output).toContain(vulnerableDir);

    // Should contain the summary
    expect(output).toContain('finding');
  });

  it('formats compliant scan with clean message when no findings', async () => {
    const result = await runScan({ directory: compliantDir });

    if (result.findings.length === 0) {
      const output = formatTerminal(result);
      expect(output).toContain('No findings');
    }
  });
});

// ── 6. HTML report from real scan ───────────────────────────────────────

describe('Integration: HTML report from scan', () => {
  it('generates valid HTML document from real scan', async () => {
    const result = await runScan({ directory: vulnerableDir });
    const totalRules = loadRules().length;
    const score = calculateScore(result.summary.total, totalRules);
    const html = generateHtml(result, score);

    // Basic HTML structure
    expect(html).toContain('<!DOCTYPE html>');
    expect(html).toContain('<html');
    expect(html).toContain('</html>');
    expect(html).toContain('<head>');
    expect(html).toContain('<body>');

    // Contains branding
    expect(html).toContain('Soul');

    // Contains scan data
    expect(html).toContain('Executive Summary');
    expect(html).toContain(vulnerableDir);
  });
});

// ── 7. Fix patches in dry-run mode ──────────────────────────────────────

describe('Integration: fix patches (dry-run)', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'integration-fix-'));
    cpSync(vulnerableDir, tmpDir, { recursive: true });
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('generates patches for vulnerable app without modifying files', async () => {
    const fixResult = await runFix(tmpDir, true);

    expect(fixResult.applied).toBe(false);
    expect(fixResult.summary.totalFixable).toBeGreaterThan(0);
    expect(fixResult.summary.patchesGenerated).toBeGreaterThan(0);
    expect(fixResult.summary.patchesApplied).toBe(0);

    // All patches should contain valid unified diff markers
    for (const patch of fixResult.patches) {
      expect(patch.patch).toContain('---');
      expect(patch.patch).toContain('+++');
      expect(patch.patch).toContain('@@');
      expect(patch.findingId).toBeTruthy();
      expect(patch.file).toBeTruthy();
      expect(patch.description).toBeTruthy();
    }

    // Verify files are unmodified
    const origCrypto = readFileSync(join(vulnerableDir, 'crypto.ts'), 'utf-8');
    const curCrypto = readFileSync(join(tmpDir, 'crypto.ts'), 'utf-8');
    expect(curCrypto).toBe(origCrypto);

    const origConfig = readFileSync(join(vulnerableDir, 'config.ts'), 'utf-8');
    const curConfig = readFileSync(join(tmpDir, 'config.ts'), 'utf-8');
    expect(curConfig).toBe(origConfig);
  });

  it('patches address known vulnerability categories', async () => {
    const fixResult = await runFix(tmpDir, true);
    const descriptions = fixResult.patches.map((p) => p.description);

    // Should include at least weak hash fixes or secret fixes
    const hasWeakHashFix = descriptions.some((d) => d.includes('SHA-256'));
    const hasSecretFix = descriptions.some((d) => d.includes('process.env'));
    expect(hasWeakHashFix || hasSecretFix).toBe(true);
  });
});

// ── 8. Fix patches apply mode ───────────────────────────────────────────

describe('Integration: fix patches (apply)', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'integration-fix-apply-'));
    cpSync(vulnerableDir, tmpDir, { recursive: true });
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('applies patches and modifies files', async () => {
    const fixResult = await runFix(tmpDir, false);

    expect(fixResult.applied).toBe(true);
    expect(fixResult.summary.patchesApplied).toBeGreaterThan(0);

    // Verify at least crypto.ts was fixed (MD5 → SHA-256)
    const cryptoContent = readFileSync(join(tmpDir, 'crypto.ts'), 'utf-8');
    expect(cryptoContent).toContain("createHash('sha256')");
    expect(cryptoContent).not.toMatch(/createHash\s*\(\s*['"]md5['"]\s*\)/);
  });

  it('re-scan after fix shows fewer findings', async () => {
    // Scan before fix
    const before = await runScan({ directory: tmpDir });

    // Apply fixes
    await runFix(tmpDir, false);

    // Scan after fix
    const after = await runScan({ directory: tmpDir });

    // Should have fewer findings after applying fixes
    expect(after.summary.total).toBeLessThan(before.summary.total);
  });
});

// ── 9. Framework filtering integration ──────────────────────────────────

describe('Integration: framework filtering', () => {
  it('filters to HIPAA-only findings', async () => {
    const result = await runScan({
      directory: vulnerableDir,
      frameworks: ['hipaa'],
    });

    expect(result.findings.every((f) => f.framework.includes('hipaa'))).toBe(true);
    expect(result.metadata.frameworks).toEqual(['hipaa']);
  });

  it('filters to GDPR-only findings', async () => {
    const result = await runScan({
      directory: vulnerableDir,
      frameworks: ['gdpr'],
    });

    expect(result.findings.every((f) => f.framework.includes('gdpr'))).toBe(true);
  });

  it('combined framework scan includes both', async () => {
    const result = await runScan({
      directory: vulnerableDir,
      frameworks: ['soc2', 'hipaa'],
    });

    expect(
      result.findings.every(
        (f) => f.framework.includes('soc2') || f.framework.includes('hipaa'),
      ),
    ).toBe(true);
  });
});

// ── 10. End-to-end pipeline ─────────────────────────────────────────────

describe('Integration: full pipeline (scan → report → badge → fix)', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'integration-pipeline-'));
    cpSync(vulnerableDir, tmpDir, { recursive: true });
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('executes full pipeline end-to-end', async () => {
    // Step 1: Scan
    const scanResult = await runScan({ directory: tmpDir });
    expect(scanResult.findings.length).toBeGreaterThan(0);
    expect(scanResult.metadata.analyzersRun.length).toBe(5);

    // Step 2: Generate JSON report
    const json = formatJSON(scanResult);
    const parsed = JSON.parse(json) as ScanResult;
    expect(parsed.summary.total).toBe(scanResult.summary.total);

    // Step 3: Generate badge
    const totalRules = loadRules().length;
    const svg = generateBadge(scanResult, totalRules);
    expect(svg).toContain('<svg');
    expect(svg).toContain('</svg>');

    // Step 4: Generate HTML report
    const score = calculateScore(scanResult.summary.total, totalRules);
    const html = generateHtml(scanResult, score);
    expect(html).toContain('<!DOCTYPE html>');
    expect(html).toContain('Executive Summary');

    // Step 5: Generate terminal report
    const terminal = formatTerminal(scanResult);
    expect(terminal).toContain('Soul Compliance Scan Results');

    // Step 6: Dry-run fix
    const dryResult = await runFix(tmpDir, true, scanResult);
    expect(dryResult.summary.patchesGenerated).toBeGreaterThan(0);
    expect(dryResult.applied).toBe(false);

    // Step 7: Apply fix
    const fixResult = await runFix(tmpDir, false);
    expect(fixResult.applied).toBe(true);
    expect(fixResult.summary.patchesApplied).toBeGreaterThan(0);

    // Step 8: Re-scan and verify improvement
    const afterScan = await runScan({ directory: tmpDir });
    expect(afterScan.summary.total).toBeLessThan(scanResult.summary.total);

    // Step 9: Verify improved badge score
    const afterScore = calculateScore(afterScan.summary.total, totalRules);
    expect(afterScore).toBeGreaterThanOrEqual(score);
  });
});
