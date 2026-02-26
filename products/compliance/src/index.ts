import type { PluginRegistry } from '@soul/plugins';
import { createScanTool } from './tools/scan.js';

export function register(registry: PluginRegistry): void {
  registry.addTool(createScanTool());
}

export { runScan, createScanTool } from './tools/scan.js';
export { formatTerminal } from './reporters/terminal.js';
export { formatJSON } from './reporters/json.js';
export type { Finding, ScanResult, Severity, Framework, RuleDefinition, ScanOptions } from './types.js';
