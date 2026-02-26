import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { which } from '../util.js';
import type { Provider, ModelRequest, ModelResponse } from '../types.js';

const execFileAsync = promisify(execFile);

export class ClaudeCliProvider implements Provider {
  name = 'claude-cli' as const;
  private binaryPath: string | null = null;

  async probe(): Promise<boolean> {
    this.binaryPath = await which('claude');
    return this.binaryPath !== null;
  }

  async complete(request: ModelRequest): Promise<ModelResponse> {
    if (!this.binaryPath) throw new Error('Claude CLI binary not found');
    const lastMessage = request.messages[request.messages.length - 1];
    if (!lastMessage) throw new Error('No messages provided');
    const args = ['--output-format', 'json', '--max-turns', '1', '-p', lastMessage.content];
    if (request.system) args.push('--system', request.system);
    try {
      const { stdout } = await execFileAsync(this.binaryPath, args, { timeout: 300_000 });
      try {
        const parsed = JSON.parse(stdout);
        return { content: parsed.result ?? stdout, model: 'claude-cli', inputTokens: parsed.usage?.input_tokens ?? 0, outputTokens: parsed.usage?.output_tokens ?? 0, stopReason: 'end_turn' };
      } catch {
        return { content: stdout.trim(), model: 'claude-cli', inputTokens: 0, outputTokens: 0, stopReason: 'end_turn' };
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      throw new Error(`Claude CLI failed: ${message}`);
    }
  }
}
