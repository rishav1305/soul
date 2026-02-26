import { ModelRouter } from '@soul/models';

export async function runProbe(): Promise<void> {
  console.log('Probing model providers...\n');

  try {
    const router = await ModelRouter.autoDetect();
    const provider = router.getProviderName();

    if (provider === 'claude-api') {
      console.log('\u2713 OAuth token found in ~/.claude/.credentials.json');
      console.log('\u2713 Direct API mode: SUCCESS');
      console.log(`\n\u25C6 Soul is ready. Provider: claude-api (Max subscription)`);
    } else if (provider === 'claude-cli') {
      console.log('\u2717 Direct API mode: unavailable (falling back)');
      console.log('\u2713 Claude CLI binary found');
      console.log(`\n\u25C6 Soul is ready. Provider: claude-cli (wrapper mode)`);
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error(`\u2717 No provider available: ${msg}`);
    console.error('\nEnsure Claude Code is installed and you are logged in:');
    console.error('  npm i -g @anthropic-ai/claude-code');
    console.error('  claude auth login');
    process.exit(1);
  }
}
