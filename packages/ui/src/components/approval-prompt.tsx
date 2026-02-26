import React from 'react';
import { Box, Text } from 'ink';
import { theme } from '../theme.js';
interface ApprovalPromptProps { message: string; detail?: string; }
export function ApprovalPrompt({ message, detail }: ApprovalPromptProps) {
  return (
    <Box flexDirection="column" marginTop={1}>
      {detail && <Text>{detail}</Text>}
      <Text>{theme.warning(message)} {theme.muted('[Y/n]')}</Text>
    </Box>
  );
}
