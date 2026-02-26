# Soul Compliance Product — Design Document

**Date:** 2026-02-26
**Author:** Rishav
**Status:** Approved

## Overview

@soul/compliance is the first product in the Soul platform. It scans codebases for SOC 2, HIPAA, and GDPR compliance issues using 90% static analysis and 10% LLM-assisted classification. It provides terminal/JSON output (free), HTML/PDF reports (paid), auto-remediation via patch files (paid), badge generation (free), and file-watch monitor mode (paid).

## Key Decisions

| Decision | Choice | Rationale |
|---|---|---|
| AST analysis | tree-sitter WASM bindings | Accurate cross-language AST parsing, no native deps, works on aarch64 |
| Report generation | HTML template + Puppeteer for PDF | Rich formatting, professional output, Puppeteer is lazy-loaded optional dep |
| Auto-remediation | Unified diff patches + approval prompt | Safe, auditable, reversible. User approves each patch individually |
| Scope | Full design spec (all analyzers, frameworks, reporters) | Ship complete product for v0.1.0 |

## 1. Package Structure

```
products/compliance/
├── src/
│   ├── index.ts              # register(registry) — registers all tools + commands
│   ├── types.ts              # Finding, Severity, Framework, ScanResult, RuleDefinition
│   ├── tools/
│   │   ├── scan.ts           # compliance.scan — meta-scanner orchestrator
│   │   ├── report.ts         # compliance.report — generate HTML/PDF/JSON
│   │   ├── badge.ts          # compliance.badge — SVG badge generator
│   │   ├── fix.ts            # compliance.fix — auto-remediation via patches
│   │   └── monitor.ts        # compliance.monitor — watch mode
│   ├── analyzers/
│   │   ├── secret-scanner.ts # Regex + Shannon entropy secret detection
│   │   ├── config-checker.ts # Parse config files for security settings
│   │   ├── ast-analyzer.ts   # tree-sitter: auth, crypto, injection patterns
│   │   ├── git-analyzer.ts   # Branch protection, commit signing, .gitignore
│   │   └── dep-auditor.ts    # CVE checking via npm audit / osv-scanner
│   ├── rules/
│   │   ├── index.ts          # Load and merge rules from YAML
│   │   ├── soc2.yaml         # SOC 2 Type II control mappings (~40 rules)
│   │   ├── hipaa.yaml        # HIPAA Security Rule mappings (~25 rules)
│   │   └── gdpr.yaml         # GDPR Article 25/32 mappings (~18 rules)
│   └── reporters/
│       ├── terminal.ts       # Ink-based TUI findings display
│       ├── json.ts           # Machine-readable JSON output
│       ├── html.ts           # Standalone HTML report
│       ├── pdf.ts            # Puppeteer HTML→PDF conversion
│       └── badge.ts          # SVG badge file generator
├── __tests__/
│   ├── fixtures/
│   │   ├── vulnerable-app/   # Intentionally insecure test app
│   │   └── compliant-app/    # Clean baseline test app
│   ├── analyzers/
│   │   ├── secret-scanner.test.ts
│   │   ├── config-checker.test.ts
│   │   ├── ast-analyzer.test.ts
│   │   ├── git-analyzer.test.ts
│   │   └── dep-auditor.test.ts
│   ├── rules.test.ts
│   ├── scan.test.ts
│   ├── reporters.test.ts
│   └── fix.test.ts
├── package.json
└── tsconfig.json
```

The `register()` function registers 5 tools and 5 CLI commands into the PluginRegistry. The CLI routes `soul compliance <command>` to the matching tool.

## 2. Core Types

