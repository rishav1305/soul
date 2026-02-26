import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';

export type Tier = 'free' | 'pro' | 'enterprise';

export function getCurrentTier(): Tier {
  try {
    const raw = readFileSync(join(homedir(), '.soul', 'credentials.json'), 'utf-8');
    const parsed = JSON.parse(raw);
    return (parsed.tier as Tier) ?? 'free';
  } catch { return 'free'; }
}

export function meetsRequirement(current: Tier, required: Tier): boolean {
  const order: Tier[] = ['free', 'pro', 'enterprise'];
  return order.indexOf(current) >= order.indexOf(required);
}

export function requireTier(required: Tier, feature: string): void {
  const current = getCurrentTier();
  if (!meetsRequirement(current, required)) throw new TierError(feature, required);
}

export class TierError extends Error {
  constructor(public readonly feature: string, public readonly requiredTier: Tier) {
    super(`${feature} requires Soul ${requiredTier}`);
    this.name = 'TierError';
  }
}
