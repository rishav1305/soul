import type { PluginRegistry } from '@soul/plugins';
import { createScanTool } from './tools/scan.js';
import { createBadgeTool } from './tools/badge.js';
import { createReportTool } from './tools/report.js';
import { createFixTool } from './tools/fix.js';

export function register(registry: PluginRegistry): void {
  registry.addTool(createScanTool());
  registry.addTool(createBadgeTool());
  registry.addTool(createReportTool());
  registry.addTool(createFixTool());
}

export { runScan, createScanTool } from './tools/scan.js';
export { createBadgeTool } from './tools/badge.js';
export { createReportTool } from './tools/report.js';
export { createFixTool, runFix } from './tools/fix.js';
export { formatTerminal } from './reporters/terminal.js';
export { formatJSON } from './reporters/json.js';
export { generateBadge, calculateScore, scoreColor } from './reporters/badge.js';
export { generateHtml } from './reporters/html.js';
export { generatePdf } from './reporters/pdf.js';
export type { Finding, ScanResult, Severity, Framework, RuleDefinition, ScanOptions } from './types.js';
