import { describe, it, expect } from 'vitest';
import { ModelRouter } from '../src/router.js';
import type { Provider, ModelRequest, ModelResponse } from '../src/types.js';

function mockProvider(name: string, probeResult: boolean): Provider {
  return {
    name: name as any,
    probe: async () => probeResult,
    complete: async (req: ModelRequest): Promise<ModelResponse> => ({
      content: `response from ${name}`, model: 'test-model',
      inputTokens: 10, outputTokens: 5, stopReason: 'end_turn',
    }),
  };
}

describe('ModelRouter', () => {
  it('uses first available provider', async () => {
    const provider = mockProvider('claude-api', true);
    const router = ModelRouter.withProvider(provider);
    expect(router.getProviderName()).toBe('claude-api');
    const response = await router.complete({ messages: [{ role: 'user', content: 'hello' }] });
    expect(response.content).toBe('response from claude-api');
  });

  it('tracks cost after completion', async () => {
    const provider = mockProvider('claude-api', true);
    const router = ModelRouter.withProvider(provider);
    await router.complete({ messages: [{ role: 'user', content: 'hello' }] });
    const tokens = router.tracker.getSessionTokens();
    expect(tokens.input).toBe(10);
    expect(tokens.output).toBe(5);
    expect(router.tracker.getSessionCost()).toBe(0);
  });
});
