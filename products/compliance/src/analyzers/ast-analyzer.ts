import type { Analyzer, Finding, RuleDefinition, ScannedFile } from '../types.js';
import { readFileSync } from 'node:fs';

/**
 * Regex-based code pattern analyzer.
 *
 * Scans TypeScript/JavaScript source files for common security anti-patterns
 * such as SQL injection, weak cryptography, empty catch blocks, and
 * exposed stack traces. Each check maps to one or more rule patterns
 * defined in the YAML rule files.
 */

const CODE_EXTENSIONS = new Set(['ts', 'js', 'tsx', 'jsx']);
const MAX_FILE_SIZE = 500 * 1024; // 500 KB

/**
 * A pattern check links a rule-pattern string to one or more regexes
 * that are run against file content (line-by-line or full-content).
 */
interface PatternCheck {
  /** The rule pattern identifier (e.g. 'sql-injection', 'weak-hash'). */
  pattern: string;
  /** Human-readable evidence label. */
  label: string;
  /**
   * Run the check against a file's content and lines.
   * Returns an array of matches with line number and evidence string.
   */
  check(content: string, lines: string[]): PatternMatch[];
}

interface PatternMatch {
  line: number;
  evidence: string;
}

// ── Pattern checks ──────────────────────────────────────────────────────

