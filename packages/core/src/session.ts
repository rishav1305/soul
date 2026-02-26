import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';

export interface SessionState {
  product?: string;
  conversationHistory: Array<{ role: string; content: string }>;
  startedAt: number;
}

const SOUL_DIR = join(homedir(), '.soul');

export function ensureSoulDir(): string {
  mkdirSync(SOUL_DIR, { recursive: true });
  return SOUL_DIR;
}

export function createSession(product?: string): SessionState {
  return { product, conversationHistory: [], startedAt: Date.now() };
}

export function saveSession(session: SessionState, id: string): void {
  const dir = ensureSoulDir();
  writeFileSync(join(dir, `session-${id}.json`), JSON.stringify(session, null, 2));
}

export function loadSession(id: string): SessionState | null {
  try {
    const raw = readFileSync(join(SOUL_DIR, `session-${id}.json`), 'utf-8');
    return JSON.parse(raw);
  } catch { return null; }
}
