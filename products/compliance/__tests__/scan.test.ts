import { describe, it, expect } from 'vitest';
import { join } from 'node:path';
import { runScan } from '../src/tools/scan.js';

const fixturesDir = join(import.meta.dirname, 'fixtures');

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
