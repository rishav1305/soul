#!/usr/bin/env node

import { runProbe } from './probe.js';
import { VERSION, initConfig } from './config.js';
import { PluginRegistry } from '@soul/plugins';
import { register as registerCompliance } from '@soul/compliance';

const args = process.argv.slice(2);

/**
 * Parse CLI flags from an argument list.
 * Supports `--key value` and `--flag` (boolean) patterns.
 */
function parseFlags(argv: string[]): Record<string, string | boolean> {
  const flags: Record<string, string | boolean> = {};
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg.startsWith('--')) {
      const key = arg.slice(2);
      const next = argv[i + 1];
      if (next === undefined || next.startsWith('--')) {
        flags[key] = true;
      } else {
        flags[key] = next;
        i++;
      }
    }
  }
  return flags;
}

/**
 * Route `soul compliance <subcommand> [directory] [--flags]` to the matching tool.
 */
async function runCompliance(registry: PluginRegistry, subArgs: string[]): Promise<void> {
  const subcommand = subArgs[0];
  const validSubcommands = ['scan', 'badge', 'report', 'fix', 'monitor'];

  if (!subcommand || !validSubcommands.includes(subcommand)) {
    console.log('Usage: soul compliance <command> [directory] [options]\n');
    console.log('Commands:');
    console.log('  scan      Run compliance scan');
    console.log('  badge     Generate compliance badge');
    console.log('  report    Generate compliance report');
    console.log('  fix       Auto-remediate compliance issues');
    console.log('  monitor   Watch mode for compliance changes');
    console.log('\nOptions:');
    console.log('  --framework soc2,hipaa   Frameworks to check');
    console.log('  --severity critical,high Severity filter');
    console.log('  --dry-run                Preview fixes without applying');
    console.log('  --format json            Output format');
    return;
  }

  const toolName = `compliance-${subcommand}`;
  const tool = registry.getTool(toolName);
  if (!tool) {
    console.error(`Unknown compliance tool: ${toolName}`);
    process.exit(1);
  }

  // Remaining args after the subcommand
  const rest = subArgs.slice(1);

  // First positional arg (not a flag) is the directory
  const positional = rest.filter((a) => !a.startsWith('--'));
  const directory = positional[0] ?? process.cwd();

  const flags = parseFlags(rest);

  // Build tool input from flags
  const input: Record<string, unknown> = {
    directory: directory.startsWith('/') ? directory : `${process.cwd()}/${directory}`,
  };

  if (flags.framework && typeof flags.framework === 'string') {
    input.frameworks = flags.framework.split(',');
  }

  if (flags.severity && typeof flags.severity === 'string') {
    input.severity = flags.severity.split(',');
  }

  if (flags['dry-run'] !== undefined) {
    input.dryRun = flags['dry-run'] === true || flags['dry-run'] === 'true';
  }

  if (flags.format && typeof flags.format === 'string') {
    input.format = flags.format;
  }

  try {
    const result = await tool.execute(input);
    if (result.success) {
      console.log(result.output);
    } else {
      console.error(result.output);
      process.exit(1);
    }
  } catch (err) {
    console.error(err instanceof Error ? err.message : String(err));
    process.exit(1);
  }
}

async function main(): Promise<void> {
  if (args.includes('--version') || args.includes('-v')) {
    console.log(`Soul v${VERSION}`);
    return;
  }

  if (args.includes('--probe')) {
    await runProbe();
    return;
  }

  if (args.includes('--help') || args.includes('-h')) {
    console.log(`\u25C6 Soul v${VERSION}\n`);
    console.log('Usage:');
    console.log('  soul                    Interactive mode');
    console.log('  soul compliance scan    Run compliance scan');
    console.log('  soul compliance badge   Generate compliance badge');
    console.log('  soul compliance report  Generate compliance report');
    console.log('  soul compliance fix     Auto-remediate issues');
    console.log('  soul compliance monitor Watch mode');
    console.log('  soul --probe            Test model provider');
    console.log('  soul --version          Show version');
    console.log('  soul --help             Show this help');
    return;
  }

  // Product subcommand routing
  if (args[0] === 'compliance') {
    const registry = new PluginRegistry();
    registerCompliance(registry);
    await runCompliance(registry, args.slice(1));
    return;
  }

  initConfig();
  console.log(`\u25C6 Soul v${VERSION}`);
  console.log('Interactive mode coming soon. Try:');
  console.log('  soul --probe        Test your model provider');
  console.log('  soul --help         See available commands');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
