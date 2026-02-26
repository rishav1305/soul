import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, writeFileSync, readFileSync, mkdirSync, cpSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { rmSync } from 'node:fs';
import {
  createFixTool,
  generatePatches,
  applyPatches,
  generateSecretFix,
  generateGitignoreFix,
  generateWeakHashFix,
  generateUnpinnedDepsFix,
  deriveEnvVarName,
  runFix,
} from '../src/tools/fix.js';
import type { Finding, ScanResult } from '../src/types.js';

const fixturesDir = join(import.meta.dirname, 'fixtures');

// ── Helpers ─────────────────────────────────────────────────────────────

function makeFinding(overrides: Partial<Finding> = {}): Finding {
  return {
    id: 'SECRET-001',
    title: 'Hardcoded credential',
    description: 'A secret was found',
    severity: 'critical',
    framework: ['soc2'],
    controlIds: ['CC6.1'],
    analyzer: 'secret-scanner',
    fixable: true,
    ...overrides,
  };
}

// ── Tests ───────────────────────────────────────────────────────────────

describe('Fix tool tier gate', () => {
  it('throws TierError for free tier', async () => {
    const tool = createFixTool();

    // Without credentials file, getCurrentTier() returns 'free',
    // so requireTier('pro', ...) will throw TierError.
    await expect(
      tool.execute({ directory: '/tmp/test', dryRun: true }),
    ).rejects.toThrow('Auto-remediation requires Soul pro');
  });

  it('registers correctly with expected name and product', () => {
    const tool = createFixTool();

    expect(tool.name).toBe('compliance-fix');
    expect(tool.product).toBe('compliance');
    expect(tool.description).toContain('remediat');
    expect(tool.requiresApproval).toBe(true);
    expect(typeof tool.execute).toBe('function');
  });
});

describe('deriveEnvVarName', () => {
  it('extracts variable name from const assignment', () => {
    expect(deriveEnvVarName("const API_KEY = 'sk-ant-api03-abc'")).toBe('API_KEY');
  });

  it('converts camelCase to UPPER_SNAKE_CASE', () => {
    expect(deriveEnvVarName("const stripeKey = 'sk_live_abc123'")).toBe('STRIPE_KEY');
  });

  it('handles export const', () => {
    expect(deriveEnvVarName("export const dbPassword = 'secret'")).toBe('DB_PASSWORD');
  });

  it('falls back to property name for object assignments', () => {
    expect(deriveEnvVarName("  apiKey: 'sk-abc123'")).toBe('API_KEY');
  });

  it('returns SECRET_VALUE as fallback', () => {
    expect(deriveEnvVarName('some random line')).toBe('SECRET_VALUE');
  });
});

describe('generateSecretFix', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fix-secret-'));
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('generates a valid unified diff for a hardcoded secret', () => {
    const filePath = join(tmpDir, 'config.ts');
    writeFileSync(filePath, "const API_KEY = 'sk-ant-api03-abcdefghijklmnopqrstuvwxyz';\n", 'utf-8');

    const finding = makeFinding({
      file: 'config.ts',
      line: 1,
      evidence: 'sk-a****wxyz',
    });

    const patch = generateSecretFix(filePath, finding);

    expect(patch).not.toBeNull();
    expect(patch!.patch).toContain('---');
    expect(patch!.patch).toContain('+++');
    expect(patch!.patch).toContain('@@');
    expect(patch!.patch).toContain('process.env.API_KEY');
    expect(patch!.description).toContain('process.env.API_KEY');
  });

  it('returns null when line does not contain a quoted string', () => {
    const filePath = join(tmpDir, 'empty.ts');
    writeFileSync(filePath, 'const x = 42;\n', 'utf-8');

    const finding = makeFinding({ file: 'empty.ts', line: 1 });
    const patch = generateSecretFix(filePath, finding);

    expect(patch).toBeNull();
  });
});

