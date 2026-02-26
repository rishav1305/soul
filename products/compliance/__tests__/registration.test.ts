import { describe, it, expect } from 'vitest';
import { PluginRegistry } from '@soul/plugins';
import { register } from '../src/index.js';

describe('Compliance registration', () => {
  it('registers all 5 tools', () => {
    const registry = new PluginRegistry();
    register(registry);
    const tools = registry.getToolsByProduct('compliance');
    expect(tools.length).toBe(5);
    expect(tools.map((t) => t.name).sort()).toEqual([
      'compliance-badge',
      'compliance-fix',
      'compliance-monitor',
      'compliance-report',
      'compliance-scan',
    ]);
  });

  it('registers CLI commands', () => {
    const registry = new PluginRegistry();
    register(registry);
    const cmds = registry.getCommandsByProduct('compliance');
    expect(cmds.length).toBe(5);
  });
});
