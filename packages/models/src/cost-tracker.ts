import type { RequestLog } from './types.js';

export class CostTracker {
  private logs: RequestLog[] = [];

  log(entry: Omit<RequestLog, 'timestamp'>): void {
    this.logs.push({ ...entry, timestamp: Date.now() });
  }

  getSessionCost(): number {
    return this.logs.reduce((sum, l) => sum + l.costUsd, 0);
  }

  getSessionTokens(): { input: number; output: number } {
    return this.logs.reduce((acc, l) => ({ input: acc.input + l.inputTokens, output: acc.output + l.outputTokens }), { input: 0, output: 0 });
  }

  getLogs(): RequestLog[] {
    return [...this.logs];
  }
}