describe('generateGitignoreFix', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fix-gitignore-'));
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('appends missing entries to .gitignore', () => {
    writeFileSync(join(tmpDir, '.gitignore'), 'node_modules\ndist\n', 'utf-8');

    const finding = makeFinding({
      id: 'CFG-001',
      analyzer: 'config-checker',
      evidence: '.env not listed in .gitignore',
    });

    const patch = generateGitignoreFix(tmpDir, finding);

    expect(patch).not.toBeNull();
    expect(patch!.patch).toContain('---');
    expect(patch!.patch).toContain('+++');
    expect(patch!.patch).toContain('@@');
    expect(patch!.patch).toContain('.env');
    expect(patch!.patch).toContain('*.pem');
    expect(patch!.patch).toContain('*.key');
  });

  it('does not duplicate entries already in .gitignore', () => {
    writeFileSync(join(tmpDir, '.gitignore'), 'node_modules\n.env\n*.pem\n*.key\n', 'utf-8');

    const finding = makeFinding({
      id: 'CFG-001',
      analyzer: 'config-checker',
      evidence: '.env not listed in .gitignore',
    });

    const patch = generateGitignoreFix(tmpDir, finding);
    expect(patch).toBeNull();
  });

  it('creates .gitignore from scratch when it does not exist', () => {
    const finding = makeFinding({
      id: 'CFG-001',
      analyzer: 'config-checker',
      evidence: '.env not listed in .gitignore',
    });

    const patch = generateGitignoreFix(tmpDir, finding);

    expect(patch).not.toBeNull();
    expect(patch!.patch).toContain('.env');
    expect(patch!.patch).toContain('*.pem');
    expect(patch!.patch).toContain('*.key');
  });
});

describe('generateWeakHashFix', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fix-weakhash-'));
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('replaces MD5 with SHA-256 in unified diff', () => {
    const filePath = join(tmpDir, 'crypto.ts');
    writeFileSync(
      filePath,
      "import crypto from 'crypto';\nconst hash = crypto.createHash('md5').update('data').digest('hex');\n",
      'utf-8',
    );

    const finding = makeFinding({
      id: 'AST-001',
      analyzer: 'ast-analyzer',
      file: 'crypto.ts',
      line: 2,
      evidence: "Weak hash: const hash = crypto.createHash('md5').update('data').digest('hex');",
      fixable: true,
    });

    const patch = generateWeakHashFix(filePath, finding);

    expect(patch).not.toBeNull();
    expect(patch!.patch).toContain('---');
    expect(patch!.patch).toContain('+++');
    expect(patch!.patch).toContain('@@');
    expect(patch!.patch).toContain("createHash('sha256')");
    expect(patch!.description).toContain('SHA-256');
  });

  it('replaces SHA1 with SHA-256', () => {
    const filePath = join(tmpDir, 'hash.ts');
    writeFileSync(
      filePath,
      "const h = crypto.createHash('sha1').update('x').digest('hex');\n",
      'utf-8',
    );

    const finding = makeFinding({
      id: 'AST-001',
      analyzer: 'ast-analyzer',
      file: 'hash.ts',
      line: 1,
      evidence: 'Weak hash: sha1',
      fixable: true,
    });

    const patch = generateWeakHashFix(filePath, finding);

    expect(patch).not.toBeNull();
    expect(patch!.patch).toContain("createHash('sha256')");
  });
});

