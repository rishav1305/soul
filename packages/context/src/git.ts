import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { access } from 'node:fs/promises';
import { join } from 'node:path';

const execFileAsync = promisify(execFile);

export interface GitInfo {
  isRepo: boolean;
  branch?: string;
  hasRemote?: boolean;
  recentCommits?: string[];
}

export async function getGitInfo(root: string): Promise<GitInfo> {
  try {
    await access(join(root, '.git'));
  } catch {
    return { isRepo: false };
  }

  try {
    const { stdout: branch } = await execFileAsync('git', ['branch', '--show-current'], { cwd: root });
    const { stdout: remotesRaw } = await execFileAsync('git', ['remote'], { cwd: root });
    const { stdout: logRaw } = await execFileAsync('git', ['log', '--oneline', '-10'], { cwd: root });

    return {
      isRepo: true,
      branch: branch.trim(),
      hasRemote: remotesRaw.trim().length > 0,
      recentCommits: logRaw.trim().split('\n').filter(Boolean),
    };
  } catch {
    return { isRepo: true };
  }
}
