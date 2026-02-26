import type { Analyzer, Finding, RuleDefinition, ScannedFile } from '../types.js';
import { readFileSync, existsSync } from 'node:fs';
import { dirname, join, basename } from 'node:path';

/**
 * Analyzes project configuration files for security and compliance issues.
 *
 * Checks include:
 * - .env files not excluded by .gitignore
 * - Dockerfile running as root or missing HEALTHCHECK
 * - Missing CI/CD configuration
 * - Wildcard CORS origins in source files
 */
export class ConfigChecker implements Analyzer {
  name = 'config-checker';

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

    // ── .env not in .gitignore ──────────────────────────────────────
    this.checkEnvNotGitignored(files, rulesByPattern, findings);

    // ── package.json checks ─────────────────────────────────────────
    this.checkPackageJson(files, rulesByPattern, findings);

    // ── Dockerfile checks ───────────────────────────────────────────
    this.checkDockerfile(files, rulesByPattern, findings);

    // ── Wildcard CORS ───────────────────────────────────────────────
    this.checkWildcardCors(files, rulesByPattern, findings);

    return findings;
  }

  /**
   * If a .env file exists in scanned files, check that .gitignore in the
   * same directory contains a .env entry.
   */
  private checkEnvNotGitignored(
    files: ScannedFile[],
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const envFiles = files.filter((f) => basename(f.path) === '.env');
    for (const envFile of envFiles) {
      const dir = dirname(envFile.path);
      const gitignorePath = join(dir, '.gitignore');
      let gitignored = false;
      try {
        const content = readFileSync(gitignorePath, 'utf-8');
        const lines = content.split('\n').map((l) => l.trim());
        gitignored = lines.some(
          (line) => line === '.env' || line === '.env*' || line === '*.env',
        );
      } catch {
        // No .gitignore — .env is not gitignored
      }
      if (!gitignored) {
        this.addFindings(findings, 'env-not-gitignored', rulesByPattern, {
          file: envFile.relativePath,
          evidence: '.env not listed in .gitignore',
        });
      }
    }
  }

  /**
   * Parse package.json and check for:
   * - Unpinned dependencies (^ or ~ prefixes)
   * - Missing engines field
   * - Missing lockfile (package-lock.json)
   * - Missing CI configuration
   */
  private checkPackageJson(
    files: ScannedFile[],
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const packageFiles = files.filter((f) => basename(f.path) === 'package.json');
    for (const pkgFile of packageFiles) {
      let pkg: Record<string, unknown>;
      try {
        const content = readFileSync(pkgFile.path, 'utf-8');
        pkg = JSON.parse(content) as Record<string, unknown>;
      } catch {
        continue;
      }

      // Unpinned dependencies
      const deps = pkg.dependencies as Record<string, string> | undefined;
      if (deps) {
        const unpinned = Object.entries(deps).filter(
          ([, version]) => version.startsWith('^') || version.startsWith('~'),
        );
        if (unpinned.length > 0) {
          this.addFindings(findings, 'unpinned-deps', rulesByPattern, {
            file: pkgFile.relativePath,
            evidence: `Unpinned: ${unpinned.map(([n]) => n).join(', ')}`,
          });
        }
      }

      // Missing engines
      if (!pkg.engines) {
        this.addFindings(findings, 'missing-engines', rulesByPattern, {
          file: pkgFile.relativePath,
          evidence: 'No engines field in package.json',
        });
      }

      // Missing lockfile
      const lockfilePath = join(dirname(pkgFile.path), 'package-lock.json');
      if (!existsSync(lockfilePath)) {
        this.addFindings(findings, 'missing-lockfile', rulesByPattern, {
          file: pkgFile.relativePath,
          evidence: 'No package-lock.json found',
        });
      }

      // No CI config
      const hasCi = files.some(
        (f) =>
          f.relativePath.startsWith('.github/workflows') ||
          basename(f.path) === '.gitlab-ci.yml' ||
          basename(f.path) === 'Jenkinsfile',
      );
      if (!hasCi) {
        this.addFindings(findings, 'no-ci-config', rulesByPattern, {
          file: pkgFile.relativePath,
          evidence: 'No CI/CD configuration found',
        });
      }
    }
  }

  /**
   * Check Dockerfile for:
   * - Running as root (USER root or no USER directive)
   * - Missing HEALTHCHECK directive
   */
  private checkDockerfile(
    files: ScannedFile[],
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const dockerfiles = files.filter((f) => basename(f.path) === 'Dockerfile');
    for (const dockerfile of dockerfiles) {
      let content: string;
      try {
        content = readFileSync(dockerfile.path, 'utf-8');
      } catch {
        continue;
      }

      // Check for root user
      const lines = content.split('\n');
      const userDirectives = lines.filter((line) => /^\s*USER\s+/i.test(line));
      const isRoot =
        userDirectives.length === 0 ||
        userDirectives.some((line) => /^\s*USER\s+root\s*$/i.test(line));

      if (isRoot) {
        // Find the line number of USER root, or report on last line
        let lineNum: number | undefined;
        for (let i = 0; i < lines.length; i++) {
          if (/^\s*USER\s+root\s*$/i.test(lines[i])) {
            lineNum = i + 1;
            break;
          }
        }
        this.addFindings(findings, 'docker-root-user', rulesByPattern, {
          file: dockerfile.relativePath,
          line: lineNum,
          evidence: userDirectives.length === 0 ? 'No USER directive' : 'USER root',
        });
      }

      // Check for HEALTHCHECK
      const hasHealthcheck = lines.some((line) => /^\s*HEALTHCHECK\s+/i.test(line));
      if (!hasHealthcheck) {
        this.addFindings(findings, 'docker-no-healthcheck', rulesByPattern, {
          file: dockerfile.relativePath,
          evidence: 'No HEALTHCHECK directive in Dockerfile',
        });
      }
    }
  }

  /**
   * Check source files for wildcard CORS configuration.
   */
  private checkWildcardCors(
    files: ScannedFile[],
    rulesByPattern: Map<string, RuleDefinition[]>,
    findings: Finding[],
  ): void {
    const TEXT_EXTENSIONS = new Set([
      'ts', 'js', 'py', 'go', 'java', 'rb',
      'yaml', 'yml', 'json', 'toml',
      'env', 'cfg', 'conf', 'ini', 'xml', 'properties',
    ]);

    for (const file of files) {
      if (!TEXT_EXTENSIONS.has(file.extension.toLowerCase())) continue;
      if (file.size > 500 * 1024) continue;

      let content: string;
      try {
        content = readFileSync(file.path, 'utf-8');
      } catch {
        continue;
      }

      if (content.includes('\0')) continue;

      const lines = content.split('\n');
      for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        if (
          line.includes('Access-Control-Allow-Origin: *') ||
          /origin:\s*['"]\*['"]/.test(line)
        ) {
          this.addFindings(findings, 'wildcard-cors', rulesByPattern, {
            file: file.relativePath,
            line: i + 1,
            evidence: 'Wildcard CORS origin detected',
          });
          break; // One finding per file is sufficient
        }
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
