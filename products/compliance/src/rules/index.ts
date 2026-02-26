import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import yaml from 'js-yaml';
import type { RuleDefinition, Framework } from '../types.js';

const __dirname = dirname(fileURLToPath(import.meta.url));

export function loadRules(options?: { frameworks?: Framework[] }): RuleDefinition[] {
  const files = ['soc2.yaml', 'hipaa.yaml', 'gdpr.yaml'];
  const allRules: RuleDefinition[] = [];

  for (const file of files) {
    const raw = readFileSync(join(__dirname, file), 'utf-8');
    const parsed = yaml.load(raw) as RuleDefinition[];
    allRules.push(...parsed);
  }

  if (options?.frameworks?.length) {
    return allRules.filter((r) => r.framework.some((f) => options.frameworks!.includes(f)));
  }
  return allRules;
}
