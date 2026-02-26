import { describe, it, expect } from 'vitest';
import { DepAuditor } from '../../src/analyzers/dep-auditor.js';
import { scanDirectory } from '@soul/context';
import { loadRules } from '../../src/rules/index.js';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const fixturesDir = join(__dirname, '..', 'fixtures');

describe('DepAuditor', () => {
  const auditor = new DepAuditor();
  const rules = loadRules().filter((r) => r.analyzer === 'dep-auditor');

  it('detects dependency issues in vulnerable app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'vulnerable-app'));
    const findings = await auditor.analyze(files, rules);
    expect(findings.length).toBeGreaterThanOrEqual(2);
    const ids = findings.map((f) => f.id);
    expect(ids).toContain('CHANGE-003'); // unpinned deps
    expect(ids).toContain('CHANGE-002'); // missing lockfile
  });

  it('produces no findings for compliant app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'compliant-app'));
    const findings = await auditor.analyze(files, rules);
    expect(findings.length).toBe(0);
  });
});
