import { describe, it, expect } from 'vitest';
import { formatTerminal } from '../src/reporters/terminal.js';
import { formatJSON } from '../src/reporters/json.js';
import type { ScanResult } from '../src/types.js';

function makeScanResult(): ScanResult {
  return {
    findings: [
      {
        id: 'SECRET001',
        title: 'Hardcoded API key detected',
        description: 'API key found in source code',
        severity: 'critical',
        framework: ['soc2', 'hipaa'],
        controlIds: ['CC6.1'],
        file: 'src/config.ts',
        line: 12,
        column: 5,
        evidence: 'const API_KEY = "sk-..."',
        analyzer: 'secret-scanner',
        fixable: false,
      },
      {
        id: 'DEP001',
        title: 'Vulnerable dependency: lodash@4.17.20',
        description: 'Known prototype pollution vulnerability',
        severity: 'high',
        framework: ['soc2'],
        controlIds: ['CC7.1'],
        file: 'package.json',
        line: 15,
        analyzer: 'dependency-auditor',
        fixable: true,
        fix: { description: 'Upgrade lodash to 4.17.21', patch: '' },
      },
      {
        id: 'CFG001',
        title: 'Missing HTTPS enforcement',
        description: 'Server not configured for TLS',
        severity: 'medium',
        framework: ['soc2', 'gdpr'],
        controlIds: ['CC6.7'],
        file: 'src/server.ts',
        line: 3,
        analyzer: 'config-checker',
        fixable: false,
      },
      {
        id: 'CFG002',
        title: 'Debug mode enabled',
        description: 'Debug mode should not be enabled in production',
        severity: 'low',
        framework: ['soc2'],
        controlIds: ['CC7.2'],
        file: '.env',
        line: 1,
        analyzer: 'config-checker',
        fixable: true,
        fix: { description: 'Set DEBUG=false', patch: '' },
      },
      {
        id: 'INFO001',
        title: 'No .gitignore found',
        description: 'Repository lacks a .gitignore file',
        severity: 'info',
        framework: ['soc2'],
        controlIds: ['CC8.1'],
        analyzer: 'git-analyzer',
        fixable: false,
      },
    ],
    summary: {
      total: 5,
      bySeverity: { critical: 1, high: 1, medium: 1, low: 1, info: 1 },
      byFramework: { soc2: 5, hipaa: 1, gdpr: 1 },
      byAnalyzer: {
        'secret-scanner': 1,
        'dependency-auditor': 1,
        'config-checker': 2,
        'git-analyzer': 1,
      },
      fixable: 2,
    },
    metadata: {
      directory: '/tmp/test-project',
      duration: 342,
      analyzersRun: ['secret-scanner', 'dependency-auditor', 'config-checker', 'git-analyzer'],
      frameworks: ['soc2', 'hipaa', 'gdpr'],
      timestamp: '2026-02-26T12:00:00.000Z',
    },
  };
}

describe('Terminal reporter', () => {
  it('returns formatted string with severity sections', () => {
    const result = makeScanResult();
    const output = formatTerminal(result);

    // Should contain severity section headers
    expect(output).toContain('CRITICAL (1)');
    expect(output).toContain('HIGH (1)');
    expect(output).toContain('MEDIUM (1)');
    expect(output).toContain('LOW (1)');
    expect(output).toContain('INFO (1)');

    // Should contain finding titles
    expect(output).toContain('Hardcoded API key detected');
    expect(output).toContain('Vulnerable dependency: lodash@4.17.20');
    expect(output).toContain('Missing HTTPS enforcement');

    // Should contain file:line references
    expect(output).toContain('src/config.ts:12:5');
    expect(output).toContain('package.json:15');
    expect(output).toContain('src/server.ts:3');

    // Should contain summary
    expect(output).toContain('5 findings');
    expect(output).toContain('2 auto-fixable');
  });

  it('shows clean message when no findings', () => {
    const result = makeScanResult();
    result.findings = [];
    result.summary.total = 0;
    const output = formatTerminal(result);
    expect(output).toContain('No findings');
  });

  it('includes metadata in header', () => {
    const result = makeScanResult();
    const output = formatTerminal(result);
    expect(output).toContain('/tmp/test-project');
    expect(output).toContain('342ms');
    expect(output).toContain('secret-scanner');
  });
});

describe('JSON reporter', () => {
  it('returns valid JSON matching ScanResult schema', () => {
    const result = makeScanResult();
    const json = formatJSON(result);

    // Should be valid JSON
    const parsed = JSON.parse(json);

    // Should have top-level ScanResult keys
    expect(parsed).toHaveProperty('findings');
    expect(parsed).toHaveProperty('summary');
    expect(parsed).toHaveProperty('metadata');

    // Should have correct counts
    expect(parsed.findings).toHaveLength(5);
    expect(parsed.summary.total).toBe(5);
    expect(parsed.summary.bySeverity.critical).toBe(1);
    expect(parsed.metadata.directory).toBe('/tmp/test-project');
  });

  it('output is parseable back to ScanResult', () => {
    const original = makeScanResult();
    const json = formatJSON(original);
    const parsed: ScanResult = JSON.parse(json);

    // Deep equality check: parsed result should match original
    expect(parsed).toEqual(original);
  });

  it('produces pretty-printed output with indentation', () => {
    const result = makeScanResult();
    const json = formatJSON(result);

    // Pretty-printed JSON uses newlines and indentation
    expect(json).toContain('\n');
    expect(json).toContain('  ');
    // Should not be a single line
    expect(json.split('\n').length).toBeGreaterThan(10);
  });
});