const PATTERN_CHECKS: PatternCheck[] = [
  // 1. SQL Injection — string concatenation or template-literal interpolation in SQL
  {
    pattern: 'sql-injection',
    label: 'SQL injection via string concatenation',
    check(_content, lines) {
      const matches: PatternMatch[] = [];
      const sqlKeyword = /\b(SELECT|INSERT|UPDATE|DELETE|DROP|ALTER)\b/i;
      for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        if (!sqlKeyword.test(line)) continue;
        // String concat with +
        if (/['"`].*\b(SELECT|INSERT|UPDATE|DELETE)\b.*['"`]\s*\+/i.test(line) ||
            /\+\s*['"`].*\b(SELECT|INSERT|UPDATE|DELETE|WHERE)\b/i.test(line)) {
          matches.push({ line: i + 1, evidence: 'SQL query built with string concatenation' });
          continue;
        }
        // Template literal with ${...} interpolation inside SQL
        if (/`[^`]*\b(SELECT|INSERT|UPDATE|DELETE)\b[^`]*\$\{[^}]+\}[^`]*`/i.test(line) ||
            /`[^`]*\$\{[^}]+\}[^`]*\b(WHERE|SELECT|INSERT|UPDATE|DELETE)\b[^`]*`/i.test(line)) {
          matches.push({ line: i + 1, evidence: 'SQL query built with template literal interpolation' });
        }
      }
      return matches;
    },
  },

  // 2. Weak Hashing — MD5 or SHA1
  {
    pattern: 'weak-hash',
    label: 'Weak hashing algorithm',
    check(_content, lines) {
      const matches: PatternMatch[] = [];
      const re = /createHash\s*\(\s*['"](?:md5|sha1)['"]\s*\)/i;
      for (let i = 0; i < lines.length; i++) {
        if (re.test(lines[i])) {
          matches.push({ line: i + 1, evidence: `Weak hash: ${lines[i].trim()}` });
        }
      }
      return matches;
    },
  },

  // 3. ECB Mode
  {
    pattern: 'ecb-mode',
    label: 'ECB encryption mode',
    check(_content, lines) {
      const matches: PatternMatch[] = [];
      const re = /createCipheriv\s*\(\s*['"][^'"]*ecb[^'"]*['"]/i;
      for (let i = 0; i < lines.length; i++) {
        if (re.test(lines[i])) {
          matches.push({ line: i + 1, evidence: 'ECB block cipher mode used' });
        }
      }
      return matches;
    },
  },

  // 4. Hardcoded Crypto Key/IV — Buffer.from('...') near createCipheriv in file
  {
    pattern: 'hardcoded-crypto-key',
    label: 'Hardcoded encryption key or IV',
    check(content, lines) {
      const matches: PatternMatch[] = [];
      // Only flag if the file also uses createCipheriv
      if (!/createCipheriv/i.test(content)) return matches;
      const re = /Buffer\.from\s*\(\s*['"][^'"]+['"]\s*\)/;
      for (let i = 0; i < lines.length; i++) {
        if (re.test(lines[i])) {
          matches.push({ line: i + 1, evidence: 'Hardcoded Buffer used with crypto' });
        }
      }
      return matches;
    },
  },

  // 5. Empty Catch Block
  {
    pattern: 'empty-catch',
    label: 'Empty catch block',
    check(content, _lines) {
      const matches: PatternMatch[] = [];
      // Use a regex that matches catch blocks where the body is empty
      // (only whitespace/newlines between { and })
      const re = /catch\s*\([^)]*\)\s*\{[\s]*\}/g;
      let match: RegExpExecArray | null;
      while ((match = re.exec(content)) !== null) {
        // Count line number from content position
        const lineNum = content.slice(0, match.index).split('\n').length;
        matches.push({ line: lineNum, evidence: 'Empty catch block swallows errors' });
      }
      return matches;
    },
  },

  // 6. Stack Trace Exposed — .stack in a response context
  {
    pattern: 'exposed-stack-trace',
    label: 'Stack trace exposed to client',
    check(_content, lines) {
      const matches: PatternMatch[] = [];
      for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        if (/\.stack\b/.test(line) && /\b(res|response)\b/.test(line)) {
          matches.push({ line: i + 1, evidence: 'Stack trace sent in response' });
        }
      }
      return matches;
    },
  },

  // 7. Sensitive data in logs
  {
    pattern: 'sensitive-in-logs',
    label: 'Sensitive data in logs',
    check(_content, lines) {
      const matches: PatternMatch[] = [];
      const logCall = /\b(console\.(log|info|debug|warn)|logger?\.(log|info|debug|warn))\s*\(/;
      const sensitivePattern = /\b(password|secret|token|apiKey|api_key|ssn|creditCard|credit_card)\b/i;
      for (let i = 0; i < lines.length; i++) {
        if (logCall.test(lines[i]) && sensitivePattern.test(lines[i])) {
          matches.push({ line: i + 1, evidence: 'Sensitive data may be logged' });
        }
      }
      return matches;
    },
  },

  // 8. Console.log in production code
  {
    pattern: 'console-in-production',
    label: 'Console.log in production code',
    check(_content, lines) {
      const matches: PatternMatch[] = [];
      const re = /\bconsole\.(log|info|debug)\s*\(/;
      for (let i = 0; i < lines.length; i++) {
        if (re.test(lines[i])) {
          matches.push({ line: i + 1, evidence: 'console.log found in source' });
        }
      }
      return matches;
    },
  },
];

export class AstAnalyzer implements Analyzer {
  name = 'ast-analyzer';

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

    for (const file of files) {
      // Only scan code files
      if (!CODE_EXTENSIONS.has(file.extension.toLowerCase())) continue;
      // Skip large files
      if (file.size > MAX_FILE_SIZE) continue;

      let content: string;
      try {
        content = readFileSync(file.path, 'utf-8');
      } catch {
        continue;
      }

      // Quick binary check
      if (content.includes('\0')) continue;

      const lines = content.split('\n');

      // Run each pattern check
      for (const check of PATTERN_CHECKS) {
        // Only run if there are rules that care about this pattern
        if (!rulesByPattern.has(check.pattern)) continue;

        const matches = check.check(content, lines);
        for (const match of matches) {
          this.addFindings(findings, check.pattern, rulesByPattern, {
            file: file.relativePath,
            line: match.line,
            evidence: match.evidence,
          });
        }
      }
    }

    return findings;
  }

  /**
   * Create findings for all rules matching a given pattern.
   * A single pattern may map to rules in multiple frameworks.
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
