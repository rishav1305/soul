import type { Analyzer, Finding, RuleDefinition, ScannedFile } from '../types.js';
import { readFileSync } from 'node:fs';

/**
 * Pattern definition linking a regex to a rule pattern identifier.
 */
interface SecretPattern {
  name: string;
  regex: RegExp;
  rulePattern: string;
}

const TEXT_EXTENSIONS = new Set([
  'ts', 'js', 'py', 'go', 'java', 'rb',
  'yaml', 'yml', 'json', 'toml',
  'env', 'cfg', 'conf', 'ini', 'xml', 'properties',
]);

const MAX_FILE_SIZE = 500 * 1024; // 500 KB

const SECRET_PATTERNS: SecretPattern[] = [
  // AWS Access Key
  {
    name: 'AWS Access Key',
    regex: /AKIA[0-9A-Z]{16}/g,
    rulePattern: 'hardcoded-credential',
  },
  // AWS Secret Key (high-entropy string following common assignment patterns)
  {
    name: 'AWS Secret Key',
    regex: /(?:aws_secret|AWS_SECRET|secret_key|SECRET_KEY)\s*[=:]\s*['"]?([A-Za-z0-9/+=]{40})['"]?/g,
    rulePattern: 'hardcoded-credential',
  },
  // GitHub Token
  {
    name: 'GitHub Token',
    regex: /gh[ps]_[A-Za-z0-9_]{36,}/g,
    rulePattern: 'api-token',
  },
  // Private Key
  {
    name: 'Private Key',
    regex: /-----BEGIN\s+(?:RSA|EC|DSA|PGP)?\s*PRIVATE KEY-----/g,
    rulePattern: 'private-key',
  },
  // JWT Token
  {
    name: 'JWT Token',
    regex: /eyJ[A-Za-z0-9\-_]+\.eyJ[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_.+/=]+/g,
    rulePattern: 'api-token',
  },
  // Slack Token
  {
    name: 'Slack Token',
    regex: /xox[bpras]-[0-9a-zA-Z-]+/g,
    rulePattern: 'api-token',
  },
  // Stripe Key
  {
    name: 'Stripe Key',
    regex: /sk_(?:live|test)_[0-9a-zA-Z]{24,}/g,
    rulePattern: 'api-token',
  },
  // Anthropic Key
  {
    name: 'Anthropic Key',
    regex: /sk-ant-[a-zA-Z0-9-]+/g,
    rulePattern: 'api-token',
  },
  // Generic Password (case insensitive)
  {
    name: 'Generic Password',
    regex: /(?:password|passwd|pwd|secret)\s*[=:]\s*['"][^'"]{4,}['"]/gi,
    rulePattern: 'hardcoded-credential',
  },
  // Generic API Key (case insensitive)
  {
    name: 'Generic API Key',
    regex: /(?:api[_-]?key|apikey)\s*[=:]\s*['"][^'"]{8,}['"]/gi,
    rulePattern: 'hardcoded-credential',
  },
  // Database URL with credentials
  {
    name: 'Database URL',
    regex: /(?:mongodb|postgres|mysql|redis):\/\/[^\s'"]+:[^\s'"]+@/g,
    rulePattern: 'hardcoded-credential',
  },
  // Google API Key
  {
    name: 'Google API Key',
    regex: /AIza[0-9A-Za-z\-_]{35}/g,
    rulePattern: 'api-token',
  },
  // Heroku API Key
  {
    name: 'Heroku API Key',
    regex: /[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/g,
    rulePattern: 'api-token',
  },
  // SendGrid API Key
  {
    name: 'SendGrid API Key',
    regex: /SG\.[0-9A-Za-z\-_]{22,}\.[0-9A-Za-z\-_]{22,}/g,
    rulePattern: 'api-token',
  },
  // Twilio API Key
  {
    name: 'Twilio API Key',
    regex: /SK[0-9a-fA-F]{32}/g,
    rulePattern: 'api-token',
  },
  // Mailgun API Key
  {
    name: 'Mailgun API Key',
    regex: /key-[0-9a-zA-Z]{32}/g,
    rulePattern: 'api-token',
  },
];

/**
 * Compute Shannon entropy of a string.
 */
function shannonEntropy(str: string): number {
  const freq = new Map<string, number>();
  for (const ch of str) freq.set(ch, (freq.get(ch) ?? 0) + 1);
  let entropy = 0;
  for (const count of freq.values()) {
    const p = count / str.length;
    entropy -= p * Math.log2(p);
  }
  return entropy;
}

/**
 * Redact the middle of a secret, keeping first and last 4 characters.
 */
function redact(secret: string): string {
  if (secret.length <= 8) return '****';
  return secret.slice(0, 4) + '****' + secret.slice(-4);
}

/**
 * Check if a string looks like it could be hex-encoded.
 */
function isHexLike(str: string): boolean {
  return /^[0-9a-fA-F]+$/.test(str);
}

/**
 * Check if a string looks like it could be base64-encoded.
 */
function isBase64Like(str: string): boolean {
  return /^[A-Za-z0-9+/=\-_]+$/.test(str);
}

/**
 * Check if a file extension is a text-like extension we should scan.
 */
function isTextExtension(ext: string): boolean {
  return TEXT_EXTENSIONS.has(ext.toLowerCase());
}

/**
 * Detect high-entropy strings that may be secrets.
 */
function detectHighEntropyStrings(line: string): string[] {
  const results: string[] = [];
  // Match quoted strings and bare assignments that look like secrets
  const candidates = line.matchAll(/['"]([A-Za-z0-9+/=\-_]{20,})['"]/g);
  for (const match of candidates) {
    const candidate = match[1];
    if (candidate.length < 20) continue;

    const threshold = isHexLike(candidate) ? 4.5 : isBase64Like(candidate) ? 5.0 : 5.0;
    const entropy = shannonEntropy(candidate);
    if (entropy > threshold) {
      results.push(candidate);
    }
  }
  return results;
}

interface MatchResult {
  patternName: string;
  rulePattern: string;
  matched: string;
  line: number;
  column: number;
}

export class SecretScanner implements Analyzer {
  name = 'secret-scanner';

  async analyze(files: ScannedFile[], rules: RuleDefinition[]): Promise<Finding[]> {
    // Filter rules relevant to this analyzer
    const myRules = rules.filter((r) => r.analyzer === this.name);
    if (myRules.length === 0) return [];

    // Build a lookup from pattern -> rules
    const rulesByPattern = new Map<string, RuleDefinition[]>();
    for (const rule of myRules) {
      const existing = rulesByPattern.get(rule.pattern) ?? [];
      existing.push(rule);
      rulesByPattern.set(rule.pattern, existing);
    }

    const findings: Finding[] = [];

    for (const file of files) {
      // Skip non-text files
      if (!isTextExtension(file.extension)) continue;
      // Skip large files
      if (file.size > MAX_FILE_SIZE) continue;

      let content: string;
      try {
        content = readFileSync(file.path, 'utf-8');
      } catch {
        // Skip files that can't be read (binary, permissions, etc.)
        continue;
      }

      // Quick binary check: if content contains null bytes, skip
      if (content.includes('\0')) continue;

      const lines = content.split('\n');
      const matches: MatchResult[] = [];

      for (let lineIdx = 0; lineIdx < lines.length; lineIdx++) {
        const line = lines[lineIdx];

        // Run each secret pattern against the line
        for (const pattern of SECRET_PATTERNS) {
          // Reset regex lastIndex since we reuse global regexes
          pattern.regex.lastIndex = 0;
          let match: RegExpExecArray | null;
          while ((match = pattern.regex.exec(line)) !== null) {
            matches.push({
              patternName: pattern.name,
              rulePattern: pattern.rulePattern,
              matched: match[0],
              line: lineIdx + 1,
              column: match.index + 1,
            });
          }
        }

        // Check for high-entropy strings
        const highEntropyMatches = detectHighEntropyStrings(line);
        for (const heMatch of highEntropyMatches) {
          // Avoid duplicate findings: skip if already matched by a specific pattern
          const alreadyMatched = matches.some(
            (m) => m.line === lineIdx + 1 && line.includes(m.matched) && heMatch.includes(m.matched),
          );
          if (!alreadyMatched) {
            matches.push({
              patternName: 'High-entropy string',
              rulePattern: 'high-entropy',
              matched: heMatch,
              line: lineIdx + 1,
              column: line.indexOf(heMatch) + 1,
            });
          }
        }
      }

      // Convert matches to findings
      for (const match of matches) {
        const matchedRules = rulesByPattern.get(match.rulePattern);
        if (!matchedRules || matchedRules.length === 0) continue;

        for (const rule of matchedRules) {
          findings.push({
            id: rule.id,
            title: rule.title,
            description: `${rule.description} (${match.patternName}: ${redact(match.matched)})`,
            severity: rule.severity,
            framework: rule.framework,
            controlIds: rule.controls,
            file: file.relativePath,
            line: match.line,
            column: match.column,
            evidence: redact(match.matched),
            analyzer: this.name,
            fixable: rule.fixable,
          });
        }
      }
    }

    return findings;
  }
}
