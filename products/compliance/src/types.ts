export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info';
export type Framework = 'soc2' | 'hipaa' | 'gdpr';

export interface Finding {
  id: string;
  title: string;
  description: string;
  severity: Severity;
  framework: Framework[];
  controlIds: string[];
  file?: string;
  line?: number;
  column?: number;
  evidence?: string;
  analyzer: string;
  fixable: boolean;
  fix?: FixSuggestion;
}

export interface FixSuggestion {
  description: string;
  patch: string;
}

export interface ScanResult {
  findings: Finding[];
  summary: {
    total: number;
    bySeverity: Record<Severity, number>;
    byFramework: Record<Framework, number>;
    byAnalyzer: Record<string, number>;
    fixable: number;
  };
  metadata: {
    directory: string;
    duration: number;
    analyzersRun: string[];
    analyzerFailures?: Array<{ analyzer: string; error: string }>;
    frameworks: Framework[];
    timestamp: string;
  };
}

export interface RuleDefinition {
  id: string;
  title: string;
  severity: Severity;
  analyzer: string;
  pattern: string;
  controls: string[];
  framework: Framework[];
  description: string;
  fixable: boolean;
}

export interface ScanOptions {
  directory: string;
  frameworks?: Framework[];
  severity?: Severity[];
  analyzers?: string[];
  exclude?: string[];
  format?: 'terminal' | 'json';
  output?: string;
}

export interface Analyzer {
  name: string;
  analyze(files: ScannedFile[], rules: RuleDefinition[]): Promise<Finding[]>;
}

// Re-export for convenience
import type { ScannedFile } from '@soul/context';
export type { ScannedFile };
