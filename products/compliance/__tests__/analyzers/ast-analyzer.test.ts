import { describe, it, expect } from 'vitest';
import { AstAnalyzer } from '../../src/analyzers/ast-analyzer.js';
import { scanDirectory } from '@soul/context';
import { loadRules } from '../../src/rules/index.js';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const fixturesDir = join(__dirname, '..', 'fixtures');

describe('AstAnalyzer', () => {
  const analyzer = new AstAnalyzer();
  const rules = loadRules().filter((r) => r.analyzer === 'ast-analyzer');

  it('detects SQL injection in vulnerable app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'vulnerable-app'));
    const findings = await analyzer.analyze(files, rules);
    const sqlFindings = findings.filter((f) => f.id === 'INJ-001');
    expect(sqlFindings.length).toBeGreaterThanOrEqual(1);
  });

  it('detects weak crypto in vulnerable app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'vulnerable-app'));
    const findings = await analyzer.analyze(files, rules);
    const cryptoFindings = findings.filter((f) => f.id === 'CRYPTO-001');
    expect(cryptoFindings.length).toBeGreaterThanOrEqual(1);
  });

  it('detects empty catch blocks', async () => {
    const files = await scanDirectory(join(fixturesDir, 'vulnerable-app'));
    const findings = await analyzer.analyze(files, rules);
    const catchFindings = findings.filter((f) => f.id === 'LOG-004');
    expect(catchFindings.length).toBeGreaterThanOrEqual(1);
  });

  it('produces minimal findings for compliant app', async () => {
    const files = await scanDirectory(join(fixturesDir, 'compliant-app'));
    const findings = await analyzer.analyze(files, rules);
    const critical = findings.filter((f) => f.severity === 'critical' || f.severity === 'high');
    expect(critical.length).toBe(0);
  });
});
