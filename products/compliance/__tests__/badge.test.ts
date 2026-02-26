import { describe, it, expect } from 'vitest';
import { generateBadge, calculateScore, scoreColor } from '../src/reporters/badge.js';
import type { ScanResult } from '../src/types.js';

function makeScanResult(overrides?: {
  totalFindings?: number;
  frameworks?: ('soc2' | 'hipaa' | 'gdpr')[];
}): ScanResult {
  const totalFindings = overrides?.totalFindings ?? 5;
  const frameworks = overrides?.frameworks ?? ['soc2', 'hipaa', 'gdpr'];

  // Build a minimal list of findings matching the requested count
  const findings = Array.from({ length: totalFindings }, (_, i) => ({
    id: `FINDING${i + 1}`,
    title: `Test finding ${i + 1}`,
    description: `Description for finding ${i + 1}`,
    severity: 'medium' as const,
    framework: ['soc2'] as ('soc2' | 'hipaa' | 'gdpr')[],
    controlIds: ['CC1.1'],
    analyzer: 'test-analyzer',
    fixable: false,
  }));

  return {
    findings,
    summary: {
      total: totalFindings,
      bySeverity: { critical: 0, high: 0, medium: totalFindings, low: 0, info: 0 },
      byFramework: { soc2: totalFindings, hipaa: 0, gdpr: 0 },
      byAnalyzer: { 'test-analyzer': totalFindings },
      fixable: 0,
    },
    metadata: {
      directory: '/tmp/test-project',
      duration: 100,
      analyzersRun: ['test-analyzer'],
      frameworks,
      timestamp: '2026-02-26T12:00:00.000Z',
    },
  };
}

describe('generateBadge', () => {
  it('generates valid SVG with opening and closing tags', () => {
    const result = makeScanResult({ totalFindings: 5 });
    const svg = generateBadge(result, 100);

    expect(svg).toContain('<svg');
    expect(svg).toContain('</svg>');
  });

  it('contains framework name in badge text', () => {
    const result = makeScanResult({ totalFindings: 0, frameworks: ['soc2'] });
    const svg = generateBadge(result, 50);

    expect(svg).toContain('SOC2');
  });

  it('contains multiple framework names when scanning multiple', () => {
    const result = makeScanResult({ totalFindings: 0, frameworks: ['soc2', 'hipaa'] });
    const svg = generateBadge(result, 50);

    expect(svg).toContain('SOC2');
    expect(svg).toContain('HIPAA');
  });

  it('contains the score percentage in badge text', () => {
    const result = makeScanResult({ totalFindings: 10 });
    const svg = generateBadge(result, 100);

    // Score = (100 - 10) / 100 * 100 = 90%
    expect(svg).toContain('90%');
  });

  it('uses green color for score above 80%', () => {
    const result = makeScanResult({ totalFindings: 5 });
    const svg = generateBadge(result, 100);

    // Score = 95% -> green (#4c1)
    expect(svg).toContain('#4c1');
  });

  it('uses yellow color for score between 60% and 80%', () => {
    const result = makeScanResult({ totalFindings: 30 });
    const svg = generateBadge(result, 100);

    // Score = 70% -> yellow (#dfb317)
    expect(svg).toContain('#dfb317');
  });

  it('uses red color for score below 60%', () => {
    const result = makeScanResult({ totalFindings: 60 });
    const svg = generateBadge(result, 100);

    // Score = 40% -> red (#e05d44)
    expect(svg).toContain('#e05d44');
  });
});

describe('calculateScore', () => {
  it('computes (totalRules - findings) / totalRules * 100', () => {
    expect(calculateScore(10, 100)).toBe(90);
    expect(calculateScore(0, 100)).toBe(100);
    expect(calculateScore(50, 100)).toBe(50);
    expect(calculateScore(100, 100)).toBe(0);
  });

  it('clamps score to 0 when findings exceed totalRules', () => {
    expect(calculateScore(150, 100)).toBe(0);
  });

  it('returns 0 when totalRules is 0', () => {
    expect(calculateScore(5, 0)).toBe(0);
  });
});

describe('scoreColor', () => {
  it('returns green for scores above 80', () => {
    expect(scoreColor(81)).toBe('#4c1');
    expect(scoreColor(100)).toBe('#4c1');
    expect(scoreColor(90)).toBe('#4c1');
  });

  it('returns yellow for scores from 60 to 80 inclusive', () => {
    expect(scoreColor(80)).toBe('#dfb317');
    expect(scoreColor(60)).toBe('#dfb317');
    expect(scoreColor(70)).toBe('#dfb317');
  });

  it('returns red for scores below 60', () => {
    expect(scoreColor(59)).toBe('#e05d44');
    expect(scoreColor(0)).toBe('#e05d44');
    expect(scoreColor(30)).toBe('#e05d44');
  });
});
