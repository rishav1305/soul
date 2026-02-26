import { readFile } from 'node:fs/promises';
import type { ScannedFile } from './scanner.js';

export interface ContextChunk {
  file: string;
  content: string;
  tokens: number;
}

export async function assembleContext(
  files: ScannedFile[],
  options?: { maxTokens?: number; extensions?: string[] },
): Promise<ContextChunk[]> {
  const maxTokens = options?.maxTokens ?? 50000;
  const extensions = options?.extensions ?? ['ts', 'js', 'tsx', 'jsx', 'py', 'yaml', 'yml', 'json', 'toml', 'md'];

  const relevant = files.filter((f) => extensions.includes(f.extension));
  relevant.sort((a, b) => a.size - b.size);

  const chunks: ContextChunk[] = [];
  let totalTokens = 0;

  for (const file of relevant) {
    if (totalTokens >= maxTokens) break;
    if (file.size > 100_000) continue;
    try {
      const content = await readFile(file.path, 'utf-8');
      const tokens = Math.ceil(content.length / 4);
      if (totalTokens + tokens > maxTokens) continue;
      chunks.push({ file: file.relativePath, content, tokens });
      totalTokens += tokens;
    } catch { continue; }
  }

  return chunks;
}
