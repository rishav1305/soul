import type { ScanResult, Severity, Framework, Finding } from '../types.js';
import { calculateScore, scoreColor } from './badge.js';

const severityOrder: Severity[] = ['critical', 'high', 'medium', 'low', 'info'];

const severityColorMap: Record<Severity, string> = {
  critical: '#e05d44',
  high: '#d9534f',
  medium: '#f0ad4e',
  low: '#5bc0de',
  info: '#999',
};

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function severityBarChart(bySeverity: Record<Severity, number>): string {
  const maxCount = Math.max(...Object.values(bySeverity), 1);
  const barWidth = 200;
  const barHeight = 24;
  const gap = 6;
  const labelWidth = 70;
  const countWidth = 40;
  const totalHeight = severityOrder.length * (barHeight + gap);

  const bars = severityOrder
    .map((sev, i) => {
      const count = bySeverity[sev];
      const w = Math.max((count / maxCount) * barWidth, 0);
      const y = i * (barHeight + gap);
      const color = severityColorMap[sev];
      return `
      <g transform="translate(0,${y})">
        <text x="${labelWidth - 4}" y="${barHeight / 2 + 5}" text-anchor="end" font-size="13" fill="#444">${sev.toUpperCase()}</text>
        <rect x="${labelWidth}" y="0" width="${w}" height="${barHeight}" rx="3" fill="${color}" />
        <text x="${labelWidth + w + 6}" y="${barHeight / 2 + 5}" font-size="13" fill="#333">${count}</text>
      </g>`;
    })
    .join('');

  return `<svg width="${labelWidth + barWidth + countWidth}" height="${totalHeight}" xmlns="http://www.w3.org/2000/svg" role="img" aria-label="Severity distribution chart">${bars}</svg>`;
}

function frameworkSection(findings: Finding[], frameworks: Framework[]): string {
  const rows = frameworks
    .map((fw) => {
      const fwFindings = findings.filter((f) => f.framework.includes(fw));
      const count = fwFindings.length;
      return `<tr><td>${fw.toUpperCase()}</td><td>${count}</td></tr>`;
    })
    .join('');
  return `
    <table class="fw-table">
      <thead><tr><th>Framework</th><th>Findings</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>`;
}

function findingsTable(findings: Finding[], severity: Severity): string {
  const filtered = findings.filter((f) => f.severity === severity);
  if (filtered.length === 0) return '';

  const rows = filtered
    .map(
      (f) => `
      <tr>
        <td>${escapeHtml(f.id)}</td>
        <td>${escapeHtml(f.title)}</td>
        <td>${f.file ? escapeHtml(f.file) + (f.line ? ':' + f.line : '') : 'project-level'}</td>
        <td>${f.framework.map((fw) => fw.toUpperCase()).join(', ')}</td>
        <td>${f.fixable ? 'Yes' : 'No'}</td>
      </tr>`,
    )
    .join('');

  const color = severityColorMap[severity];

  return `
    <details open>
      <summary style="color:${color};font-weight:bold;font-size:1.1em;cursor:pointer;">
        ${severity.toUpperCase()} (${filtered.length})
      </summary>
      <table class="findings-table">
        <thead>
          <tr><th>ID</th><th>Title</th><th>Location</th><th>Frameworks</th><th>Fixable</th></tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>
    </details>`;
}

/**
 * Generate a standalone HTML compliance report with embedded CSS.
 *
 * Sections:
 *  - Executive summary (score, scan date, framework coverage)
 *  - Findings by severity (collapsible)
 *  - Findings by framework
 *  - Inline SVG severity distribution chart
 *  - Soul branding header/footer
 */
export function generateHtml(result: ScanResult, totalRules: number): string {
  const score = calculateScore(result.summary.total, totalRules);
  const color = scoreColor(score);
  const scanDate = result.metadata.timestamp;
  const frameworkList = result.metadata.frameworks.map((f) => f.toUpperCase()).join(', ');

  const severitySections = severityOrder
    .map((sev) => findingsTable(result.findings, sev))
    .join('');

  const chart = severityBarChart(result.summary.bySeverity);

  const fwSection = frameworkSection(result.findings, result.metadata.frameworks);

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>Soul Compliance Report</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #333; background: #f9fafb; line-height: 1.6; }
  .header { background: #1a1a2e; color: #fff; padding: 24px 32px; display: flex; align-items: center; justify-content: space-between; }
  .header h1 { font-size: 1.6em; font-weight: 600; }
  .header .brand { font-size: 0.9em; opacity: 0.8; }
  .container { max-width: 960px; margin: 0 auto; padding: 24px; }
  .executive-summary { background: #fff; border-radius: 8px; padding: 24px; margin-bottom: 24px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }
  .executive-summary h2 { margin-bottom: 16px; font-size: 1.3em; }
  .score-badge { display: inline-block; font-size: 2em; font-weight: 700; padding: 8px 20px; border-radius: 8px; color: #fff; }
  .meta-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 16px; }
  .meta-grid dt { font-weight: 600; color: #555; }
  .meta-grid dd { margin: 0 0 8px 0; }
  .section { background: #fff; border-radius: 8px; padding: 24px; margin-bottom: 24px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }
  .section h2 { margin-bottom: 16px; font-size: 1.2em; }
  details { margin-bottom: 12px; }
  summary { padding: 8px 0; }
  table { width: 100%; border-collapse: collapse; margin-top: 8px; }
  th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #eee; font-size: 0.9em; }
  th { background: #f5f5f5; font-weight: 600; }
  .findings-table { margin-bottom: 16px; }
  .fw-table { max-width: 400px; }
  .chart-wrapper { display: flex; justify-content: center; padding: 16px 0; }
  .footer { text-align: center; padding: 24px; color: #888; font-size: 0.85em; border-top: 1px solid #eee; margin-top: 32px; }
</style>
</head>
<body>
<div class="header">
  <h1>Soul Compliance Report</h1>
  <span class="brand">Powered by Soul</span>
</div>
<div class="container">
  <div class="executive-summary">
    <h2>Executive Summary</h2>
    <span class="score-badge" style="background:${color}">${score}%</span>
    <dl class="meta-grid">
      <dt>Scan Date</dt>
      <dd>${escapeHtml(scanDate)}</dd>
      <dt>Directory</dt>
      <dd>${escapeHtml(result.metadata.directory)}</dd>
      <dt>Frameworks</dt>
      <dd>${frameworkList}</dd>
      <dt>Total Findings</dt>
      <dd>${result.summary.total}</dd>
      <dt>Total Rules</dt>
      <dd>${totalRules}</dd>
      <dt>Duration</dt>
      <dd>${result.metadata.duration}ms</dd>
      <dt>Analyzers</dt>
      <dd>${result.metadata.analyzersRun.join(', ')}</dd>
      <dt>Auto-fixable</dt>
      <dd>${result.summary.fixable}</dd>
    </dl>
  </div>

  <div class="section">
    <h2>Severity Distribution</h2>
    <div class="chart-wrapper">${chart}</div>
  </div>

  <div class="section">
    <h2>Findings by Severity</h2>
    ${severitySections || '<p>No findings.</p>'}
  </div>

  <div class="section">
    <h2>Findings by Framework</h2>
    ${fwSection}
  </div>
</div>
<div class="footer">
  Generated by Soul Compliance &mdash; ${escapeHtml(scanDate)}
</div>
</body>
</html>`;
}
