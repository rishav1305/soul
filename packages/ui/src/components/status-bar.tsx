import React from 'react';
import { Box, Text } from 'ink';
import { theme } from '../theme.js';
interface StatusBarProps { provider: string; inputTokens: number; outputTokens: number; costUsd: number; }
export function StatusBar({ provider, inputTokens, outputTokens, costUsd }: StatusBarProps) {
  const totalTokens = inputTokens + outputTokens;
  const tokenStr = totalTokens > 1000 ? `${(totalTokens / 1000).toFixed(1)}K` : String(totalTokens);
  const costStr = costUsd === 0 ? '$0' : `$${costUsd.toFixed(4)}`;
  return <Box marginTop={1}><Text>{theme.muted(`Provider: ${provider} \u2022 Tokens: ${tokenStr} \u2022 ${costStr}`)}</Text></Box>;
}
