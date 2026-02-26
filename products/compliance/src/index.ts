import type { PluginRegistry } from '@soul/plugins';

export function register(_registry: PluginRegistry): void {
  // Tools and commands will be registered as they are built
}

export type { Finding, ScanResult, Severity, Framework, RuleDefinition, ScanOptions } from './types.js';