```typescript
type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info';
type Framework = 'soc2' | 'hipaa' | 'gdpr';

interface Finding {
  id: string;                    // e.g. 'SECRET-001', 'CRYPTO-003'
  title: string;                 // "Hardcoded AWS access key"
  description: string;
  severity: Severity;
  framework: Framework[];        // Which frameworks this maps to
  controlIds: string[];          // e.g. ['CC6.1', 'CC6.7']
  file?: string;                 // Relative path
  line?: number;
  column?: number;
  evidence?: string;             // Offending code (redacted for secrets)
  analyzer: string;              // Which analyzer found it
  fixable: boolean;
  fix?: FixSuggestion;
}

interface FixSuggestion {
  description: string;           // "Replace hardcoded key with env variable"
  patch: string;                 // Unified diff
}

interface ScanResult {
  findings: Finding[];
  summary: {
    total: number;
    bySeverity: Record<Severity, number>;
    byFramework: Record<Framework, number>;
    byAnalyzer: Record<string, number>;
    fixable: number;
  };
  metadata: {
    directory: string;
    duration: number;
    analyzersRun: string[];
    frameworks: Framework[];
    timestamp: string;
  };
}

interface RuleDefinition {
  id: string;
  title: string;
  severity: Severity;
  analyzer: string;
  pattern: string;               // Analyzer-specific pattern key
  controls: string[];            // Framework control IDs
  framework: Framework[];
  description: string;
  fixable: boolean;
}
```

Rules defined in YAML, loaded at scan time, filtered by requested frameworks.

## 3. Analyzers

All analyzers implement a consistent interface:

```typescript
interface Analyzer {
  name: string;
  analyze(files: ScannedFile[], rules: RuleDefinition[]): Promise<Finding[]>;
}
```

### Secret Scanner
- ~30 regex patterns: AWS keys, GitHub tokens, generic passwords, private keys, JWTs, Slack tokens
- Shannon entropy detection: >4.5 for hex strings, >5.0 for base64
- Evidence redacted: first 4 chars + `****`

### Config Checker
- package.json: missing lockfile, unpinned engines
- .env files: existence check (should be in .gitignore)
- Dockerfile: USER root, no health check, privileged ports
- nginx/apache: missing HSTS, weak TLS, no rate limiting
- CORS: wildcard origins
- Security headers: CSP, X-Frame-Options, X-Content-Type-Options

### AST Analyzer (tree-sitter WASM)
- Auth: missing middleware on routes, no session timeout, no RBAC
- Crypto: weak algorithms (MD5, SHA1 for passwords), no salt, ECB mode
- SQL injection: string concatenation in queries
- Input validation: no sanitization, missing schemas
- Error handling: empty catch blocks, exposed stack traces
- Languages: TypeScript, JavaScript, Python, Go, Java (WASM grammars)
- Falls back to regex for unsupported languages

### Git Analyzer
- .gitignore: missing .env, node_modules, secrets entries
- Branch protection: checks local config
- Commit history: unsigned commits, force pushes in reflog
- Missing CODEOWNERS file
- Large binary files in git

### Dependency Auditor
- `npm audit --json` for CVE detection
- `osv-scanner --json` if available (broader coverage)
- License check: flags copyleft (GPL, AGPL) conflicts
- Pinning: detects unpinned production deps (^ or ~ ranges)
- Graceful fallback if tools unavailable

## 4. Scan Orchestrator

```
soul compliance scan [path] [flags]

Flags:
  --framework soc2,hipaa,gdpr    # Default: all
  --severity critical,high       # Minimum severity to report
  --format terminal|json         # Default: terminal
  --output <file>                # Write to file
  --analyzer <name>              # Run specific analyzer(s)
  --exclude <glob>               # Additional exclusions
```

Flow:
1. Scan directory via `@soul/context` `scanDirectory()`
2. Load rules from YAML, filter by requested frameworks
3. Run all 5 analyzers in parallel via `Promise.allSettled()`
4. Deduplicate findings (same file+line+rule = merge, keep highest severity)
5. Build ScanResult with summary stats
6. Route to requested reporter

Terminal output summary:
```
◆ Scan complete: 47 findings (3 critical, 12 high, 20 medium, 12 low)
  Frameworks: SOC 2 (38), HIPAA (22), GDPR (15)
  Fixable: 31/47 — run `soul compliance fix` to remediate
```

## 5. Reporters

### Terminal (free)
Ink components via `@soul/ui`. Findings grouped by severity, color-coded. Shows file:line, title, control IDs, evidence snippet.

### JSON (free)
Full ScanResult as pretty-printed JSON. Exit code 1 if critical/high findings, 0 otherwise. CI-friendly.