describe('generateUnpinnedDepsFix', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fix-deps-'));
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('strips ^ and ~ from dependency versions', () => {
    const filePath = join(tmpDir, 'package.json');
    writeFileSync(
      filePath,
      JSON.stringify(
        {
          name: 'test',
          dependencies: {
            express: '^4.18.0',
            lodash: '~4.17.21',
            mysql: '^2.18.1',
          },
        },
        null,
        2,
      ) + '\n',
      'utf-8',
    );

    const finding = makeFinding({
      id: 'CFG-002',
      analyzer: 'config-checker',
      file: 'package.json',
      evidence: 'Unpinned: express, lodash, mysql',
      fixable: true,
    });

    const patch = generateUnpinnedDepsFix(filePath, finding);

    expect(patch).not.toBeNull();
    expect(patch!.patch).toContain('---');
    expect(patch!.patch).toContain('+++');
    expect(patch!.patch).toContain('@@');
    // The patched version should not contain ^ or ~ before version numbers
    expect(patch!.patch).toContain('"4.18.0"');
    expect(patch!.description).toContain('Pin dependency');
  });
});

describe('generatePatches (integration)', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fix-patches-'));
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('generates patches for multiple fixable finding types', () => {
    // Set up files
    writeFileSync(
      join(tmpDir, 'config.ts'),
      "const API_KEY = 'sk-ant-api03-abcdefghijklmnopqrstuvwxyz';\n",
      'utf-8',
    );
    writeFileSync(
      join(tmpDir, '.gitignore'),
      'node_modules\ndist\n',
      'utf-8',
    );
    writeFileSync(
      join(tmpDir, 'crypto.ts'),
      "import crypto from 'crypto';\nconst hash = crypto.createHash('md5').update('data').digest('hex');\n",
      'utf-8',
    );

    const findings: Finding[] = [
      makeFinding({
        id: 'SECRET-001',
        analyzer: 'secret-scanner',
        file: 'config.ts',
        line: 1,
        fixable: true,
      }),
      makeFinding({
        id: 'CFG-001',
        analyzer: 'config-checker',
        file: '.env',
        evidence: '.env not listed in .gitignore',
        fixable: true,
      }),
      makeFinding({
        id: 'AST-001',
        analyzer: 'ast-analyzer',
        file: 'crypto.ts',
        line: 2,
        evidence: "Weak hash: crypto.createHash('md5')",
        fixable: true,
      }),
      // Non-fixable finding should be skipped
      makeFinding({
        id: 'CFG-003',
        analyzer: 'config-checker',
        fixable: false,
      }),
    ];

    const patches = generatePatches(tmpDir, findings);

    // Should have 3 patches (secret, gitignore, weak hash)
    expect(patches.length).toBe(3);
    expect(patches.every((p) => p.patch.includes('---'))).toBe(true);
    expect(patches.every((p) => p.patch.includes('+++'))).toBe(true);
    expect(patches.every((p) => p.patch.includes('@@'))).toBe(true);
  });
});

describe('Dry-run mode', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fix-dryrun-'));
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('returns patches without modifying files', () => {
    const originalContent = "import crypto from 'crypto';\nconst hash = crypto.createHash('md5').update('data').digest('hex');\n";
    const filePath = join(tmpDir, 'crypto.ts');
    writeFileSync(filePath, originalContent, 'utf-8');

    const findings: Finding[] = [
      makeFinding({
        id: 'AST-001',
        analyzer: 'ast-analyzer',
        file: 'crypto.ts',
        line: 2,
        evidence: "Weak hash: createHash('md5')",
        fixable: true,
      }),
    ];

    const patches = generatePatches(tmpDir, findings);
    expect(patches.length).toBe(1);

    // File should not have been modified (dry-run is the default — generatePatches never writes)
    const afterContent = readFileSync(filePath, 'utf-8');
    expect(afterContent).toBe(originalContent);
  });
});

