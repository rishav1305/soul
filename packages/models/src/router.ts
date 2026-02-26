import type { Provider, ModelRequest, ModelResponse, ProviderName } from './types.js';
import { CostTracker } from './cost-tracker.js';
import { ClaudeApiProvider } from './providers/claude-api.js';
import { ClaudeCliProvider } from './providers/claude-cli.js';

export class ModelRouter {
  private activeProvider: Provider | null = null;
  readonly tracker = new CostTracker();

  private constructor(private providers: Provider[]) {}

  static async autoDetect(): Promise<ModelRouter> {
    const providers: Provider[] = [new ClaudeApiProvider(), new ClaudeCliProvider()];
    const router = new ModelRouter(providers);
    for (const provider of providers) {
      const ok = await provider.probe();
      if (ok) { router.activeProvider = provider; break; }
    }
    if (!router.activeProvider) {
      throw new Error('No model provider available. Ensure Claude Code is installed and you are logged in (claude auth login).');
    }
    return router;
  }

  static withProvider(provider: Provider): ModelRouter {
    const router = new ModelRouter([provider]);
    router.activeProvider = provider;
    return router;
  }

  getProviderName(): ProviderName {
    return this.activeProvider?.name ?? 'claude-api';
  }

  async complete(request: ModelRequest, meta?: { taskType?: string; product?: string }): Promise<ModelResponse> {
    if (!this.activeProvider) throw new Error('No model provider available');
    const start = Date.now();
    try {
      const response = await this.activeProvider.complete(request);
      this.tracker.log({
        provider: this.activeProvider.name, model: response.model,
        inputTokens: response.inputTokens, outputTokens: response.outputTokens,
        costUsd: 0, latencyMs: Date.now() - start,
        taskType: meta?.taskType ?? 'default', product: meta?.product ?? 'core',
      });
      return response;
    } catch (err) {
      const currentIdx = this.providers.indexOf(this.activeProvider);
      for (let i = currentIdx + 1; i < this.providers.length; i++) {
        const fallback = this.providers[i];
        const ok = await fallback.probe();
        if (ok) { this.activeProvider = fallback; return this.complete(request, meta); }
      }
      throw err;
    }
  }
}
