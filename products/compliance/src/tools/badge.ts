import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { z } from 'zod';
import type { ToolDefinition, ToolResult } from '@soul/plugins';
import { runScan } from './scan.js';
import { loadRules } from '../rules/index.js';
import { generateBadge } from '../reporters/badge.js';
import type { Framework } from '../types.js';

/**
 * Zod input schema for the badge tool.
 */
const BadgeInputSchema = z.object({
  directory: z.string().describe('Absolute path to the directory to scan'),
  frameworks: z
    .array(z.enum(['soc2', 'hipaa', 'gdpr']))
    .optional()
    .describe('Frameworks to check against (default: all)'),
});

/**
 * Create a ToolDefinition for the compliance badge generator.
 *
 * The tool runs a compliance scan, loads the rule set to determine totalRules,
 * generates a shields.io-style SVG badge, writes it to the scanned directory,
 * and returns the SVG as an artifact.
 */
export function createBadgeTool(): ToolDefinition {
  return {
    name: 'compliance-badge',
    description:
      'Generate an SVG compliance badge for a project directory showing the compliance score',
    product: 'compliance',
    inputSchema: BadgeInputSchema,
    requiresApproval: false,
    execute: async (input: unknown): Promise<ToolResult> => {
      const parsed = BadgeInputSchema.parse(input);

      // Run the scan
      const result = await runScan({
        directory: parsed.directory,
        frameworks: parsed.frameworks,
      });

      // Load rules to get total count (matching the framework filter)
      const rules = loadRules({ frameworks: parsed.frameworks });
      const totalRules = rules.length;

      // Generate the SVG badge
      const svg = generateBadge(result, totalRules);

      // Write badge to the scanned directory
      const badgePath = join(parsed.directory, 'compliance-badge.svg');
      writeFileSync(badgePath, svg, 'utf-8');

      const score =
        totalRules > 0
          ? Math.round(((totalRules - result.summary.total) / totalRules) * 100 * 10) / 10
          : 0;

      return {
        success: true,
        output: `Compliance badge generated: ${score}% (${result.summary.total} findings out of ${totalRules} rules). Written to ${badgePath}`,
        structured: {
          score,
          totalFindings: result.summary.total,
          totalRules,
          badgePath,
        },
        artifacts: [
          {
            type: 'badge',
            path: badgePath,
            content: svg,
          },
        ],
      };
    },
  };
}
