import type { ModelRouter, ModelResponse } from '@soul/models';
import type { PluginRegistry, ToolResult } from '@soul/plugins';
import { ToolExecutor } from '@soul/plugins';
import { createSession, type SessionState } from './session.js';

export interface SoulRuntimeOptions {
  router: ModelRouter;
  registry: PluginRegistry;
  product?: string;
}

export class SoulRuntime {
  private router: ModelRouter;
  private registry: PluginRegistry;
  private executor: ToolExecutor;
  private session: SessionState;

  constructor(options: SoulRuntimeOptions) {
    this.router = options.router;
    this.registry = options.registry;
    this.executor = new ToolExecutor();
    this.session = createSession(options.product);
  }

  getProviderName(): string { return this.router.getProviderName(); }
  getSession(): SessionState { return this.session; }
  getTokens(): { input: number; output: number } { return this.router.tracker.getSessionTokens(); }
  getCost(): number { return this.router.tracker.getSessionCost(); }

  async chat(userMessage: string): Promise<ModelResponse> {
    this.session.conversationHistory.push({ role: 'user', content: userMessage });

    const tools = this.registry.getAllTools();
    const systemPrompt = this.buildSystemPrompt();

    const response = await this.router.complete(
      {
        messages: this.session.conversationHistory.map((m) => ({
          role: m.role as 'user' | 'assistant',
          content: m.content,
        })),
        system: systemPrompt,
        tools: tools.map((t) => ({
          name: t.name,
          description: t.description,
          input_schema: {},
        })),
      },
      { product: this.session.product },
    );

    this.session.conversationHistory.push({ role: 'assistant', content: response.content });
    return response;
  }

  async executeTool(toolName: string, input: unknown): Promise<ToolResult> {
    const tool = this.registry.getTool(toolName);
    if (!tool) return { success: false, output: `Tool "${toolName}" not found` };
    return this.executor.execute(tool, input);
  }

  private buildSystemPrompt(): string {
    const tools = this.registry.getAllTools();
    const toolList = tools.map((t) => `- ${t.name}: ${t.description}`).join('\n');
    return [
      'You are Soul, an AI-native infrastructure agent.',
      this.session.product ? `You are operating in ${this.session.product} mode.` : '',
      tools.length > 0 ? `Available tools:\n${toolList}` : '',
    ].filter(Boolean).join('\n\n');
  }
}