### HTML (paid)
Standalone single-file HTML with embedded CSS. Sections: executive summary, findings by framework, by severity, remediation roadmap. Inline SVG charts, Soul branding. No JS dependencies in output.

### PDF (paid)
Puppeteer renders HTML report to PDF. Puppeteer is lazy-loaded — only imported when PDF requested. If not installed, shows helpful error with install command.

### Badge (free)
SVG badge in shields.io format. Score = (total rules - findings) / total rules * 100. Green >80%, yellow 60-80%, red <60%. Writes `compliance-badge.svg`.

## 6. Auto-Remediation (Fix)

```
soul compliance fix [path] [flags]

Flags:
  --dry-run              # Show patches without applying
  --auto-approve         # Apply all without prompting (CI mode)
  --finding <id>         # Fix specific finding(s)
  --severity critical,high
```

Flow:
1. Run scan (or accept piped ScanResult JSON from stdin)
2. Filter fixable findings
3. Generate unified diff patches (deterministic templates, not LLM)
4. Present via `<DiffPreview />` + `<ApprovalPrompt />` [Y/n/all/skip]
5. Apply via `git apply`, skip on conflict
6. Summary: "Applied 8/12 fixes. 4 skipped."

Fix templates are colocated with detection logic in each analyzer. Tier gate: `requireTier('pro')`.

## 7. Monitor Mode

```
soul compliance monitor [path] [flags]

Flags:
  --interval <seconds>   # Re-scan interval (default: 30)
  --severity critical,high
```

Uses `fs.watch` (recursive). On file change:
1. Debounce 500ms
2. Re-scan only changed files
3. Diff findings against previous scan
4. Display new/resolved findings
5. Running status bar with finding counts and timestamp

Persistent Ink terminal process. Ctrl+C exits cleanly. Tier gate: `requireTier('pro')`.

## 8. CLI Integration

Subcommand routing in `apps/cli/src/main.ts`:

```
soul compliance scan [path] [flags]
soul compliance report [path] [flags]
soul compliance fix [path] [flags]
soul compliance badge [path] [flags]
soul compliance monitor [path] [flags]
```

Simple argument parsing: `args[0]` = product, `args[1]` = command, rest = flags. No CLI framework dependency. The compliance `register()` adds CommandDefinition entries, CLI routes to matching tool's `execute()`.

## 9. Testing

### Fixtures
- `vulnerable-app/`: hardcoded secrets, no auth, SQL concat, MD5, unpinned deps, committed .env, root Docker user, no CODEOWNERS
- `compliant-app/`: bcrypt, parameterized queries, helmet, auth middleware, pinned deps, proper .gitignore

### Test Layers
1. **Analyzer unit tests**: ~5-8 per analyzer, ~30 total. Vulnerable fixture → expected findings. Compliant fixture → zero findings.
2. **Rule loading**: validate YAML, no duplicate IDs, required fields present, referenced analyzers exist.
3. **Scan integration**: full scan on both fixtures, assert finding counts match expected.
4. **Reporter tests**: JSON is valid ScanResult, HTML contains sections, badge SVG is valid.
5. **Fix tests**: generate patches, verify valid diffs, apply to temp copy, re-scan confirms resolution.

No LLM calls in tests — all static analysis is deterministic.

## 10. Dependencies

| Package | Purpose | Notes |
|---|---|---|
| `web-tree-sitter` | WASM tree-sitter runtime | ~2MB |
| `tree-sitter-typescript` | TS/JS grammar | WASM, ~1MB |
| `tree-sitter-python` | Python grammar | WASM, ~500KB |
| `tree-sitter-go` | Go grammar | WASM, ~500KB |
| `tree-sitter-java` | Java grammar | WASM, ~500KB |
| `js-yaml` | Parse rule YAML files | ~100KB |
| `diff` | Generate unified diffs | ~50KB |
| `puppeteer` | HTML→PDF | Optional peer dep, lazy loaded |

All tree-sitter grammars are WASM — no native compilation, works on aarch64 and x86.

## 11. Free/Paid Gate

```
Free:  scan + terminal output + JSON export + badge
Paid:  auto-remediation + HTML/PDF reports + monitor mode + custom rules
```

Free reveals problems. Paid fixes them.
