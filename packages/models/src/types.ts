export interface ModelRequest {
  messages: Message[];
  model?: string;
  maxTokens?: number;
  system?: string;
  tools?: ToolSpec[];
}

export interface Message {
  role: 'user' | 'assistant';
  content: string;
}

export interface ToolSpec {
  name: string;
  description: string;
  input_schema: Record<string, unknown>;
}

export interface ModelResponse {
  content: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  stopReason: string;
  toolCalls?: ToolCall[];
}

export interface ToolCall {
  name: string;
  input: Record<string, unknown>;
}

export interface RequestLog {
  provider: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
  costUsd: number;
  latencyMs: number;
  taskType: string;
  product: string;
  timestamp: number;
}

export type ProviderName = 'claude-api' | 'claude-cli' | 'local' | 'openrouter';

export interface Provider {
  name: ProviderName;
  probe(): Promise<boolean>;
  complete(request: ModelRequest): Promise<ModelResponse>;
}
