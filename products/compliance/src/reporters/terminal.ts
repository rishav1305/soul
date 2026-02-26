import { theme } from '@soul/ui';
import type { ScanResult, Severity, Finding } from '../types.js';

const severityColors: Record<Severity, (s: string) => string> = {
  critical: theme.error,
  high: theme.error,
  medium: theme.warning,
  low: theme.info,
  info: theme.muted,
};

const severityIcons: Record<Severity, string> = {
  critical: '\u2717',
  high: '\u2717',
  medium: '\u26A0',
  low: '\u25CB',
  info: '\u2139',
};

const severityOrder: Severity[] = ['critical', 'high', 'medium', 'low', 'info'];

function formatLocation(finding: Finding): string {
  if (!finding.file) return '';
  let loc = finding.file;
  if (finding.line != null) {
    loc += `:${finding.line}`;
    if (finding.column != null) {
      loc += `:${finding.column}`;
    }
  }
  return loc;
}

export function formatTerminal(result: ScanResult): string {
  const lines: string[] = [];

  lines.push('');
  lines.push(theme.brand(`${theme.marker} Soul Compliance Scan Results`));
  lines.push(theme.muted(`  Directory: ${result.metadata.directory}`));
  lines.push(theme.muted(`  Duration:  ${result.metadata.duration}ms`));
  lines.push(theme.muted(`  Analyzers: ${result.metadata.analyzersRun.join(', ')}`));
  lines.push('');

  if (result.findings.length === 0) {
    lines.push(theme.success('  No findings! Your project looks clean.'));
    lines.push('');
    return lines.join('\n');
  }

  // Group findings by severity
  const grouped = new Map<Severity, Finding[]>();
  for (const finding of result.findings) {
    const list = grouped.get(finding.severity) ?? [];
    list.push(finding);
    grouped.set(finding.severity, list);
  }

  for (const severity of severityOrder) {
    const findings = grouped.get(severity);
    if (!findings?.length) continue;

    const color = severityColors[severity];
    const icon = severityIcons[severity];

    lines.push(color(`  ${severity.toUpperCase()} (${findings.length})`));

    for (const finding of findings) {
      lines.push(`    ${color(icon)} ${finding.title}`);
      const loc = formatLocation(finding);
      if (loc) {
        lines.push(`      ${theme.muted(`\u2192 ${loc}`)}`);
      }
      if (finding.evidence) {
        lines.push(`      ${theme.muted(finding.evidence)}`);
      }
    }

    lines.push('');
  }

  // Summary line
  const parts: string[] = [];
  parts.push(`${result.summary.total} finding${result.summary.total !== 1 ? 's' : ''}`);

  const critHigh = (result.summary.bySeverity.critical ?? 0) + (result.summary.bySeverity.high ?? 0);
  if (critHigh > 0) {
    parts.push(theme.error(`${critHigh} critical/high`));
  }
  if (result.summary.fixable > 0) {
    parts.push(theme.success(`${result.summary.fixable} auto-fixable`));
  }

  lines.push(theme.muted(`  Summary: ${parts.join(' | ')}`));
  lines.push('');

  return lines.join('\n');
}
