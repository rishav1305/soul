import { z } from 'zod';

export interface ToolDefinition {
  name: string;
  description: string;
  product: string;
  inputSchema: z.ZodType;
  requiresApproval: boolean;
  execute: (input: unknown) => Promise<ToolResult>;
}

export interface ToolResult {
  success: boolean;
  output: string;
  structured?: Record<string, unknown>;
  artifacts?: Artifact[];
}

export interface Artifact {
  type: 'report' | 'badge' | 'file';
  path: string;
  content?: string;
}

export interface CommandDefinition {
  name: string;
  description: string;
  product: string;
}
