import { describe, it, expect } from 'vitest';
import { ToolExecutor } from '../src/executor.js';
import type { ToolDefinition } from '../src/schema.js';
import { z } from 'zod';

describe('ToolExecutor', () => {
  const executor = new ToolExecutor();

  function makeTool(overrides?: Partial<ToolDefinition>): ToolDefinition {
    return {
      name: 'test.tool', description: 'test', product: 'test',
      inputSchema: z.object({ value: z.string() }),
      requiresApproval: false,
      execute: async (input) => ({ success: true, output: `Got: ${(input as { value: string }).value}` }),
      ...overrides,
    };
  }

  it('validates input and executes tool', async () => {
    const result = await executor.execute(makeTool(), { value: 'hello' });
    expect(result.success).toBe(true);
    expect(result.output).toBe('Got: hello');
  });

  it('returns error for invalid input', async () => {
    const result = await executor.execute(makeTool(), { value: 123 });
    expect(result.success).toBe(false);
    expect(result.output).toContain('Invalid input');
  });

  it('catches execution errors', async () => {
    const tool = makeTool({ execute: async () => { throw new Error('boom'); } });
    const result = await executor.execute(tool, { value: 'test' });
    expect(result.success).toBe(false);
    expect(result.output).toContain('boom');
  });
});
