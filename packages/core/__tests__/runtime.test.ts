import { describe, it, expect } from 'vitest';
import { SoulRuntime } from '../src/runtime.js';
import { PluginRegistry } from '@soul/plugins';
import { ModelRouter } from '@soul/models';
import type { Provider, ModelRequest, ModelResponse } from '@soul/models';

function mockProvider(): Provider {
  return {
    name: 'claude-api',
    probe: async () => true,
    complete: async (_req: ModelRequest): Promise<ModelResponse> => ({
      content: 'Hello from Soul',
      model: 'test',
      inputTokens: 10,
      outputTokens: 5,
      stopReason: 'end_turn',
    }),
  };
}

describe('SoulRuntime', () => {
  it('initializes with router and registry', () => {
    const runtime = new SoulRuntime({
      router: ModelRouter.withProvider(mockProvider()),
      registry: new PluginRegistry(),
    });
    expect(runtime.getProviderName()).toBe('claude-api');
    expect(runtime.getSession().conversationHistory).toHaveLength(0);
  });

  it('sends a chat message and tracks history', async () => {
    const runtime = new SoulRuntime({
      router: ModelRouter.withProvider(mockProvider()),
      registry: new PluginRegistry(),
      product: 'compliance',
    });
    const response = await runtime.chat('scan my code');
    expect(response.content).toBe('Hello from Soul');
    expect(runtime.getSession().conversationHistory).toHaveLength(2);
    expect(runtime.getTokens().input).toBe(10);
  });

  it('executes a registered tool', async () => {
    const registry = new PluginRegistry();
    const { z } = await import('zod');
    registry.addTool({
      name: 'test.echo', description: 'echo', product: 'test',
      inputSchema: z.object({ msg: z.string() }),
      requiresApproval: false,
      execute: async (input) => ({ success: true, output: `Echo: ${(input as { msg: string }).msg}` }),
    });
    const runtime = new SoulRuntime({
      router: ModelRouter.withProvider(mockProvider()),
      registry,
    });
    const result = await runtime.executeTool('test.echo', { msg: 'hi' });
    expect(result.success).toBe(true);
    expect(result.output).toBe('Echo: hi');
  });

  it('returns error for unknown tool', async () => {
    const runtime = new SoulRuntime({
      router: ModelRouter.withProvider(mockProvider()),
      registry: new PluginRegistry(),
    });
    const result = await runtime.executeTool('nonexistent', {});
    expect(result.success).toBe(false);
    expect(result.output).toContain('not found');
  });
});
