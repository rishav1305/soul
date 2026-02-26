import { readdir, readFile, stat } from 'node:fs/promises';
import { join, relative } from 'node:path';
import ignore from 'ignore';

export interface ScannedFile {
  path: string;
  relativePath: string;
  size: number;
  extension: string;
}

export async function scanDirectory(
  root: string,
  options?: { maxFiles?: number },
): Promise<ScannedFile[]> {
  const maxFiles = options?.maxFiles ?? 10000;
  const ig = ignore();

  try {
    const gitignoreContent = await readFile(join(root, '.gitignore'), 'utf-8');
    ig.add(gitignoreContent);
  } catch { /* no .gitignore */ }

  ig.add(['node_modules', '.git', 'dist', 'coverage', '.turbo']);

  const files: ScannedFile[] = [];

  async function walk(dir: string): Promise<void> {
    if (files.length >= maxFiles) return;
    const entries = await readdir(dir, { withFileTypes: true });
    for (const entry of entries) {
      if (files.length >= maxFiles) return;
      const fullPath = join(dir, entry.name);
      const relPath = relative(root, fullPath);
      if (ig.ignores(relPath)) continue;
      if (entry.isDirectory()) {
        await walk(fullPath);
      } else if (entry.isFile()) {
        const stats = await stat(fullPath);
        const ext = entry.name.includes('.') ? entry.name.split('.').pop() ?? '' : '';
        files.push({ path: fullPath, relativePath: relPath, size: stats.size, extension: ext });
      }
    }
  }

  await walk(root);
  return files;
}
