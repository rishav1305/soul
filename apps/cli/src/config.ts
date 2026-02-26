import { join } from 'node:path';
import { homedir } from 'node:os';
import { ensureSoulDir } from '@soul/core';

export const VERSION = '0.1.0';
export const SOUL_DIR = join(homedir(), '.soul');

export function initConfig(): void {
  ensureSoulDir();
}
