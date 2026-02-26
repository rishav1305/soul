import { z } from 'zod';
import { scanDirectory } from '@soul/context';
import type { ToolDefinition, ToolResult } from '@soul/plugins';
import { loadRules } from '../rules/index.js';
import { SecretScanner } from '../analyzers/secret-scanner.js';
import { ConfigChecker } from '../analyzers/config-checker.js';
import { AstAnalyzer } from '../analyzers/ast-analyzer.js';
import { GitAnalyzer } from '../analyzers/git-analyzer.js';
import { DepAuditor } from '../analyzers/dep-auditor.js';
import type {
  Analyzer,
  Finding,
  ScanOptions,
  ScanResult,
  Severity,
  Framework,
} from '../types.js';

/**
 * Deduplicate findings based on the composite key of file + line + id.
 * When a finding has no file or no line, it uses empty-string / 0 as the key part.
 */
export function deduplicateFindings(findings: Finding[]): Finding[] {
  const seen = new Set<string>();
  const result: Finding[] = [];
  for (const finding of findings) {
    const key = `${finding.file ?? ''}:${finding.line ?? 0}:${finding.id}`;
    if (!seen.has(key)) {
      seen.add(key);
      result.push(finding);
    }
  }
  return result;
}

/**
 * Build a summary from a list of findings.
 */
export function buildSummary(findings: Finding[]): ScanResult['summary'] {
  const bySeverity: Record<Severity, number> = {
    critical: 0,
    high: 0,
    medium: 0,
    low: 0,
    info: 0,
  };

  const byFramework: Record<Framework, number> = {
    soc2: 0,
    hipaa: 0,
    gdpr: 0,
  };

  const byAnalyzer: Record<string, number> = {};
  let fixable = 0;

  for (const finding of findings) {
    bySeverity[finding.severity]++;

    for (const fw of finding.framework) {
      byFramework[fw]++;
    }

    byAnalyzer[finding.analyzer] = (byAnalyzer[finding.analyzer] ?? 0) + 1;

    if (finding.fixable) {
      fixable++;
    }
  }

  return {
    total: findings.length,
    bySeverity,
    byFramework,
    byAnalyzer,
    fixable,
  };
}

/**
 * Run a compliance scan across a directory.
 *
 * Instantiates all 5 analyzers, runs them in parallel via Promise.allSettled(),
 * deduplicates findings, applies severity/framework filters, and builds a ScanResult.
 */
export async function runScan(options: ScanOptions): Promise<ScanResult> {
  const start = Date.now();

  // Scan the directory for files
  const files = await scanDirectory(options.directory);

  // Load rules, optionally filtering by framework
  const rules = loadRules({ frameworks: options.frameworks });

  // Instantiate all analyzers
  const analyzers: Analyzer[] = [
    new SecretScanner(),
    new ConfigChecker(),
    new AstAnalyzer(),
    new GitAnalyzer(),
    new DepAuditor(),
  ];

  // Filter analyzers if specific ones requested
  const active = options.analyzers
    ? analyzers.filter((a) => options.analyzers!.includes(a.name))
    : analyzers;

  // Run all analyzers in parallel
  const results = await Promise.allSettled(
    active.map((a) => a.analyze(files, rules.filter((r) => r.analyzer === a.name))),
  );

  // Collect findings, skip failed analyzers
  let findings: Finding[] = [];
  const analyzersRun: string[] = [];
  for (let i = 0; i < results.length; i++) {
    const result = results[i];
    if (result.status === 'fulfilled') {
      findings.push(...result.value);
      analyzersRun.push(active[i].name);
    }
  }

  // Deduplicate on file+line+id
  findings = deduplicateFindings(findings);

  // Filter by severity if requested
  if (options.severity?.length) {
    findings = findings.filter((f) => options.severity!.includes(f.severity));
  }

  // Filter by framework if requested (only include findings that match at least one)
  if (options.frameworks?.length) {
    findings = findings.filter((f) =>
      f.framework.some((fw) => options.frameworks!.includes(fw)),
    );
  }

  // Filter by file exclusions if requested
  if (options.exclude?.length) {
    findings = findings.filter(
      (f) => !f.file || !options.exclude!.some((pattern) => f.file!.includes(pattern)),
    );
  }

  const duration = Date.now() - start;

  return {
    findings,
    summary: buildSummary(findings),
    metadata: {
      directory: options.directory,
      duration,
      analyzersRun,
      frameworks: options.frameworks ?? (['soc2', 'hipaa', 'gdpr'] as Framework[]),
      timestamp: new Date().toISOString(),
    },
  };
}

