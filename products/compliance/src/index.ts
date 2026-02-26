import type { PluginRegistry } from '@soul/plugins';
import { createScanTool } from './tools/scan.js';
import { createBadgeTool } from './tools/badge.js';
import { createReportTool } from './tools/report.js';
import { createFixTool } from './tools/fix.js';
import { createMonitorTool } from './tools/monitor.js';

export function register(registry: PluginRegistry): void {
  registry.addTool(createScanTool());
  registry.addTool(createBadgeTool());
  registry.addTool(createReportTool());
  registry.addTool(createFixTool());
  registry.addTool(createMonitorTool());

  registry.addCommand({ name: 'compliance scan', description: 'Run compliance scan', product: 'compliance' });
  registry.addCommand({ name: 'compliance badge', description: 'Generate compliance badge', product: 'compliance' });
  registry.addCommand({ name: 'compliance report', description: 'Generate compliance report', product: 'compliance' });
  registry.addCommand({ name: 'compliance fix', description: 'Auto-remediate compliance issues', product: 'compliance' });
  registry.addCommand({ name: 'compliance monitor', description: 'Watch mode for compliance changes', product: 'compliance' });
}

export { runScan, createScanTool } from './tools/scan.js';
export { createBadgeTool } from './tools/badge.js';
export { createReportTool } from './tools/report.js';
export { createFixTool, runFix } from './tools/fix.js';
export { createMonitorTool, startMonitor, diffResults, formatMonitorDiff } from './tools/monitor.js';
export type { MonitorDiff, MonitorOptions, MonitorHandle } from './tools/monitor.js';
export { formatTerminal } from './reporters/terminal.js';
export { formatJSON } from './reporters/json.js';
export { generateBadge, calculateScore, scoreColor } from './reporters/badge.js';
export { generateHtml } from './reporters/html.js';
export { generatePdf } from './reporters/pdf.js';
export type { Finding, ScanResult, Severity, Framework, RuleDefinition, ScanOptions } from './types.js';
