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

  /** Create a router with explicit provider list. First available (probed) provider becomes active. */
  static async withProviders(providers: Provider[]): Promise<ModelRouter> {
    const router = new ModelRouter(providers);
    for (const provider of providers) {
      const ok = await provider.probe();
      if (ok) { router.activeProvider = provider; break; }
    }
    if (!router.activeProvider) {
      throw new Error('No model provider available');
    }
    return router;
  }

  getProviderName(): ProviderName {
    return this.activeProvider?.name ?? 'claude-api';
  }

  async complete(request: ModelRequest, meta?: { taskType?: string; product?: string }): Promise<ModelResponse> {
    if (!this.activeProvider) throw new Error('No model provider available');

    // Build ordered list: active provider first, then remaining providers in order
    const currentIdx = this.providers.indexOf(this.activeProvider);
    const ordered = [
      this.activeProvider,
      ...this.providers.filter((_, i) => i !== currentIdx),
    ];

    const start = Date.now();
    let lastError: Error | undefined;

    for (const provider of ordered) {
      // Skip probe for the active provider (already probed at startup)
      if (provider !== this.activeProvider) {
        const ok = await provider.probe();
        if (!ok) continue;
      }
      try {
        const response = await provider.complete(request);
        this.activeProvider = provider;
        this.tracker.log({
          provider: provider.name, model: response.model,
          inputTokens: response.inputTokens, outputTokens: response.outputTokens,
          costUsd: 0, latencyMs: Date.now() - start,
          taskType: meta?.taskType ?? 'default', product: meta?.product ?? 'core',
        });
        return response;
      } catch (err) {
        lastError = err instanceof Error ? err : new Error(String(err));
      }
    }

    throw lastError ?? new Error('No model provider available');
  }
}
