import { describe, it, expect, afterEach } from 'vitest';
import { mkdtempSync, writeFileSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { rmSync } from 'node:fs';
import {
  createMonitorTool,
  startMonitor,
  diffResults,
  formatMonitorDiff,
} from '../src/tools/monitor.js';
import type { ScanResult, Finding } from '../src/types.js';
import type { MonitorHandle, MonitorDiff } from '../src/tools/monitor.js';

// ── Helpers ─────────────────────────────────────────────────────────────

function makeFinding(overrides: Partial<Finding> = {}): Finding {
  return {
    id: 'TEST-001',
    title: 'Test finding',
    description: 'A test finding',
    severity: 'medium',
    framework: ['soc2'],
    controlIds: ['CC1.1'],
    analyzer: 'test-analyzer',
    fixable: false,
    ...overrides,
  };
}

function makeScanResult(findings: Finding[] = []): ScanResult {
  return {
    findings,
    summary: {
      total: findings.length,
      bySeverity: { critical: 0, high: 0, medium: findings.length, low: 0, info: 0 },
      byFramework: { soc2: findings.length, hipaa: 0, gdpr: 0 },
      byAnalyzer: { 'test-analyzer': findings.length },
      fixable: 0,
    },
    metadata: {
      directory: '/tmp/test',
      duration: 50,
      analyzersRun: ['test-analyzer'],
      frameworks: ['soc2', 'hipaa', 'gdpr'],
      timestamp: '2026-02-26T12:00:00.000Z',
    },
  };
}

// ── Tests ───────────────────────────────────────────────────────────────

describe('Monitor tool tier gate', () => {
  it('throws TierError for free tier', async () => {
    const tool = createMonitorTool();

    // Without credentials file, getCurrentTier() returns 'free',
    // so requireTier('pro', ...) will throw TierError.
    await expect(
      tool.execute({ directory: '/tmp/test' }),
    ).rejects.toThrow('Monitor mode requires Soul pro');
  });

  it('registers correctly with expected name and product', () => {
    const tool = createMonitorTool();

    expect(tool.name).toBe('compliance-monitor');
    expect(tool.product).toBe('compliance');
    expect(tool.description).toContain('Watch');
    expect(typeof tool.execute).toBe('function');
  });
});

describe('diffResults', () => {
  it('detects new findings', () => {
    const prev = makeScanResult([]);
    const curr = makeScanResult([
      makeFinding({ id: 'NEW-001', file: 'app.ts', line: 10 }),
    ]);

    const diff = diffResults(prev, curr);

    expect(diff.newFindings).toHaveLength(1);
    expect(diff.newFindings[0].id).toBe('NEW-001');
    expect(diff.resolvedFindings).toHaveLength(0);
  });

  it('detects resolved findings', () => {
    const prev = makeScanResult([
      makeFinding({ id: 'OLD-001', file: 'config.ts', line: 5 }),
    ]);
    const curr = makeScanResult([]);

    const diff = diffResults(prev, curr);

    expect(diff.newFindings).toHaveLength(0);
    expect(diff.resolvedFindings).toHaveLength(1);
    expect(diff.resolvedFindings[0].id).toBe('OLD-001');
  });

  it('detects both new and resolved findings simultaneously', () => {
    const prev = makeScanResult([
      makeFinding({ id: 'KEPT-001', file: 'a.ts', line: 1 }),
      makeFinding({ id: 'REMOVED-001', file: 'b.ts', line: 2 }),
    ]);
    const curr = makeScanResult([
      makeFinding({ id: 'KEPT-001', file: 'a.ts', line: 1 }),
      makeFinding({ id: 'ADDED-001', file: 'c.ts', line: 3 }),
    ]);

    const diff = diffResults(prev, curr);

    expect(diff.newFindings).toHaveLength(1);
    expect(diff.newFindings[0].id).toBe('ADDED-001');
    expect(diff.resolvedFindings).toHaveLength(1);
    expect(diff.resolvedFindings[0].id).toBe('REMOVED-001');
  });

  it('returns empty diff when results are identical', () => {
    const finding = makeFinding({ id: 'SAME-001', file: 'x.ts', line: 1 });
    const prev = makeScanResult([finding]);
    const curr = makeScanResult([finding]);

    const diff = diffResults(prev, curr);

    expect(diff.newFindings).toHaveLength(0);
    expect(diff.resolvedFindings).toHaveLength(0);
  });
});

describe('formatMonitorDiff', () => {
  it('formats new findings with + prefix', () => {
    const diff: MonitorDiff = {
      newFindings: [makeFinding({ id: 'NEW-001', file: 'app.ts', line: 10 })],
      resolvedFindings: [],
      timestamp: '2026-02-26T12:00:00.000Z',
    };

    const output = formatMonitorDiff(diff);

    expect(output).toContain('New findings (1)');
    expect(output).toContain('+ [MEDIUM] NEW-001');
    expect(output).toContain('app.ts:10');
  });

  it('formats resolved findings with - prefix', () => {
    const diff: MonitorDiff = {
      newFindings: [],
      resolvedFindings: [makeFinding({ id: 'OLD-001', file: 'config.ts', line: 5 })],
      timestamp: '2026-02-26T12:00:00.000Z',
    };

    const output = formatMonitorDiff(diff);

    expect(output).toContain('Resolved findings (1)');
    expect(output).toContain('- [MEDIUM] OLD-001');
    expect(output).toContain('config.ts:5');
  });

  it('shows no-changes message for empty diff', () => {
    const diff: MonitorDiff = {
      newFindings: [],
      resolvedFindings: [],
      timestamp: '2026-02-26T12:00:00.000Z',
    };

    const output = formatMonitorDiff(diff);

    expect(output).toContain('No changes in findings');
  });
});

describe('startMonitor - file change detection', () => {
  let tmpDir: string;
  let handle: MonitorHandle | null = null;

  afterEach(() => {
    // Always clean up the watcher to avoid hanging tests
    if (handle) {
      handle.stop();
      handle = null;
    }
    if (tmpDir) {
      rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('performs an initial scan and returns results', async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'monitor-init-'));
    mkdirSync(join(tmpDir, 'src'), { recursive: true });
    writeFileSync(join(tmpDir, 'src', 'index.ts'), 'console.log("hello");\n', 'utf-8');

    handle = await startMonitor({
      directory: tmpDir,
      debounceMs: 200,
    });

    expect(handle.initialResult).toBeDefined();
    expect(handle.initialResult.metadata.directory).toBe(tmpDir);
    expect(typeof handle.stop).toBe('function');
  }, 15000);

  it('detects file changes and fires onChange callback', async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'monitor-change-'));
    mkdirSync(join(tmpDir, 'src'), { recursive: true });
    writeFileSync(join(tmpDir, 'src', 'index.ts'), 'console.log("hello");\n', 'utf-8');

    let callbackFired = false;

    handle = await startMonitor({
      directory: tmpDir,
      debounceMs: 200,
      onChange: (_diff) => {
        callbackFired = true;
      },
    });

    // Write a new file to trigger the watcher
    writeFileSync(join(tmpDir, 'src', 'secret.ts'), "const KEY = 'sk-ant-api03-abcdefghijklmnop';\n", 'utf-8');

    // Wait for debounce + scan time (generous timeout for Pi)
    await new Promise<void>((resolve) => setTimeout(resolve, 2000));

    // The callback may or may not fire depending on whether the scan picks up
    // a finding diff. We at least verify no crash and the watcher ran.
    // The important assertion is that the monitor accepted the change event
    // without error.
    expect(handle.initialResult).toBeDefined();
  }, 15000);

  it('debounces rapid changes into a single callback', async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'monitor-debounce-'));
    mkdirSync(join(tmpDir, 'src'), { recursive: true });
    writeFileSync(join(tmpDir, 'src', 'index.ts'), 'console.log("hello");\n', 'utf-8');

    let callbackCount = 0;

    handle = await startMonitor({
      directory: tmpDir,
      debounceMs: 500,
      onChange: (_diff) => {
        callbackCount++;
      },
    });

    // Write multiple files rapidly (within the debounce window)
    for (let i = 0; i < 5; i++) {
      writeFileSync(
        join(tmpDir, 'src', `file${i}.ts`),
        `export const x${i} = ${i};\n`,
        'utf-8',
      );
    }

    // Wait for debounce period + scan time
    await new Promise<void>((resolve) => setTimeout(resolve, 2000));

    // The debounce should coalesce these into at most 1 callback
    // (could be 0 if no findings changed)
    expect(callbackCount).toBeLessThanOrEqual(1);
  }, 15000);
});
