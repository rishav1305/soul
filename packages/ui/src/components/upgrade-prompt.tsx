import React from 'react';
import { Box, Text } from 'ink';
import { theme } from '../theme.js';
interface UpgradePromptProps { feature: string; tier: string; price: string; }
export function UpgradePrompt({ feature, tier, price }: UpgradePromptProps) {
  return (
    <Box flexDirection="column" marginTop={1} borderStyle="single" paddingX={1}>
      <Text>{theme.warning(`\u26A1 ${feature} requires Soul ${tier}`)}</Text>
      <Text>{theme.muted(`   ${price}`)}</Text>
      <Text> </Text>
      <Text>  {theme.info('\u2192 soul upgrade')}          {theme.muted('(opens soul.dev)')}</Text>
    </Box>
  );
}
