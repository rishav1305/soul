import React from 'react';
import { Box, Text } from 'ink';
import { theme } from '../theme.js';

interface HeaderProps { version: string; product?: string; provider?: string; }

export function Header({ version, product, provider }: HeaderProps) {
  const productLabel = product ? ` (${product})` : '';
  return (
    <Box flexDirection="column" marginBottom={1}>
      <Text>{theme.brand(`${theme.marker} Soul v${version}${productLabel}`)}</Text>
      {provider && <Text>{theme.muted(`  Provider: ${provider}`)}</Text>}
    </Box>
  );
}
