import { describe, it, expect, vi } from 'vitest';
import { ModelRouter } from '../src/router.js';
import type { Provider, ModelRequest, ModelResponse } from '../src/types.js';

function mockProvider(
  name: string,
  opts: {
    probeResult?: boolean;
    completeResult?: ModelResponse;
    completeError?: Error;
  } = {},
): Provider {
  const {
    probeResult = true,
    completeResult = {
      content: `response from ${name}`, model: 'test-model',
      inputTokens: 10, outputTokens: 5, stopReason: 'end_turn',
    },
    completeError,
  } = opts;

  return {
    name: name as any,
    probe: vi.fn(async () => probeResult),
    complete: vi.fn(async (_req: ModelRequest): Promise<ModelResponse> => {
      if (completeError) throw completeError;
      return completeResult;
    }),
  };
}

const testRequest: ModelRequest = { messages: [{ role: 'user', content: 'hello' }] };

describe('ModelRouter', () => {
  it('uses first available provider', async () => {
    const provider = mockProvider('claude-api');
    const router = ModelRouter.withProvider(provider);
    expect(router.getProviderName()).toBe('claude-api');
    const response = await router.complete(testRequest);
    expect(response.content).toBe('response from claude-api');
  });

  it('tracks cost after completion', async () => {
    const provider = mockProvider('claude-api');
    const router = ModelRouter.withProvider(provider);
    await router.complete(testRequest);
    const tokens = router.tracker.getSessionTokens();
    expect(tokens.input).toBe(10);
    expect(tokens.output).toBe(5);
    expect(router.tracker.getSessionCost()).toBe(0);
  });

  describe('fallback behavior', () => {
    it('falls back to next provider when primary fails', async () => {
      const primary = mockProvider('claude-api', {
        completeError: new Error('API unavailable'),
      });
      const fallback = mockProvider('claude-cli');

      const router = await ModelRouter.withProviders([primary, fallback]);
      // Primary was selected during withProviders since probe returned true
      expect(router.getProviderName()).toBe('claude-api');

      const response = await router.complete(testRequest);
      // Should have fallen back to claude-cli
      expect(response.content).toBe('response from claude-cli');
      expect(router.getProviderName()).toBe('claude-cli');
    });

    it('throws last error when all providers fail', async () => {
      const primary = mockProvider('claude-api', {
        completeError: new Error('API unavailable'),
      });
      const secondary = mockProvider('claude-cli', {
        completeError: new Error('CLI binary crashed'),
      });

      const router = await ModelRouter.withProviders([primary, secondary]);

      await expect(router.complete(testRequest)).rejects.toThrow('CLI binary crashed');
    });

    it('skips fallback providers that fail probe', async () => {
      const primary = mockProvider('claude-api', {
        completeError: new Error('API unavailable'),
      });
      const badFallback = mockProvider('local', { probeResult: false });
      const goodFallback = mockProvider('claude-cli');

      const router = await ModelRouter.withProviders([primary, badFallback, goodFallback]);

      const response = await router.complete(testRequest);
      expect(response.content).toBe('response from claude-cli');
      // badFallback.probe should have been called during fallback
      expect(badFallback.probe).toHaveBeenCalled();
      // badFallback.complete should NOT have been called (probe failed)
      expect(badFallback.complete).not.toHaveBeenCalled();
    });

    it('withProviders selects first provider that probes successfully', async () => {
      const failing = mockProvider('claude-api', { probeResult: false });
      const working = mockProvider('claude-cli');

      const router = await ModelRouter.withProviders([failing, working]);
      expect(router.getProviderName()).toBe('claude-cli');
    });

    it('withProviders throws when no provider probes successfully', async () => {
      const a = mockProvider('claude-api', { probeResult: false });
      const b = mockProvider('claude-cli', { probeResult: false });

      await expect(ModelRouter.withProviders([a, b])).rejects.toThrow('No model provider available');
    });

    it('logs cost for the fallback provider that succeeds', async () => {
      const primary = mockProvider('claude-api', {
        completeError: new Error('API unavailable'),
      });
      const fallback = mockProvider('claude-cli', {
        completeResult: {
          content: 'fallback response', model: 'cli-model',
          inputTokens: 20, outputTokens: 15, stopReason: 'end_turn',
        },
      });

      const router = await ModelRouter.withProviders([primary, fallback]);
      await router.complete(testRequest);

      const tokens = router.tracker.getSessionTokens();
      expect(tokens.input).toBe(20);
      expect(tokens.output).toBe(15);
    });
  });
});
