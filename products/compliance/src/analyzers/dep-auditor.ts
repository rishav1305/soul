import type { Analyzer, Finding, RuleDefinition, ScannedFile } from '../types.js';
import { readFileSync, existsSync } from 'node:fs';
import { dirname, join, basename } from 'node:path';

/**
 * Audits project dependencies for compliance issues.
 *
 * Static checks (no external commands):
 * - Unpinned production dependencies (^ or ~ version ranges)
 * - Missing lockfile (package-lock.json or npm-shrinkwrap.json)
 * - Missing engines field in package.json
 * - Copyleft license (GPL, AGPL, LGPL) in package.json
 */
export class DepAuditor implements Analyzer {
  name = 'dep-auditor';

  async analyze(files: ScannedFile[], rules: RuleDefinition[]): Promise<Finding[]> {
    const myRules = rules.filter((r) => r.analyzer === this.name);
    if (myRules.length === 0) return [];

    // Build lookup from pattern -> rules (multiple rules may share a pattern across frameworks)
    const rulesByPattern = new Map<string, RuleDefinition[]>();
    for (const rule of myRules) {
      const existing = rulesByPattern.get(rule.pattern) ?? [];
      existing.push(rule);
      rulesByPattern.set(rule.pattern, existing);
    }

    const findings: Finding[] = [];

    const packageFiles = files.filter((f) => basename(f.path) === 'package.json');

    for (const pkgFile of packageFiles) {
      let pkg: Record<string, unknown>;
      try {
        const content = readFileSync(pkgFile.path, 'utf-8');
        pkg = JSON.parse(content) as Record<string, unknown>;
      } catch {
        continue;
      }

      // ── Unpinned dependencies ────────────────────────────────────
      this.checkUnpinnedDeps(pkg, pkgFile, rulesByPattern, findings);

      // ── Missing lockfile ─────────────────────────────────────────
      this.checkMissingLockfile(pkgFile, rulesByPattern, findings);

      // ── Missing engines ──────────────────────────────────────────
      this.checkMissingEngines(pkg, pkgFile, rulesByPattern, findings);

      // ── Copyleft license ─────────────────────────────────────────
      this.checkCopyleftLicense(pkg, pkgFile, rulesByPattern, findings);
    }

    return findings;
  }

  /**
   * Check production dependencies for unpinned version ranges (^ or ~).
   * Creates ONE finding per package.json listing all unpinned deps in the evidence.
   */
  private checkUnpinnedDeps(
    pkg: Record<string, unknown>,
    pkgFile: ScannedFile,
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const deps = pkg.dependencies as Record<string, string> | undefined;
    if (!deps) return;

    const unpinned = Object.entries(deps).filter(
      ([, version]) => version.startsWith('^') || version.startsWith('~'),
    );

    if (unpinned.length > 0) {
      this.addFindings(findings, 'unpinned-deps', rulesByPattern, {
        file: pkgFile.relativePath,
        evidence: `Unpinned: ${unpinned.map(([name]) => name).join(', ')}`,
      });
    }
  }

  /**
   * Check if a lockfile (package-lock.json or npm-shrinkwrap.json) exists
   * in the same directory as the package.json.
   */
  private checkMissingLockfile(
    pkgFile: ScannedFile,
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const dir = dirname(pkgFile.path);
    const hasLockfile =
      existsSync(join(dir, 'package-lock.json')) ||
      existsSync(join(dir, 'npm-shrinkwrap.json'));

    if (!hasLockfile) {
      this.addFindings(findings, 'missing-lockfile', rulesByPattern, {
        file: pkgFile.relativePath,
        evidence: 'No package-lock.json or npm-shrinkwrap.json found',
      });
    }
  }

  /**
   * Check if package.json has an engines field.
   */
  private checkMissingEngines(
    pkg: Record<string, unknown>,
    pkgFile: ScannedFile,
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    if (!pkg.engines) {
      this.addFindings(findings, 'missing-engines', rulesByPattern, {
        file: pkgFile.relativePath,
        evidence: 'No engines field in package.json',
      });
    }
  }

  /**
   * Check if the license field in package.json indicates a copyleft license
   * (GPL, AGPL, LGPL).
   */
  private checkCopyleftLicense(
    pkg: Record<string, unknown>,
    pkgFile: ScannedFile,
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const license = pkg.license as string | undefined;
    if (!license) return;

    const upper = license.toUpperCase();
    if (upper.includes('GPL') || upper.includes('AGPL') || upper.includes('LGPL')) {
      this.addFindings(findings, 'copyleft-license', rulesByPattern, {
        file: pkgFile.relativePath,
        evidence: `Copyleft license detected: ${license}`,
      });
    }
  }

  /**
   * Create findings for all rules matching a given pattern.
   * A single pattern may map to rules in multiple frameworks (soc2, hipaa, gdpr).
   */
  private addFindings(
    findings: Finding[],
    pattern: string,
    rulesByPattern: Map<string, RuleDefinition[]>,
    context: { file?: string; line?: number; evidence?: string },
  ): void {
    const matchedRules = rulesByPattern.get(pattern);
    if (!matchedRules || matchedRules.length === 0) return;

    for (const rule of matchedRules) {
      findings.push({
        id: rule.id,
        title: rule.title,
        description: rule.description,
        severity: rule.severity,
        framework: rule.framework,
        controlIds: rule.controls,
        file: context.file,
        line: context.line,
        evidence: context.evidence,
        analyzer: this.name,
        fixable: rule.fixable,
      });
    }
  }
}
