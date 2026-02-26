import type { ToolDefinition, ToolResult } from './schema.js';

export class ToolExecutor {
  async execute(tool: ToolDefinition, rawInput: unknown): Promise<ToolResult> {
    const parsed = tool.inputSchema.safeParse(rawInput);
    if (!parsed.success) {
      return { success: false, output: `Invalid input: ${parsed.error.message}` };
    }
    try {
      return await tool.execute(parsed.data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      return { success: false, output: `Tool execution failed: ${message}` };
    }
  }
}