/**
 * Zod input schema for the scan tool.
 */
const ScanInputSchema = z.object({
  directory: z.string().describe('Absolute path to the directory to scan'),
  frameworks: z
    .array(z.enum(['soc2', 'hipaa', 'gdpr']))
    .optional()
    .describe('Frameworks to check against (default: all)'),
  severity: z
    .array(z.enum(['critical', 'high', 'medium', 'low', 'info']))
    .optional()
    .describe('Only return findings of these severity levels'),
  analyzers: z
    .array(z.string())
    .optional()
    .describe('Only run these analyzers (default: all)'),
  exclude: z
    .array(z.string())
    .optional()
    .describe('File path patterns to exclude from results'),
  format: z
    .enum(['terminal', 'json'])
    .optional()
    .describe('Output format (default: terminal)'),
  output: z.string().optional().describe('Output file path'),
});

/**
 * Create a ToolDefinition compatible with the PluginRegistry.
 */
export function createScanTool(): ToolDefinition {
  return {
    name: 'compliance-scan',
    description:
      'Scan a project directory for compliance and security issues across SOC2, HIPAA, and GDPR frameworks',
    product: 'compliance',
    inputSchema: ScanInputSchema,
    requiresApproval: false,
    execute: async (input: unknown): Promise<ToolResult> => {
      const parsed = ScanInputSchema.parse(input);
      const result = await runScan(parsed);

      const output = formatScanOutput(result);

      return {
        success: true,
        output,
        structured: result as unknown as Record<string, unknown>,
      };
    },
  };
}

/**
 * Format scan result as a human-readable string for terminal output.
 */
function formatScanOutput(result: ScanResult): string {
  const lines: string[] = [];

  lines.push(`Compliance Scan Results`);
  lines.push(`======================`);
  lines.push(`Directory: ${result.metadata.directory}`);
  lines.push(`Duration: ${result.metadata.duration}ms`);
  lines.push(`Analyzers: ${result.metadata.analyzersRun.join(', ')}`);
  lines.push(`Frameworks: ${result.metadata.frameworks.join(', ')}`);
  lines.push('');

  lines.push(`Summary`);
  lines.push(`-------`);
  lines.push(`Total findings: ${result.summary.total}`);
  lines.push(`  Critical: ${result.summary.bySeverity.critical}`);
  lines.push(`  High:     ${result.summary.bySeverity.high}`);
  lines.push(`  Medium:   ${result.summary.bySeverity.medium}`);
  lines.push(`  Low:      ${result.summary.bySeverity.low}`);
  lines.push(`  Info:     ${result.summary.bySeverity.info}`);
  lines.push(`  Fixable:  ${result.summary.fixable}`);
  lines.push('');

  if (result.findings.length > 0) {
    lines.push(`Findings`);
    lines.push(`--------`);
    for (const finding of result.findings) {
      const location = finding.file
        ? `${finding.file}${finding.line ? ':' + finding.line : ''}`
        : 'project-level';
      lines.push(
        `[${finding.severity.toUpperCase()}] ${finding.id}: ${finding.title} (${location})`,
      );
      if (finding.evidence) {
        lines.push(`  Evidence: ${finding.evidence}`);
      }
    }
  }

  return lines.join('\n');
}
