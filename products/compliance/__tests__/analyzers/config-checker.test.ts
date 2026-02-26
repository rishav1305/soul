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
  });

  it('produces no findings for compliant app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'compliant-app'));
    const findings = await checker.analyze(files, rules);
    expect(findings.length).toBe(0);
  });
});
