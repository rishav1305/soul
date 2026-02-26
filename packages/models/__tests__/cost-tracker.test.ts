import { describe, it, expect } from 'vitest';
import { CostTracker } from '../src/cost-tracker.js';

describe('CostTracker', () => {
  it('tracks session tokens and cost', () => {
    const tracker = new CostTracker();
    tracker.log({ provider: 'claude-api', model: 'sonnet', inputTokens: 100, outputTokens: 50, costUsd: 0, latencyMs: 200, taskType: 'test', product: 'compliance' });
    tracker.log({ provider: 'claude-api', model: 'sonnet', inputTokens: 200, outputTokens: 100, costUsd: 0, latencyMs: 300, taskType: 'test', product: 'compliance' });
    expect(tracker.getSessionTokens()).toEqual({ input: 300, output: 150 });
    expect(tracker.getSessionCost()).toBe(0);
    expect(tracker.getLogs()).toHaveLength(2);
  });
});
