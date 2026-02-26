import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { scanDirectory } from '../src/scanner.js';
import { mkdtemp, writeFile, mkdir, rm } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

describe('scanDirectory', () => {
  let tmpDir: string;

  beforeAll(async () => {
    tmpDir = await mkdtemp(join(tmpdir(), 'soul-test-'));
    await mkdir(join(tmpDir, 'src'));
    await writeFile(join(tmpDir, 'src', 'app.ts'), 'console.log("hello")');
    await writeFile(join(tmpDir, 'src', 'util.ts'), 'export const x = 1');
    await writeFile(join(tmpDir, 'package.json'), '{}');
    await mkdir(join(tmpDir, 'node_modules'));
    await writeFile(join(tmpDir, 'node_modules', 'dep.js'), 'module.exports = 1');
    await writeFile(join(tmpDir, '.gitignore'), '*.log\n');
    await writeFile(join(tmpDir, 'debug.log'), 'log data');
  });

  afterAll(async () => { await rm(tmpDir, { recursive: true }); });

  it('scans files respecting .gitignore', async () => {
    const files = await scanDirectory(tmpDir);
    const paths = files.map((f) => f.relativePath);
    expect(paths).toContain('src/app.ts');
    expect(paths).toContain('src/util.ts');
    expect(paths).toContain('package.json');
    expect(paths).not.toContain('debug.log');
    expect(paths.some((p) => p.includes('node_modules'))).toBe(false);
  });

  it('respects maxFiles limit', async () => {
    const files = await scanDirectory(tmpDir, { maxFiles: 2 });
    expect(files.length).toBeLessThanOrEqual(2);
  });
});
