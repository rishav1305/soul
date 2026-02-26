import { describe, it, expect } from 'vitest';
import { meetsRequirement, TierError } from '../src/tiers.js';

describe('Tier system', () => {
  it('free meets free', () => { expect(meetsRequirement('free', 'free')).toBe(true); });
  it('free does not meet pro', () => { expect(meetsRequirement('free', 'pro')).toBe(false); });
  it('pro meets pro and free', () => {
    expect(meetsRequirement('pro', 'pro')).toBe(true);
    expect(meetsRequirement('pro', 'free')).toBe(true);
  });
  it('enterprise meets everything', () => {
    expect(meetsRequirement('enterprise', 'enterprise')).toBe(true);
    expect(meetsRequirement('enterprise', 'pro')).toBe(true);
    expect(meetsRequirement('enterprise', 'free')).toBe(true);
  });
  it('TierError contains feature and tier info', () => {
    const err = new TierError('auto-fix', 'pro');
    expect(err.feature).toBe('auto-fix');
    expect(err.requiredTier).toBe('pro');
    expect(err.message).toContain('pro');
  });
});
