import { describe, it, expect } from 'vitest';
import { GitAnalyzer } from '../../src/analyzers/git-analyzer.js';
import { scanDirectory } from '@soul/context';
import { loadRules } from '../../src/rules/index.js';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const fixturesDir = join(__dirname, '..', 'fixtures');

describe('GitAnalyzer', () => {
  const analyzer = new GitAnalyzer();
  const rules = loadRules().filter((r) => r.analyzer === 'git-analyzer');

  it('detects git issues in vulnerable app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'vulnerable-app'));
    const findings = await analyzer.analyze(files, rules);
    expect(findings.length).toBeGreaterThanOrEqual(3);
    const ids = findings.map((f) => f.id);
    expect(ids).toContain('ACCESS-001'); // missing CODEOWNERS
  });

  it('produces no findings for compliant app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'compliant-app'));
    const findings = await analyzer.analyze(files, rules);
    expect(findings.length).toBe(0);
  });
});
