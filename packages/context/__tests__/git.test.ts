import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { getGitInfo } from '../src/git.js';
import { mkdtemp, rm } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { execFileSync } from 'node:child_process';

describe('getGitInfo', () => {
  let tmpDir: string;

  beforeAll(async () => {
    tmpDir = await mkdtemp(join(tmpdir(), 'soul-git-test-'));
    execFileSync('git', ['init'], { cwd: tmpDir });
    execFileSync('git', ['commit', '--allow-empty', '-m', 'init'], { cwd: tmpDir });
  });

  afterAll(async () => { await rm(tmpDir, { recursive: true }); });

  it('detects a git repo', async () => {
    const info = await getGitInfo(tmpDir);
    expect(info.isRepo).toBe(true);
    expect(info.recentCommits).toHaveLength(1);
  });

  it('returns isRepo false for non-git directory', async () => {
    const nonGit = await mkdtemp(join(tmpdir(), 'soul-nongit-'));
    const info = await getGitInfo(nonGit);
    expect(info.isRepo).toBe(false);
    await rm(nonGit, { recursive: true });
  });
});
