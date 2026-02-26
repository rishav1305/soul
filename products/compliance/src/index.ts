import type { PluginRegistry } from '@soul/plugins';
import { createScanTool } from './tools/scan.js';
import { createBadgeTool } from './tools/badge.js';

export function register(registry: PluginRegistry): void {
  registry.addTool(createScanTool());
  registry.addTool(createBadgeTool());
}

export { runScan, createScanTool } from './tools/scan.js';
export { createBadgeTool } from './tools/badge.js';
export { formatTerminal } from './reporters/terminal.js';
export { formatJSON } from './reporters/json.js';
export { generateBadge, calculateScore, scoreColor } from './reporters/badge.js';
export type { Finding, ScanResult, Severity, Framework, RuleDefinition, ScanOptions } from './types.js';
