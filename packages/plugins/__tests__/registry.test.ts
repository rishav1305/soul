import { describe, it, expect } from 'vitest';
import { PluginRegistry } from '../src/registry.js';
import { z } from 'zod';

describe('PluginRegistry', () => {
  it('registers a tool and retrieves it by name', () => {
    const registry = new PluginRegistry();
    registry.addTool({
      name: 'test.hello', description: 'A test tool', product: 'test',
      inputSchema: z.object({ message: z.string() }),
      requiresApproval: false,
      execute: async (input) => ({ success: true, output: `Hello ${(input as { message: string }).message}` }),
    });
    const tool = registry.getTool('test.hello');
    expect(tool).toBeDefined();
    expect(tool!.name).toBe('test.hello');
    expect(tool!.product).toBe('test');
  });

  it('lists all tools for a product', () => {
    const registry = new PluginRegistry();
    const schema = z.object({});
    const execute = async () => ({ success: true, output: '' });
    registry.addTool({ name: 'a.one', description: '', product: 'a', inputSchema: schema, requiresApproval: false, execute });
    registry.addTool({ name: 'a.two', description: '', product: 'a', inputSchema: schema, requiresApproval: false, execute });
    registry.addTool({ name: 'b.one', description: '', product: 'b', inputSchema: schema, requiresApproval: false, execute });
    expect(registry.getToolsByProduct('a')).toHaveLength(2);
    expect(registry.getToolsByProduct('b')).toHaveLength(1);
    expect(registry.getToolsByProduct('c')).toHaveLength(0);
  });

  it('returns all tool names', () => {
    const registry = new PluginRegistry();
    const schema = z.object({});
    const execute = async () => ({ success: true, output: '' });
    registry.addTool({ name: 'x.a', description: '', product: 'x', inputSchema: schema, requiresApproval: false, execute });
    registry.addTool({ name: 'x.b', description: '', product: 'x', inputSchema: schema, requiresApproval: false, execute });
    expect(registry.listToolNames()).toEqual(['x.a', 'x.b']);
  });

  it('throws when registering duplicate tool name', () => {
    const registry = new PluginRegistry();
    const tool = { name: 'dup.tool', description: '', product: 'dup', inputSchema: z.object({}), requiresApproval: false, execute: async () => ({ success: true, output: '' }) };
    registry.addTool(tool);
    expect(() => registry.addTool(tool)).toThrow('already registered');
  });

  it('registers and retrieves commands', () => {
    const registry = new PluginRegistry();
    registry.addCommand({ name: 'soul test run', description: 'Run tests', product: 'test' });
    const cmds = registry.getCommands();
    expect(cmds).toHaveLength(1);
    expect(cmds[0].name).toBe('soul test run');
  });
});
