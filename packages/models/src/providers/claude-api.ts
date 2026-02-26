import Anthropic from '@anthropic-ai/sdk';
import { readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import type { Provider, ModelRequest, ModelResponse } from '../types.js';

interface ClaudeCredentials {
  accessToken: string;
  refreshToken: string;
  expiresAt: number;
}

function readOAuthToken(): ClaudeCredentials | null {
  try {
    const raw = readFileSync(join(homedir(), '.claude', '.credentials.json'), 'utf-8');
    const parsed = JSON.parse(raw);
    const oauth = parsed?.claudeAiOauth;
    if (!oauth?.accessToken) return null;
    return { accessToken: oauth.accessToken, refreshToken: oauth.refreshToken, expiresAt: oauth.expiresAt };
  } catch { return null; }
}

export class ClaudeApiProvider implements Provider {
  name = 'claude-api' as const;
  private client: Anthropic | null = null;

  async probe(): Promise<boolean> {
    const creds = readOAuthToken();
    if (!creds) return false;
    if (Date.now() > creds.expiresAt) return false;
    try {
      const client = new Anthropic({ apiKey: creds.accessToken });
      // Lightweight probe: count tokens on a tiny input instead of a full chat completion.
      // This validates the token works without generating output tokens (much cheaper).
      await client.messages.countTokens({
        model: 'claude-haiku-4-5-20251001',
        messages: [{ role: 'user', content: 'ok' }],
      });
      this.client = client;
      return true;
    } catch { this.client = null; return false; }
  }

  async complete(request: ModelRequest): Promise<ModelResponse> {
    const creds = readOAuthToken();
    if (!creds) throw new Error('No OAuth credentials found');
    if (Date.now() > creds.expiresAt) {
      this.client = null;
      throw new Error('OAuth token expired');
    }
    this.client = new Anthropic({ apiKey: creds.accessToken });
    const response = await this.client.messages.create({
      model: request.model ?? 'claude-sonnet-4-6',
      max_tokens: request.maxTokens ?? 4096,
      system: request.system,
      messages: request.messages.map((m) => ({ role: m.role, content: m.content })),
    });
    const textContent = response.content.filter((c) => c.type === 'text').map((c) => c.text).join('');
    const toolCalls = response.content.filter((c) => c.type === 'tool_use').map((c) => ({ name: c.name, input: c.input as Record<string, unknown> }));
    return {
      content: textContent,
      model: response.model,
      inputTokens: response.usage.input_tokens,
      outputTokens: response.usage.output_tokens,
      stopReason: response.stop_reason ?? 'end_turn',
      toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
    };
  }
}
