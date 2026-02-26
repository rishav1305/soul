import type { ToolDefinition, CommandDefinition } from './schema.js';

export class PluginRegistry {
  private tools = new Map<string, ToolDefinition>();
  private commands: CommandDefinition[] = [];

  addTool(tool: ToolDefinition): void {
    if (this.tools.has(tool.name)) {
      throw new Error(`Tool "${tool.name}" already registered`);
    }
    this.tools.set(tool.name, tool);
  }

  addTools(tools: ToolDefinition[]): void {
    for (const tool of tools) { this.addTool(tool); }
  }

  getTool(name: string): ToolDefinition | undefined {
    return this.tools.get(name);
  }

  getToolsByProduct(product: string): ToolDefinition[] {
    return [...this.tools.values()].filter((t) => t.product === product);
  }

  listToolNames(): string[] {
    return [...this.tools.keys()];
  }

  getAllTools(): ToolDefinition[] {
    return [...this.tools.values()];
  }

  addCommand(command: CommandDefinition): void {
    this.commands.push(command);
  }

  addCommands(commands: CommandDefinition[]): void {
    this.commands.push(...commands);
  }

  getCommands(): CommandDefinition[] {
    return [...this.commands];
  }

  getCommandsByProduct(product: string): CommandDefinition[] {
    return this.commands.filter((c) => c.product === product);
  }
}
