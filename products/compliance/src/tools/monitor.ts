import { watch, type FSWatcher } from 'node:fs';
import { z } from 'zod';
import { requireTier } from '@soul/core';
import type { ToolDefinition, ToolResult } from '@soul/plugins';
import { runScan } from './scan.js';
import type { Finding, ScanResult, ScanOptions } from '../types.js';

// ── Types ───────────────────────────────────────────────────────────────

/**
 * Diff between two consecutive scan results.
 */
export interface MonitorDiff {
  newFindings: Finding[];
  resolvedFindings: Finding[];
  timestamp: string;
}

/**
 * Options for starting the monitor.
 */
export interface MonitorOptions {
  directory: string;
  debounceMs?: number;
  scanOptions?: Omit<ScanOptions, 'directory'>;
  onChange?: (diff: MonitorDiff) => void;
}

/**
 * Handle returned by `startMonitor()` to control the watcher lifecycle.
 */
export interface MonitorHandle {
  /** Stop the file watcher and clean up timers. */
  stop(): void;
  /** The initial scan result captured when the monitor started. */
  initialResult: ScanResult;
}

// ── Finding key helpers ─────────────────────────────────────────────────

/**
 * Build a unique key for a finding based on file, line, and id.
 */
function findingKey(f: Finding): string {
  return `${f.file ?? ''}:${f.line ?? 0}:${f.id}`;
}

// ── Diff logic ──────────────────────────────────────────────────────────

/**
 * Compute the diff between a previous and current scan result.
 *
 * - `newFindings`: findings present in `curr` but not in `prev`
 * - `resolvedFindings`: findings present in `prev` but not in `curr`
 */
export function diffResults(prev: ScanResult, curr: ScanResult): MonitorDiff {
  const prevKeys = new Set(prev.findings.map(findingKey));
  const currKeys = new Set(curr.findings.map(findingKey));

  const newFindings = curr.findings.filter((f) => !prevKeys.has(findingKey(f)));
  const resolvedFindings = prev.findings.filter((f) => !currKeys.has(findingKey(f)));

  return {
    newFindings,
    resolvedFindings,
    timestamp: new Date().toISOString(),
  };
}

// ── Monitor core ────────────────────────────────────────────────────────

/**
 * Start watching a directory for file changes. After debounce, re-scans
 * and computes a diff against the previous result, calling `onChange` with
 * any new or resolved findings.
 *
 * @returns A `MonitorHandle` with a `stop()` method and the initial scan result.
 */
export async function startMonitor(options: MonitorOptions): Promise<MonitorHandle> {
  const debounceMs = options.debounceMs ?? 500;

  // Run the initial scan
  const initialResult = await runScan({
    directory: options.directory,
    ...options.scanOptions,
  });

  let previousResult = initialResult;
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  let watcher: FSWatcher | null = null;

  const handleChange = () => {
    // Clear any pending debounce timer
    if (debounceTimer !== null) {
      clearTimeout(debounceTimer);
    }

    debounceTimer = setTimeout(async () => {
      debounceTimer = null;
      try {
        const newResult = await runScan({
          directory: options.directory,
          ...options.scanOptions,
        });

        const diff = diffResults(previousResult, newResult);
        previousResult = newResult;

        // Only fire callback if there are actual changes
        if (diff.newFindings.length > 0 || diff.resolvedFindings.length > 0) {
          options.onChange?.(diff);
        }
      } catch {
        // Scan failure during watch is non-fatal; silently retry on next change
      }
    }, debounceMs);
  };

  // Start the file watcher
  watcher = watch(options.directory, { recursive: true }, (_eventType, _filename) => {
    handleChange();
  });

  const stop = () => {
    if (debounceTimer !== null) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
    if (watcher !== null) {
      watcher.close();
      watcher = null;
    }
  };

  return { stop, initialResult };
}

// ── Formatting ──────────────────────────────────────────────────────────

/**
 * Format a MonitorDiff as human-readable terminal output.
 */
export function formatMonitorDiff(diff: MonitorDiff): string {
  const lines: string[] = [];

  lines.push(`[${diff.timestamp}] Monitor detected changes`);
  lines.push('');

  if (diff.newFindings.length > 0) {
    lines.push(`New findings (${diff.newFindings.length}):`);
    for (const f of diff.newFindings) {
      const location = f.file
        ? `${f.file}${f.line ? ':' + f.line : ''}`
        : 'project-level';
      lines.push(`  + [${f.severity.toUpperCase()}] ${f.id}: ${f.title} (${location})`);
    }
    lines.push('');
  }

  if (diff.resolvedFindings.length > 0) {
    lines.push(`Resolved findings (${diff.resolvedFindings.length}):`);
    for (const f of diff.resolvedFindings) {
      const location = f.file
        ? `${f.file}${f.line ? ':' + f.line : ''}`
        : 'project-level';
      lines.push(`  - [${f.severity.toUpperCase()}] ${f.id}: ${f.title} (${location})`);
    }
    lines.push('');
  }

  if (diff.newFindings.length === 0 && diff.resolvedFindings.length === 0) {
    lines.push('  No changes in findings.');
  }

  return lines.join('\n');
}

// ── Tool definition ─────────────────────────────────────────────────────

const MonitorInputSchema = z.object({
  directory: z.string().describe('Absolute path to the directory to watch'),
  frameworks: z
    .array(z.enum(['soc2', 'hipaa', 'gdpr']))
    .optional()
    .describe('Frameworks to check against (default: all)'),
  severity: z
    .array(z.enum(['critical', 'high', 'medium', 'low', 'info']))
    .optional()
    .describe('Only return findings of these severity levels'),
});

/**
 * Create a ToolDefinition for the file watch monitor.
 *
 * This tool is gated behind the `pro` tier.
 */
export function createMonitorTool(): ToolDefinition {
  return {
    name: 'compliance-monitor',
    description:
      'Watch a project directory for file changes and continuously re-scan for compliance issues (requires Soul Pro)',
    product: 'compliance',
    inputSchema: MonitorInputSchema,
    requiresApproval: false,
    execute: async (input: unknown): Promise<ToolResult> => {
      // Tier gate: free tier cannot use monitor mode
      requireTier('pro', 'Monitor mode');

      const parsed = MonitorInputSchema.parse(input);

      // Run an initial scan and return its results.
      // The actual long-running watcher is started via startMonitor()
      // from the CLI layer, not from the tool execute path.
      const result = await runScan({
        directory: parsed.directory,
        frameworks: parsed.frameworks,
        severity: parsed.severity,
      });

      const lines: string[] = [];
      lines.push('Monitor Mode - Initial Scan');
      lines.push('==========================');
      lines.push(`Directory: ${result.metadata.directory}`);
      lines.push(`Total findings: ${result.summary.total}`);
      lines.push(`  Critical: ${result.summary.bySeverity.critical}`);
      lines.push(`  High:     ${result.summary.bySeverity.high}`);
      lines.push(`  Medium:   ${result.summary.bySeverity.medium}`);
      lines.push(`  Low:      ${result.summary.bySeverity.low}`);
      lines.push(`  Info:     ${result.summary.bySeverity.info}`);
      lines.push('');
      lines.push('Watching for changes...');

      return {
        success: true,
        output: lines.join('\n'),
        structured: result as unknown as Record<string, unknown>,
      };
    },
  };
}