describe('Apply patches', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fix-apply-'));
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('applies weak hash fix to file', () => {
    const originalContent = "import crypto from 'crypto';\nconst hash = crypto.createHash('md5').update('data').digest('hex');\n";
    writeFileSync(join(tmpDir, 'crypto.ts'), originalContent, 'utf-8');

    const findings: Finding[] = [
      makeFinding({
        id: 'AST-001',
        analyzer: 'ast-analyzer',
        file: 'crypto.ts',
        line: 2,
        evidence: "Weak hash: createHash('md5')",
        fixable: true,
      }),
    ];

    const patches = generatePatches(tmpDir, findings);
    const applied = applyPatches(tmpDir, patches);

    expect(applied).toBe(1);

    const newContent = readFileSync(join(tmpDir, 'crypto.ts'), 'utf-8');
    expect(newContent).toContain("createHash('sha256')");
    expect(newContent).not.toContain("createHash('md5')");
  });

  it('applies gitignore fix to file', () => {
    writeFileSync(join(tmpDir, '.gitignore'), 'node_modules\ndist\n', 'utf-8');

    const findings: Finding[] = [
      makeFinding({
        id: 'CFG-001',
        analyzer: 'config-checker',
        file: '.env',
        evidence: '.env not listed in .gitignore',
        fixable: true,
      }),
    ];

    const patches = generatePatches(tmpDir, findings);
    const applied = applyPatches(tmpDir, patches);

    expect(applied).toBe(1);

    const newContent = readFileSync(join(tmpDir, '.gitignore'), 'utf-8');
    expect(newContent).toContain('.env');
    expect(newContent).toContain('*.pem');
    expect(newContent).toContain('*.key');
  });

  it('applies unpinned deps fix to package.json', () => {
    writeFileSync(
      join(tmpDir, 'package.json'),
      JSON.stringify(
        { name: 'test', dependencies: { express: '^4.18.0', lodash: '~4.17.21' } },
        null,
        2,
      ) + '\n',
      'utf-8',
    );

    const findings: Finding[] = [
      makeFinding({
        id: 'CFG-002',
        analyzer: 'config-checker',
        file: 'package.json',
        evidence: 'Unpinned: express, lodash',
        fixable: true,
      }),
    ];

    const patches = generatePatches(tmpDir, findings);
    const applied = applyPatches(tmpDir, patches);

    expect(applied).toBe(1);

    const newContent = readFileSync(join(tmpDir, 'package.json'), 'utf-8');
    expect(newContent).not.toMatch(/"[~^]\d/);
    expect(newContent).toContain('"4.18.0"');
    expect(newContent).toContain('"4.17.21"');
  });
});

describe('Full fix pipeline with fixtures', () => {
  let tmpDir: string;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fix-full-'));
    // Copy vulnerable-app fixtures to temp dir
    cpSync(join(fixturesDir, 'vulnerable-app'), tmpDir, { recursive: true });
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
  });

  it('scans and generates patches for vulnerable app in dry-run mode', async () => {
    const result = await runFix(tmpDir, true);

    expect(result.applied).toBe(false);
    expect(result.summary.totalFixable).toBeGreaterThan(0);
    expect(result.summary.patchesGenerated).toBeGreaterThan(0);
    expect(result.summary.patchesApplied).toBe(0);

    // All patches should be valid unified diffs
    for (const patch of result.patches) {
      expect(patch.patch).toContain('---');
      expect(patch.patch).toContain('+++');
      expect(patch.patch).toContain('@@');
    }

    // Files should not be modified
    const originalCrypto = readFileSync(join(fixturesDir, 'vulnerable-app', 'crypto.ts'), 'utf-8');
    const currentCrypto = readFileSync(join(tmpDir, 'crypto.ts'), 'utf-8');
    expect(currentCrypto).toBe(originalCrypto);
  });

  it('applies patches when dryRun is false', async () => {
    const result = await runFix(tmpDir, false);

    expect(result.applied).toBe(true);
    expect(result.summary.patchesApplied).toBeGreaterThan(0);

    // Verify at least one file was actually changed
    const cryptoContent = readFileSync(join(tmpDir, 'crypto.ts'), 'utf-8');
    // MD5 and SHA1 should be replaced with SHA-256
    expect(cryptoContent).not.toMatch(/createHash\s*\(\s*['"]md5['"]\s*\)/);
    expect(cryptoContent).toContain("createHash('sha256')");
  });
});
