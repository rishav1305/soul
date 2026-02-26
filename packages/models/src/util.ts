import { access } from 'node:fs/promises';
import { join } from 'node:path';

export async function which(binary: string): Promise<string | null> {
  const paths = (process.env.PATH ?? '').split(':');
  for (const dir of paths) {
    const full = join(dir, binary);
    try { await access(full); return full; } catch { continue; }
  }
  return null;
}
