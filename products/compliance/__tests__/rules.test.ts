import { describe, it, expect } from 'vitest';
import { loadRules } from '../src/rules/index.js';

describe('Rule loader', () => {
  it('loads all rules from YAML files', () => {
    const rules = loadRules();
    expect(rules.length).toBeGreaterThanOrEqual(60);
  });

  it('every rule has required fields', () => {
    const rules = loadRules();
    for (const rule of rules) {
      expect(rule.id).toBeTruthy();
      expect(rule.title).toBeTruthy();
      expect(rule.severity).toMatch(/^(critical|high|medium|low|info)$/);
      expect(rule.analyzer).toBeTruthy();
      expect(rule.pattern).toBeTruthy();
      expect(rule.controls.length).toBeGreaterThan(0);
      expect(rule.framework.length).toBeGreaterThan(0);
      expect(rule.description).toBeTruthy();
      expect(typeof rule.fixable).toBe('boolean');
    }
  });

  it('has no duplicate rule IDs within same framework', () => {
    const rules = loadRules();
    const seen = new Map<string, Set<string>>();
    for (const rule of rules) {
      for (const fw of rule.framework) {
        const set = seen.get(fw) ?? new Set();
        expect(set.has(rule.id), `Duplicate ${rule.id} in ${fw}`).toBe(false);
        set.add(rule.id);
        seen.set(fw, set);
      }
    }
  });

  it('filters rules by framework', () => {
    const soc2 = loadRules({ frameworks: ['soc2'] });
    const hipaa = loadRules({ frameworks: ['hipaa'] });
    expect(soc2.every((r) => r.framework.includes('soc2'))).toBe(true);
    expect(hipaa.every((r) => r.framework.includes('hipaa'))).toBe(true);
  });
});
