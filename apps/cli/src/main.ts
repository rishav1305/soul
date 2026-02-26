#!/usr/bin/env node

import { runProbe } from './probe.js';
import { VERSION, initConfig } from './config.js';

const args = process.argv.slice(2);

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
    console.log('  soul --probe            Test model provider');
    console.log('  soul --version          Show version');
    console.log('  soul --help             Show this help');
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
