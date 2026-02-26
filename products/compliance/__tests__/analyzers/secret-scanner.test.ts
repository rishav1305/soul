import { describe, it, expect } from 'vitest';
import { join } from 'node:path';
import { SecretScanner } from '../../src/analyzers/secret-scanner.js';
import { loadRules } from '../../src/rules/index.js';
import { scanDirectory } from '@soul/context';

const fixturesDir = join(import.meta.dirname, '..', 'fixtures');

describe('SecretScanner', () => {
  const scanner = new SecretScanner();
  const rules = loadRules();

  it('detects at least 5 secrets in the vulnerable app', async () => {
    const vulnerableDir = join(fixturesDir, 'vulnerable-app');
    const files = await scanDirectory(vulnerableDir);
    const findings = await scanner.analyze(files, rules);

    expect(findings.length).toBeGreaterThanOrEqual(5);
    expect(findings.every((f) => f.analyzer === 'secret-scanner')).toBe(true);
    expect(findings.some((f) => f.id === 'SECRET-001')).toBe(true);
  });

  it('produces no findings in the compliant app', async () => {
    const compliantDir = join(fixturesDir, 'compliant-app');
    const files = await scanDirectory(compliantDir);
    const findings = await scanner.analyze(files, rules);

    expect(findings).toHaveLength(0);
  });

  it('redacts all evidence with ****', async () => {
    const vulnerableDir = join(fixturesDir, 'vulnerable-app');
    const files = await scanDirectory(vulnerableDir);
    const findings = await scanner.analyze(files, rules);

    for (const finding of findings) {
      expect(finding.evidence).toContain('****');
    }
  });

  it('detects JWT tokens', async () => {
    const vulnerableDir = join(fixturesDir, 'vulnerable-app');
    const files = await scanDirectory(vulnerableDir);
    const findings = await scanner.analyze(files, rules);

    const jwtFindings = findings.filter(
      (f) => f.evidence && f.evidence.includes('eyJ'),
    );
    expect(jwtFindings.length).toBeGreaterThan(0);
  });
});
