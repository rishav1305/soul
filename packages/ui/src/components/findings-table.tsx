import React from 'react';
import { Box, Text } from 'ink';
import { theme } from '../theme.js';

export interface Finding { severity: 'critical' | 'high' | 'medium' | 'low'; rule: string; message: string; file?: string; line?: number; }
interface FindingsTableProps { findings: Finding[]; score?: number; }

const severityColor = { critical: theme.error, high: theme.error, medium: theme.warning, low: theme.info };
const severityIcon = { critical: '\u2717', high: '\u2717', medium: '\u26A0', low: '\u25CB' };

export function FindingsTable({ findings, score }: FindingsTableProps) {
  const grouped = new Map<string, Finding[]>();
  for (const f of findings) { const list = grouped.get(f.severity) ?? []; list.push(f); grouped.set(f.severity, list); }
  const order: Finding['severity'][] = ['critical', 'high', 'medium', 'low'];
  return (
    <Box flexDirection="column" borderStyle="single" paddingX={1}>
      {order.map((sev) => {
        const items = grouped.get(sev);
        if (!items?.length) return null;
        const color = severityColor[sev];
        const icon = severityIcon[sev];
        return (
          <Box key={sev} flexDirection="column" marginBottom={1}>
            <Text>{color(`${sev.toUpperCase()} (${items.length})`)}</Text>
            {items.map((f, i) => (
              <Box key={i} flexDirection="column" marginLeft={1}>
                <Text> {color(icon)} {f.message}</Text>
                {f.file && <Text>   {theme.muted(`\u2192 ${f.file}${f.line ? `:${f.line}` : ''}`)}</Text>}
              </Box>
            ))}
          </Box>
        );
      })}
      {score !== undefined && <Text>{theme.muted(`Score: ${score}/100 \u2014 ${findings.length} violations found`)}</Text>}
    </Box>
  );
}
