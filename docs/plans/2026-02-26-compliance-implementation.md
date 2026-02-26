# Soul Compliance Product Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build @soul/compliance — SOC 2/HIPAA/GDPR compliance scanning with 5 analyzers, 5 reporters, auto-remediation, and monitor mode.

**Architecture:** Products register tools into the existing PluginRegistry. The compliance package has 5 analyzers (secret-scanner, config-checker, ast-analyzer, git-analyzer, dep-auditor) that produce Finding objects, a scan orchestrator that runs them in parallel, reporters that format output, and a fix tool that generates unified diff patches. The CLI routes `soul compliance <cmd>` to the matching tool.

**Tech Stack:** TypeScript ESM, tree-sitter WASM, js-yaml, diff, Puppeteer (optional), Vitest

**Design doc:** `docs/plans/2026-02-26-compliance-product-design.md`

**Existing codebase context:**
- `packages/plugins/src/schema.ts` — ToolDefinition, ToolResult, Artifact, CommandDefinition interfaces
- `packages/plugins/src/registry.ts` — PluginRegistry with addTool(), addCommand()
- `packages/context/src/scanner.ts` — scanDirectory() returns ScannedFile[]
- `packages/core/src/tiers.ts` — requireTier(), TierError, getCurrentTier()
- `packages/ui/src/components/findings-table.tsx` — FindingsTable component (Finding type: severity, rule, message, file, line)
- `packages/ui/src/components/approval-prompt.tsx` — ApprovalPrompt component
- `packages/ui/src/components/upgrade-prompt.tsx` — UpgradePrompt component
- `apps/cli/src/main.ts` — Currently handles --version, --probe, --help only. Needs subcommand routing.
- `apps/cli/package.json` — Depends on all @soul/* packages. Needs @soul/compliance added.

---

## Task 1: Scaffold @soul/compliance package

**Files:**
- Create: `products/compliance/package.json`
- Create: `products/compliance/tsconfig.json`
- Create: `products/compliance/src/types.ts`
- Create: `products/compliance/src/index.ts`

**Step 1: Create package.json**

Create `products/compliance/package.json`:
```json
{
  "name": "@soul/compliance",
  "version": "0.1.0",
  "type": "module",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "scripts": {
    "build": "tsc",
    "test": "vitest run",
    "typecheck": "tsc --noEmit",
    "clean": "rm -rf dist"
  },
  "dependencies": {
    "@soul/core": "0.1.0",
    "@soul/plugins": "0.1.0",
    "@soul/context": "0.1.0",
    "@soul/ui": "0.1.0",
    "js-yaml": "4.1.0",
    "diff": "7.0.0"
  },
  "devDependencies": {
    "@types/js-yaml": "4.0.9",
    "@types/diff": "6.0.0",
    "typescript": "^5.7.3",
    "vitest": "^4.0.0"
  },
  "peerDependencies": {
    "puppeteer": ">=22.0.0"
  },
  "peerDependenciesMeta": {
    "puppeteer": { "optional": true }
  }
}
```

**Step 2: Create tsconfig.json**

Create `products/compliance/tsconfig.json`:
```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "outDir": "dist",
    "rootDir": "src",
    "resolveJsonModule": true
  },
  "include": ["src"]
}
```

**Step 3: Create types.ts**

Create `products/compliance/src/types.ts`:
```typescript
export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info';
export type Framework = 'soc2' | 'hipaa' | 'gdpr';

export interface Finding {
  id: string;
  title: string;
  description: string;
  severity: Severity;
  framework: Framework[];
  controlIds: string[];
  file?: string;
  line?: number;
  column?: number;
  evidence?: string;
  analyzer: string;
  fixable: boolean;
  fix?: FixSuggestion;
}

export interface FixSuggestion {
  description: string;
  patch: string;
}

export interface ScanResult {
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

export interface RuleDefinition {
  id: string;
  title: string;
  severity: Severity;
  analyzer: string;
  pattern: string;
  controls: string[];
  framework: Framework[];
  description: string;
  fixable: boolean;
}

export interface ScanOptions {
  directory: string;
  frameworks?: Framework[];
  severity?: Severity[];
  analyzers?: string[];
  exclude?: string[];
  format?: 'terminal' | 'json';
  output?: string;
}

export interface Analyzer {
  name: string;
  analyze(files: ScannedFile[], rules: RuleDefinition[]): Promise<Finding[]>;
}

// Re-export for convenience
import type { ScannedFile } from '@soul/context';
export type { ScannedFile };
```

**Step 4: Create stub index.ts**

Create `products/compliance/src/index.ts`:
```typescript
import type { PluginRegistry } from '@soul/plugins';

export function register(_registry: PluginRegistry): void {
  // Tools and commands will be registered as they are built
}

export type { Finding, ScanResult, Severity, Framework, RuleDefinition, ScanOptions } from './types.js';
```

**Step 5: Install dependencies and verify build**

Run:
```bash
cd ~/soul && npm install
npx turbo build --filter=@soul/compliance
```
Expected: Build succeeds with no errors.

**Step 6: Commit**

```bash
git add products/compliance/
git commit -m "feat(compliance): scaffold package with types and stub registration"
```

---

## Task 2: Rule definitions (YAML) and loader

**Files:**
- Create: `products/compliance/src/rules/soc2.yaml`
- Create: `products/compliance/src/rules/hipaa.yaml`
- Create: `products/compliance/src/rules/gdpr.yaml`
- Create: `products/compliance/src/rules/index.ts`
- Create: `products/compliance/__tests__/rules.test.ts`

**Step 1: Create soc2.yaml**

Create `products/compliance/src/rules/soc2.yaml` with ~40 rules covering:
- Access Control (CC6.1-CC6.8): auth mechanism, RBAC, MFA, session timeout (~10 rules)
- Encryption (CC6.1, CC6.7): data at rest, in transit, key management (~8 rules)
- Logging (CC7.1-CC7.4): audit trail, admin logging, log retention (~7 rules)
- Change Management (CC8.1): CI/CD, code review, deployment approval (~8 rules)
- Vendor Risk (CC9.1): CVEs, license compatibility, dep pinning (~7 rules)

Each rule follows this format:
```yaml
- id: SECRET-001
  title: Hardcoded credentials detected
  severity: critical
  analyzer: secret-scanner
  pattern: hardcoded-credential
  controls: [CC6.1, CC6.7]
  framework: [soc2]
  description: Secrets must not be hardcoded. Use environment variables or a secrets manager.
  fixable: true
```

**Step 2: Create hipaa.yaml**

Create `products/compliance/src/rules/hipaa.yaml` with ~25 rules covering HIPAA Security Rule (164.312):
- Access controls (164.312(a)): unique user ID, emergency access, automatic logoff, encryption
- Audit controls (164.312(b)): activity logs, access logging
- Integrity controls (164.312(c)): mechanism to authenticate ePHI
- Transmission security (164.312(e)): encryption in transit

Rules that overlap with SOC 2 (e.g., SECRET-001) use the same `id` and `pattern` but different `controls` and `framework: [hipaa]`.

**Step 3: Create gdpr.yaml**

Create `products/compliance/src/rules/gdpr.yaml` with ~18 rules covering Articles 25 and 32:
- Data protection by design (Art 25): input validation, encryption, access control
- Security of processing (Art 32): encryption, pseudonymisation, resilience, testing

**Step 4: Write the failing test**

Create `products/compliance/__tests__/rules.test.ts`:
```typescript
import { describe, it, expect } from 'vitest';
import { loadRules } from '../src/rules/index.js';

describe('Rule loader', () => {
  it('loads all rules from YAML files', () => {
    const rules = loadRules();
    expect(rules.length).toBeGreaterThanOrEqual(60);
  });

  it('every rule has required fields', () => {
    const rules = loadRules();
    for (const rule of rules) {
      expect(rule.id).toBeTruthy();
      expect(rule.title).toBeTruthy();
      expect(rule.severity).toMatch(/^(critical|high|medium|low|info)$/);
      expect(rule.analyzer).toBeTruthy();
      expect(rule.pattern).toBeTruthy();
      expect(rule.controls.length).toBeGreaterThan(0);
      expect(rule.framework.length).toBeGreaterThan(0);
      expect(rule.description).toBeTruthy();
      expect(typeof rule.fixable).toBe('boolean');
    }
  });

  it('has no duplicate rule IDs within same framework', () => {
    const rules = loadRules();
    const seen = new Map<string, Set<string>>();
    for (const rule of rules) {
      for (const fw of rule.framework) {
        const set = seen.get(fw) ?? new Set();
        expect(set.has(rule.id), `Duplicate ${rule.id} in ${fw}`).toBe(false);
        set.add(rule.id);
        seen.set(fw, set);
      }
    }
  });

  it('filters rules by framework', () => {
    const soc2 = loadRules({ frameworks: ['soc2'] });
    const hipaa = loadRules({ frameworks: ['hipaa'] });
    expect(soc2.every((r) => r.framework.includes('soc2'))).toBe(true);
    expect(hipaa.every((r) => r.framework.includes('hipaa'))).toBe(true);
  });
});
```

**Step 5: Run test to verify it fails**

Run: `cd ~/soul && npx vitest run products/compliance/__tests__/rules.test.ts`
Expected: FAIL — module not found

**Step 6: Implement rule loader**

Create `products/compliance/src/rules/index.ts`:
```typescript
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import yaml from 'js-yaml';
import type { RuleDefinition, Framework } from '../types.js';

const __dirname = dirname(fileURLToPath(import.meta.url));

export function loadRules(options?: { frameworks?: Framework[] }): RuleDefinition[] {
  const files = ['soc2.yaml', 'hipaa.yaml', 'gdpr.yaml'];
  const allRules: RuleDefinition[] = [];

  for (const file of files) {
    const raw = readFileSync(join(__dirname, file), 'utf-8');
    const parsed = yaml.load(raw) as RuleDefinition[];
    allRules.push(...parsed);
  }

  if (options?.frameworks?.length) {
    return allRules.filter((r) => r.framework.some((f) => options.frameworks!.includes(f)));
  }
  return allRules;
}
```

Note: Since YAML files are in `src/rules/` but the loader runs from `dist/rules/`, the YAML files need to be copied to dist. Add a `postbuild` script or use `tsc` with asset copying. The simplest approach: add a copy step to the build script in package.json:
```json
"build": "tsc && cp src/rules/*.yaml dist/rules/"
```

**Step 7: Run tests to verify they pass**

Run: `cd ~/soul && npm install && npx turbo build --filter=@soul/compliance && npx vitest run products/compliance/__tests__/rules.test.ts`
Expected: 4 tests pass

**Step 8: Commit**

```bash
git add products/compliance/src/rules/ products/compliance/__tests__/rules.test.ts products/compliance/package.json
git commit -m "feat(compliance): add rule definitions for SOC2, HIPAA, GDPR with YAML loader"
```

---

## Task 3: Secret scanner analyzer

**Files:**
- Create: `products/compliance/src/analyzers/secret-scanner.ts`
- Create: `products/compliance/__tests__/analyzers/secret-scanner.test.ts`
- Create: `products/compliance/__tests__/fixtures/vulnerable-app/server.ts`
- Create: `products/compliance/__tests__/fixtures/vulnerable-app/config.ts`
- Create: `products/compliance/__tests__/fixtures/compliant-app/server.ts`

**Step 1: Create test fixtures**

Create `products/compliance/__tests__/fixtures/vulnerable-app/server.ts`:
```typescript
// Intentionally insecure — test fixture
const AWS_ACCESS_KEY = 'AKIAIOSFODNN7EXAMPLE';
const AWS_SECRET_KEY = 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY';
const GITHUB_TOKEN = 'ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef12';
const DB_PASSWORD = 'password = "super_secret_123"';
const PRIVATE_KEY = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF8PbnGy0AHB7MhgHcTz6sE2I2yPB
-----END RSA PRIVATE KEY-----`;
const JWT_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U';
const SLACK_TOKEN = 'xoxb-123456789012-1234567890123-abcdefghijklmnopqrstuvwx';

export function getConnection() {
  return { host: 'db.example.com', password: DB_PASSWORD };
}
```

Create `products/compliance/__tests__/fixtures/vulnerable-app/config.ts`:
```typescript
// More secrets
const API_KEY = 'sk-ant-api03-abcdefghijklmnopqrstuvwxyz';
const STRIPE_KEY = 'sk_live_abcdefghijklmnopqrstuvwxyz1234567890';
export const config = { apiKey: API_KEY, stripeKey: STRIPE_KEY };
```

Create `products/compliance/__tests__/fixtures/compliant-app/server.ts`:
```typescript
// Properly secured — test fixture
const DB_PASSWORD = process.env.DB_PASSWORD;
const API_KEY = process.env.API_KEY;

export function getConnection() {
  return { host: process.env.DB_HOST, password: DB_PASSWORD };
}
```

**Step 2: Write the failing test**

Create `products/compliance/__tests__/analyzers/secret-scanner.test.ts`:
```typescript
import { describe, it, expect } from 'vitest';
import { SecretScanner } from '../../src/analyzers/secret-scanner.js';
import { scanDirectory } from '@soul/context';
import { loadRules } from '../../src/rules/index.js';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const fixturesDir = join(__dirname, '..', 'fixtures');

describe('SecretScanner', () => {
  const scanner = new SecretScanner();
  const rules = loadRules().filter((r) => r.analyzer === 'secret-scanner');

  it('detects hardcoded secrets in vulnerable app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'vulnerable-app'));
    const findings = await scanner.analyze(files, rules);
    expect(findings.length).toBeGreaterThanOrEqual(5);
    expect(findings.every((f) => f.analyzer === 'secret-scanner')).toBe(true);
    const ids = findings.map((f) => f.id);
    expect(ids).toContain('SECRET-001'); // hardcoded credential
  });

  it('produces no findings for compliant app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'compliant-app'));
    const findings = await scanner.analyze(files, rules);
    expect(findings.length).toBe(0);
  });

  it('redacts evidence in findings', async () => {
    const files = await scanDirectory(join(fixturesDir, 'vulnerable-app'));
    const findings = await scanner.analyze(files, rules);
    for (const f of findings) {
      if (f.evidence) {
        expect(f.evidence).toContain('****');
      }
    }
  });

  it('detects high-entropy strings', async () => {
    const files = await scanDirectory(join(fixturesDir, 'vulnerable-app'));
    const findings = await scanner.analyze(files, rules);
    // JWT tokens should be caught by entropy or pattern
    const jwtFindings = findings.filter((f) => f.evidence?.includes('eyJ'));
    expect(jwtFindings.length).toBeGreaterThanOrEqual(1);
  });
});
```

**Step 3: Run test to verify it fails**

Run: `cd ~/soul && npx turbo build --filter=@soul/compliance && npx vitest run products/compliance/__tests__/analyzers/secret-scanner.test.ts`
Expected: FAIL — module not found

**Step 4: Implement secret scanner**

Create `products/compliance/src/analyzers/secret-scanner.ts`:

The scanner should:
1. Define ~30 regex patterns (AWS, GitHub, Slack, Stripe, private keys, JWTs, generic passwords, etc.)
2. Implement Shannon entropy calculation
3. Read each file's content, run all patterns, collect matches
4. For high-entropy: scan for hex strings >20 chars (entropy >4.5) and base64 strings >20 chars (entropy >5.0)
5. Redact evidence: show first 4 chars + `****`
6. Map each match to the appropriate rule by `pattern` field
7. Return Finding[] with file, line, evidence, severity from rule

Key implementation details:
- Skip binary files (check for null bytes in first 512 bytes)
- Skip files >500KB
- Only scan text-like extensions: ts, js, py, go, java, rb, yaml, yml, json, toml, env, cfg, conf, ini, xml, properties
- Shannon entropy: `H(X) = -Σ p(x) log2(p(x))` over character frequencies

**Step 5: Run tests to verify they pass**

Run: `cd ~/soul && npx turbo build --filter=@soul/compliance && npx vitest run products/compliance/__tests__/analyzers/secret-scanner.test.ts`
Expected: 4 tests pass

**Step 6: Commit**

```bash
git add products/compliance/src/analyzers/secret-scanner.ts products/compliance/__tests__/
git commit -m "feat(compliance): add secret scanner with regex patterns and entropy detection"
```

---

## Task 4: Config checker analyzer

**Files:**
- Create: `products/compliance/src/analyzers/config-checker.ts`
- Create: `products/compliance/__tests__/analyzers/config-checker.test.ts`
- Create: fixture files for config testing

**Step 1: Add fixture files**

Add to `vulnerable-app/`:
- `package.json` with `"express": "^4.18.0"` (unpinned), no `engines` field
- `.env` file with `DB_PASSWORD=secret123`
- `Dockerfile` with `USER root`, no HEALTHCHECK
- `.gitignore` that does NOT include `.env`

Add to `compliant-app/`:
- `package.json` with pinned deps, `engines` set
- `.gitignore` that includes `.env`
- `Dockerfile` with non-root USER, HEALTHCHECK present

**Step 2: Write the failing test**

Create `products/compliance/__tests__/analyzers/config-checker.test.ts`:
```typescript
import { describe, it, expect } from 'vitest';
import { ConfigChecker } from '../../src/analyzers/config-checker.js';
import { scanDirectory } from '@soul/context';
import { loadRules } from '../../src/rules/index.js';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const fixturesDir = join(__dirname, '..', 'fixtures');

describe('ConfigChecker', () => {
  const checker = new ConfigChecker();
  const rules = loadRules().filter((r) => r.analyzer === 'config-checker');

  it('detects config issues in vulnerable app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'vulnerable-app'));
    const findings = await checker.analyze(files, rules);
    expect(findings.length).toBeGreaterThanOrEqual(3);
    const ids = findings.map((f) => f.id);
    expect(ids).toContain('CONFIG-001'); // .env committed
  });

  it('produces no findings for compliant app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'compliant-app'));
    const findings = await checker.analyze(files, rules);
    expect(findings.length).toBe(0);
  });
});
```

**Step 3: Implement config checker**

Create `products/compliance/src/analyzers/config-checker.ts`:

Checks by file type:
- `.env` files: check if in .gitignore
- `package.json`: unpinned deps (^ or ~), missing engines, missing lockfile (check for package-lock.json)
- `Dockerfile`/`docker-compose.yml`: USER root, no HEALTHCHECK, privileged mode
- Config files (nginx.conf, etc.): missing security headers via regex

**Step 4: Run tests, verify pass, commit**

```bash
git commit -m "feat(compliance): add config checker for env, package.json, Docker security"
```

---

## Task 5: Git analyzer

**Files:**
- Create: `products/compliance/src/analyzers/git-analyzer.ts`
- Create: `products/compliance/__tests__/analyzers/git-analyzer.test.ts`

**Step 1: Write failing test**

Tests:
- Detects missing .gitignore entries (.env not in .gitignore) in vulnerable fixture
- Detects missing CODEOWNERS
- Clean for compliant fixture (which has proper .gitignore and CODEOWNERS)

**Step 2: Implement git analyzer**

Checks:
- .gitignore missing common entries: .env, node_modules, dist, *.pem, *.key
- No CODEOWNERS file
- No .github/CODEOWNERS or docs/CODEOWNERS
- Large files (>5MB) tracked in git (check file sizes in ScannedFile[])

Does not shell out to git commands — only inspects files that were scanned. This keeps it pure and testable with fixtures.

**Step 3: Run tests, commit**

```bash
git commit -m "feat(compliance): add git analyzer for gitignore, CODEOWNERS checks"
```

---

## Task 6: Dependency auditor

**Files:**
- Create: `products/compliance/src/analyzers/dep-auditor.ts`
- Create: `products/compliance/__tests__/analyzers/dep-auditor.test.ts`

**Step 1: Write failing test**

Tests:
- Detects unpinned deps in vulnerable fixture's package.json
- Detects missing lockfile
- Clean for compliant fixture

**Step 2: Implement dependency auditor**

Checks (static, no shelling out to npm audit for unit tests):
- Parse package.json: flag deps with `^` or `~` in production dependencies
- Check for package-lock.json or npm-shrinkwrap.json existence
- Parse package-lock.json if present: check for known problematic packages (a small built-in list of critically vulnerable packages)
- License check: read license field, flag GPL/AGPL in dependencies

Shell-out checks (only when running for real, not in unit tests):
- `npm audit --json` if available
- `osv-scanner --json` if available
- Graceful fallback — if commands not available, skip those checks

**Step 3: Run tests, commit**

```bash
git commit -m "feat(compliance): add dependency auditor for pinning, lockfile, license checks"
```

---

## Task 7: AST analyzer (tree-sitter)

**Files:**
- Create: `products/compliance/src/analyzers/ast-analyzer.ts`
- Create: `products/compliance/__tests__/analyzers/ast-analyzer.test.ts`
- Modify: `products/compliance/package.json` (add web-tree-sitter dep)

**Step 1: Add fixture code with security issues**

Add to `vulnerable-app/`:
- `auth.ts`: express routes with no auth middleware, no session timeout
- `crypto.ts`: MD5 hashing, ECB mode, hardcoded IV
- `db.ts`: SQL string concatenation (`"SELECT * FROM users WHERE id = " + userId`)
- `handler.ts`: empty catch blocks, stack trace exposed to client

Add to `compliant-app/`:
- `auth.ts`: auth middleware on all routes, session config with timeout
- `crypto.ts`: bcrypt, AES-256-GCM
- `db.ts`: parameterized queries
- `handler.ts`: proper error handling

**Step 2: Write failing test**

Tests:
- Detects SQL injection pattern in vulnerable app
- Detects weak crypto in vulnerable app
- Detects empty catch blocks
- Clean for compliant app

**Step 3: Implement AST analyzer**

Uses `web-tree-sitter` with WASM grammars. For v0.1.0, ship TypeScript/JavaScript grammar only (most users are Node.js). Add Python/Go/Java grammars later.

Implementation:
1. Initialize tree-sitter WASM once (lazy singleton)
2. Parse each .ts/.js file into AST
3. Run tree-sitter queries for each pattern:
   - SQL injection: query for string concatenation inside function calls named `query`, `exec`, `execute`
   - Weak crypto: query for `createHash('md5')`, `createHash('sha1')`, `createCipheriv('aes-128-ecb', ...)`
   - Empty catch: query for catch clauses with empty block statements
   - Missing auth: query for express `.get()`, `.post()` calls — heuristic, check if first arg is a path and there's no middleware arg
4. Fallback to regex for files tree-sitter can't parse

Note: tree-sitter WASM initialization is async. The analyzer must handle this.

If `web-tree-sitter` causes issues on aarch64 (RPi), fall back to pure regex patterns. The analyzer should have a `useTreeSitter: boolean` flag that defaults to true but falls back gracefully.

**Step 4: Run tests, commit**

```bash
git commit -m "feat(compliance): add AST analyzer with tree-sitter WASM for TS/JS"
```

---

## Task 8: Scan orchestrator tool

**Files:**
- Create: `products/compliance/src/tools/scan.ts`
- Create: `products/compliance/__tests__/scan.test.ts`

**Step 1: Write failing integration test**

```typescript
import { describe, it, expect } from 'vitest';
import { runScan } from '../src/tools/scan.js';

describe('Scan orchestrator', () => {
  it('scans vulnerable app and returns findings', async () => {
    const result = await runScan({ directory: join(fixturesDir, 'vulnerable-app') });
    expect(result.findings.length).toBeGreaterThan(0);
    expect(result.summary.total).toBe(result.findings.length);
    expect(result.summary.bySeverity.critical).toBeGreaterThan(0);
    expect(result.metadata.analyzersRun.length).toBe(5);
  });

  it('scans compliant app with minimal findings', async () => {
    const result = await runScan({ directory: join(fixturesDir, 'compliant-app') });
    expect(result.findings.filter((f) => f.severity === 'critical').length).toBe(0);
  });

  it('filters by framework', async () => {
    const result = await runScan({
      directory: join(fixturesDir, 'vulnerable-app'),
      frameworks: ['hipaa'],
    });
    expect(result.findings.every((f) => f.framework.includes('hipaa'))).toBe(true);
  });

  it('deduplicates findings on same file+line+id', async () => {
    const result = await runScan({ directory: join(fixturesDir, 'vulnerable-app') });
    const keys = result.findings.map((f) => `${f.file}:${f.line}:${f.id}`);
    expect(new Set(keys).size).toBe(keys.length);
  });
});
```

**Step 2: Implement scan orchestrator**

Create `products/compliance/src/tools/scan.ts`:

```typescript
export async function runScan(options: ScanOptions): Promise<ScanResult> {
  const start = Date.now();
  const files = await scanDirectory(options.directory);
  const rules = loadRules({ frameworks: options.frameworks });

  const analyzers: Analyzer[] = [
    new SecretScanner(),
    new ConfigChecker(),
    new AstAnalyzer(),
    new GitAnalyzer(),
    new DepAuditor(),
  ];

  // Filter analyzers if specific ones requested
  const active = options.analyzers
    ? analyzers.filter((a) => options.analyzers!.includes(a.name))
    : analyzers;

  // Run all in parallel
  const results = await Promise.allSettled(
    active.map((a) => a.analyze(files, rules.filter((r) => r.analyzer === a.name)))
  );

  // Collect findings, skip failed analyzers
  let findings: Finding[] = [];
  const analyzersRun: string[] = [];
  for (let i = 0; i < results.length; i++) {
    const result = results[i];
    if (result.status === 'fulfilled') {
      findings.push(...result.value);
      analyzersRun.push(active[i].name);
    }
  }

  // Deduplicate
  findings = deduplicateFindings(findings);

  // Build summary
  return { findings, summary: buildSummary(findings), metadata: { ... } };
}
```

**Step 3: Register as a PluginRegistry tool**

In `scan.ts`, also export a `createScanTool()` function that returns a `ToolDefinition` compatible with the registry. The tool's `execute()` parses input via Zod schema, calls `runScan()`, and formats output.

**Step 4: Run tests, commit**

```bash
git commit -m "feat(compliance): add scan orchestrator with parallel analyzer execution"
```

---

## Task 9: Terminal and JSON reporters

**Files:**
- Create: `products/compliance/src/reporters/terminal.ts`
- Create: `products/compliance/src/reporters/json.ts`
- Create: `products/compliance/__tests__/reporters.test.ts`

**Step 1: Write failing test**

Tests:
- Terminal reporter returns formatted string with severity sections
- JSON reporter returns valid JSON matching ScanResult schema
- JSON reporter output is parseable back to ScanResult

**Step 2: Implement terminal reporter**

Formats ScanResult into a colored terminal string using chalk (from @soul/ui theme). Groups findings by severity, shows file:line references, and summary line at the end.

Note: This is a string formatter, NOT an Ink component. The Ink `<FindingsTable />` in @soul/ui is for interactive mode. The reporter is for `soul compliance scan` non-interactive output.

**Step 3: Implement JSON reporter**

Serializes ScanResult to pretty-printed JSON. Simple `JSON.stringify(result, null, 2)`.

**Step 4: Run tests, commit**

```bash
git commit -m "feat(compliance): add terminal and JSON reporters"
```

---

## Task 10: Badge generator

**Files:**
- Create: `products/compliance/src/reporters/badge.ts`
- Create: `products/compliance/src/tools/badge.ts`
- Create: `products/compliance/__tests__/badge.test.ts`

**Step 1: Write failing test**

Tests:
- Generates valid SVG
- Score calculation: (totalRules - findings) / totalRules * 100
- Color: green >80%, yellow 60-80%, red <60%
- Contains framework name in badge text

**Step 2: Implement badge generator**

SVG template string approach (shields.io style). Takes ScanResult, calculates score, renders SVG with appropriate color. Writes to `compliance-badge.svg` in the scanned directory.

**Step 3: Register as tool, run tests, commit**

```bash
git commit -m "feat(compliance): add SVG badge generator"
```

---

## Task 11: HTML and PDF reporters

**Files:**
- Create: `products/compliance/src/reporters/html.ts`
- Create: `products/compliance/src/reporters/pdf.ts`
- Create: `products/compliance/src/tools/report.ts`
- Create: `products/compliance/__tests__/report.test.ts`

**Step 1: Write failing test**

Tests:
- HTML output contains expected sections (executive summary, findings tables)
- HTML is valid (contains `<html>`, `</html>`, `<style>`)
- PDF tool checks tier gate (throws TierError for free tier)
- Report tool registers correctly in registry

**Step 2: Implement HTML reporter**

Standalone HTML template with embedded CSS. Sections:
- Executive summary with score, scan date, framework coverage
- Findings by severity (collapsible sections)
- Findings by framework
- Inline SVG charts (bar chart of severity distribution)
- Soul branding in header/footer

All in a single template literal function — no template engine.

**Step 3: Implement PDF reporter**

Dynamic import of puppeteer. If not installed, throw helpful error. Otherwise, launch headless browser, load HTML string, print to PDF.

```typescript
export async function generatePdf(html: string, outputPath: string): Promise<void> {
  let puppeteer;
  try {
    puppeteer = await import('puppeteer');
  } catch {
    throw new Error('PDF generation requires puppeteer. Run: npm i -g puppeteer');
  }
  const browser = await puppeteer.default.launch({ headless: true });
  const page = await browser.newPage();
  await page.setContent(html, { waitUntil: 'networkidle0' });
  await page.pdf({ path: outputPath, format: 'A4', printBackground: true });
  await browser.close();
}
```

**Step 4: Create report tool with tier gate**

The report tool calls `requireTier('pro', 'HTML/PDF reports')` before generating. Free tier gets terminal/JSON only.

**Step 5: Run tests, commit**

```bash
git commit -m "feat(compliance): add HTML/PDF report generation with tier gate"
```

---

## Task 12: Auto-remediation (fix) tool

**Files:**
- Create: `products/compliance/src/tools/fix.ts`
- Create: `products/compliance/__tests__/fix.test.ts`

**Step 1: Write failing test**

Tests:
- Generates valid unified diff patches for fixable findings
- Dry-run mode returns patches without applying
- Tier gate: throws TierError for free tier
- Patches can be applied (test with temp directory copy)

**Step 2: Implement fix tool**

Flow:
1. Run scan (or accept ScanResult from input)
2. Filter to fixable findings
3. For each finding, generate a patch using the `diff` package:
   - Secret → replace with `process.env.VARIABLE_NAME`
   - Missing .gitignore entry → append line
   - Unpinned dep → replace `^version` with `version` from lockfile
   - Weak crypto → replace MD5 with SHA-256 (template replacement)
4. Return patches in dry-run mode, or apply via writing patched files

Note: Use the `diff` npm package to create unified diffs. For applying, read original file, apply changes, write back. Don't shell out to `git apply` — keep it pure Node.js for testability.

**Step 3: Run tests, commit**

```bash
git commit -m "feat(compliance): add auto-remediation with patch generation and tier gate"
```

---

## Task 13: Monitor mode tool

**Files:**
- Create: `products/compliance/src/tools/monitor.ts`
- Create: `products/compliance/__tests__/monitor.test.ts`

**Step 1: Write failing test**

Tests:
- Monitor starts and detects file changes (use temp dir, write file, assert callback fires)
- Debounces rapid changes
- Tier gate: throws TierError for free tier

**Step 2: Implement monitor**

Uses `fs.watch` recursive. On change:
1. Debounce 500ms with a timer
2. Re-scan only changed files (filter ScannedFile[] to those matching changed paths)
3. Diff against previous ScanResult
4. Output only new/resolved findings

For the terminal UI, output formatted text (not Ink interactive — that's for interactive mode which is future work).

**Step 3: Run tests, commit**

```bash
git commit -m "feat(compliance): add file watch monitor mode with debounce and tier gate"
```

---

## Task 14: Product registration and CLI routing

**Files:**
- Modify: `products/compliance/src/index.ts` — register all 5 tools and commands
- Modify: `apps/cli/src/main.ts` — add subcommand routing
- Modify: `apps/cli/package.json` — add @soul/compliance dependency
- Create: `products/compliance/__tests__/registration.test.ts`

**Step 1: Write failing test**

```typescript
import { describe, it, expect } from 'vitest';
import { PluginRegistry } from '@soul/plugins';
import { register } from '../src/index.js';

describe('Compliance registration', () => {
  it('registers all 5 tools', () => {
    const registry = new PluginRegistry();
    register(registry);
    const tools = registry.getToolsByProduct('compliance');
    expect(tools.length).toBe(5);
    expect(tools.map((t) => t.name).sort()).toEqual([
      'compliance.badge', 'compliance.fix', 'compliance.monitor',
      'compliance.report', 'compliance.scan',
    ]);
  });

  it('registers CLI commands', () => {
    const registry = new PluginRegistry();
    register(registry);
    const cmds = registry.getCommandsByProduct('compliance');
    expect(cmds.length).toBe(5);
  });
});
```

**Step 2: Implement registration**

Update `products/compliance/src/index.ts` to import all tool creators and register them.

**Step 3: Update CLI**

Modify `apps/cli/src/main.ts` to:
1. Import `register` from `@soul/compliance`
2. Create PluginRegistry, call `register(registry)`
3. Route `soul compliance <cmd>` to the matching tool via `registry.getTool('compliance.<cmd>')`
4. Parse flags into tool input
5. Call `tool.execute(input)` and print result

Add `"@soul/compliance": "0.1.0"` to `apps/cli/package.json` dependencies.

**Step 4: Run full build and tests**

```bash
cd ~/soul && npm install && npx turbo build && npx turbo test
```

**Step 5: Commit**

```bash
git commit -m "feat(compliance): register tools and add CLI subcommand routing"
```

---

## Task 15: Full integration test and cleanup

**Files:**
- Create: `products/compliance/__tests__/integration.test.ts`
- Verify all existing tests pass

**Step 1: Write integration test**

End-to-end test that exercises the full flow:
1. Run `runScan()` on vulnerable fixture → assert findings
2. Run `runScan()` on compliant fixture → assert clean
3. Generate JSON report → parse and validate
4. Generate badge → check SVG validity
5. Generate fix patches in dry-run → verify patches exist

**Step 2: Run full test suite**

```bash
cd ~/soul && npx turbo build --force && npx turbo test --force && npx turbo typecheck
```

All tests must pass across all packages.

**Step 3: Verify CLI works end-to-end**

```bash
soul compliance scan ~/soul --format json
soul compliance badge ~/soul
soul compliance scan ~/soul
```

**Step 4: Commit and push**

```bash
git add -A
git commit -m "test(compliance): add integration tests and verify full pipeline"
git push github master
```
