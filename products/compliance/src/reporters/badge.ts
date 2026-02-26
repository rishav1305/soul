import type { ScanResult, Framework } from '../types.js';

/**
 * Calculate the compliance score as a percentage.
 *
 * Score = (totalRules - findings) / totalRules * 100
 * Clamped to [0, 100].
 */
export function calculateScore(totalFindings: number, totalRules: number): number {
  if (totalRules <= 0) return 0;
  const score = ((totalRules - totalFindings) / totalRules) * 100;
  return Math.max(0, Math.min(100, Math.round(score * 10) / 10));
}

/**
 * Return the badge color based on score thresholds.
 *
 * - Green (#4c1) for scores above 80%
 * - Yellow (#dfb317) for scores between 60% and 80% (inclusive)
 * - Red (#e05d44) for scores below 60%
 */
export function scoreColor(score: number): string {
  if (score > 80) return '#4c1';
  if (score >= 60) return '#dfb317';
  return '#e05d44';
}

/**
 * Build a human-readable label from the scanned frameworks.
 */
function frameworkLabel(frameworks: Framework[]): string {
  if (frameworks.length === 0) return 'compliance';
  return frameworks.map((f) => f.toUpperCase()).join(' + ');
}

/**
 * Generate an SVG compliance badge in shields.io flat style.
 *
 * The badge displays the framework name(s) on the left half and the
 * compliance score percentage on the right half, colored according to
 * the score thresholds.
 */
export function generateBadge(result: ScanResult, totalRules: number): string {
  const score = calculateScore(result.summary.total, totalRules);
  const color = scoreColor(score);
  const label = frameworkLabel(result.metadata.frameworks);
  const scoreText = `${score}%`;

  // Calculate text widths (approximate: 6.5px per character + padding)
  const labelWidth = Math.max(label.length * 6.5 + 10, 40);
  const valueWidth = Math.max(scoreText.length * 6.5 + 10, 40);
  const totalWidth = labelWidth + valueWidth;

  return `<svg xmlns="http://www.w3.org/2000/svg" width="${totalWidth}" height="20" role="img" aria-label="${label}: ${scoreText}">
  <title>${label}: ${scoreText}</title>
  <linearGradient id="s" x2="0" y2="100%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r">
    <rect width="${totalWidth}" height="20" rx="3" fill="#fff"/>
  </clipPath>
  <g clip-path="url(#r)">
    <rect width="${labelWidth}" height="20" fill="#555"/>
    <rect x="${labelWidth}" width="${valueWidth}" height="20" fill="${color}"/>
    <rect width="${totalWidth}" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="11">
    <text x="${labelWidth / 2}" y="14">${label}</text>
    <text x="${labelWidth + valueWidth / 2}" y="14">${scoreText}</text>
  </g>
</svg>`;
}
