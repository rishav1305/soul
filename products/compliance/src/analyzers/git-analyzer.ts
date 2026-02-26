import type { Analyzer, Finding, RuleDefinition, ScannedFile } from '../types.js';
import { readFileSync } from 'node:fs';
import { basename } from 'node:path';

/**
 * Analyzes repository hygiene for compliance issues.
 *
 * Checks include (static file inspection only - no git commands):
 * - .gitignore completeness (*.pem, *.key, .env entries)
 * - CODEOWNERS file presence
 * - SECURITY.md file presence
 * - LICENSE file presence
 * - Large files (>5MB) that should not be in git
 */
export class GitAnalyzer implements Analyzer {
  name = 'git-analyzer';

  private static readonly LARGE_FILE_THRESHOLD = 5 * 1024 * 1024; // 5 MB

  async analyze(files: ScannedFile[], rules: RuleDefinition[]): Promise<Finding[]> {
    const myRules = rules.filter((r) => r.analyzer === this.name);
    if (myRules.length === 0) return [];

    // Build lookup from pattern -> rules (multiple rules may share a pattern)
    const rulesByPattern = new Map<string, RuleDefinition[]>();
    for (const rule of myRules) {
      const existing = rulesByPattern.get(rule.pattern) ?? [];
      existing.push(rule);
      rulesByPattern.set(rule.pattern, existing);
    }

    const findings: Finding[] = [];

    // ── .gitignore completeness ─────────────────────────────────────
    this.checkGitignoreCompleteness(files, rulesByPattern, findings);

    // ── CODEOWNERS ──────────────────────────────────────────────────
    this.checkCodeowners(files, rulesByPattern, findings);

    // ── SECURITY.md ─────────────────────────────────────────────────
    this.checkSecurityPolicy(files, rulesByPattern, findings);

    // ── LICENSE ─────────────────────────────────────────────────────
    this.checkLicense(files, rulesByPattern, findings);

    // ── Large files ─────────────────────────────────────────────────
    this.checkLargeFiles(files, rulesByPattern, findings);

    return findings;
  }

  /**
   * Check .gitignore files for completeness:
   * - .env entry
   * - *.pem and *.key entries
   * - General completeness
   */
  private checkGitignoreCompleteness(
    files: ScannedFile[],
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const gitignoreFiles = files.filter((f) => basename(f.path) === '.gitignore');

    for (const gitignoreFile of gitignoreFiles) {
      let content: string;
      try {
        content = readFileSync(gitignoreFile.path, 'utf-8');
      } catch {
        continue;
      }

      const lines = content.split('\n').map((l) => l.trim());

      // Check for .env
      const hasEnv = lines.some(
        (line) => line === '.env' || line === '.env*' || line === '*.env',
      );
      if (!hasEnv) {
        this.addFindings(findings, 'env-not-gitignored', rulesByPattern, {
          file: gitignoreFile.relativePath,
          evidence: '.env not listed in .gitignore',
        });
      }

      // Check for *.pem and *.key
      const hasPem = lines.some((line) => line === '*.pem');
      const hasKey = lines.some((line) => line === '*.key');
      if (!hasPem || !hasKey) {
        const missing = [];
        if (!hasPem) missing.push('*.pem');
        if (!hasKey) missing.push('*.key');
        this.addFindings(findings, 'sensitive-not-gitignored', rulesByPattern, {
          file: gitignoreFile.relativePath,
          evidence: `Missing gitignore entries: ${missing.join(', ')}`,
        });
      }

      // General incompleteness: flag if missing .env OR (*.pem AND *.key)
      if (!hasEnv || !hasPem || !hasKey) {
        this.addFindings(findings, 'incomplete-gitignore', rulesByPattern, {
          file: gitignoreFile.relativePath,
          evidence: 'Gitignore is missing recommended entries for sensitive files',
        });
      }
    }
  }

  /**
   * Check for CODEOWNERS file in standard locations:
   * CODEOWNERS, .github/CODEOWNERS, docs/CODEOWNERS
   */
  private checkCodeowners(
    files: ScannedFile[],
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const hasCodeowners = files.some((f) => {
      const rel = f.relativePath;
      return (
        rel === 'CODEOWNERS' ||
        rel === '.github/CODEOWNERS' ||
        rel === 'docs/CODEOWNERS'
      );
    });

    if (!hasCodeowners) {
      this.addFindings(findings, 'missing-codeowners', rulesByPattern, {
        evidence: 'No CODEOWNERS file found in repository',
      });
    }
  }

  /**
   * Check for SECURITY.md file.
   */
  private checkSecurityPolicy(
    files: ScannedFile[],
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const hasSecurityPolicy = files.some(
      (f) => basename(f.path).toUpperCase() === 'SECURITY.MD',
    );

    if (!hasSecurityPolicy) {
      this.addFindings(findings, 'no-security-policy', rulesByPattern, {
        evidence: 'No SECURITY.md file found in repository',
      });
    }
  }

  /**
   * Check for LICENSE file (LICENSE, LICENSE.md, or LICENSE.txt).
   */
  private checkLicense(
    files: ScannedFile[],
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const hasLicense = files.some((f) => {
      const name = basename(f.path).toUpperCase();
      return name === 'LICENSE' || name === 'LICENSE.MD' || name === 'LICENSE.TXT';
    });

    if (!hasLicense) {
      this.addFindings(findings, 'missing-license', rulesByPattern, {
        evidence: 'No LICENSE file found in repository',
      });
    }
  }

  /**
   * Flag any file larger than 5MB.
   */
  private checkLargeFiles(
    files: ScannedFile[],
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    for (const file of files) {
      if (file.size > GitAnalyzer.LARGE_FILE_THRESHOLD) {
        const sizeMB = (file.size / (1024 * 1024)).toFixed(1);
        this.addFindings(findings, 'large-binary-in-git', rulesByPattern, {
          file: file.relativePath,
          evidence: `File size: ${sizeMB} MB (threshold: 5 MB)`,
        });
      }
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
