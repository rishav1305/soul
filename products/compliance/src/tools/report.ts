import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { z } from 'zod';
import type { ToolDefinition, ToolResult } from '@soul/plugins';
import { requireTier } from '@soul/core';
import { runScan } from './scan.js';
import { loadRules } from '../rules/index.js';
import { generateHtml } from '../reporters/html.js';
import { generatePdf } from '../reporters/pdf.js';
import type { Framework } from '../types.js';

/**
 * Zod input schema for the report tool.
 */
const ReportInputSchema = z.object({
  directory: z.string().describe('Absolute path to the directory to scan'),
  frameworks: z
    .array(z.enum(['soc2', 'hipaa', 'gdpr']))
    .optional()
    .describe('Frameworks to check against (default: all)'),
  format: z
    .enum(['html', 'pdf'])
    .default('html')
    .describe('Output format: html or pdf (default: html)'),
  output: z
    .string()
    .optional()
    .describe('Custom output path for the report file'),
});

/**
 * Create a ToolDefinition for HTML/PDF report generation.
 *
 * This tool is gated behind the `pro` tier. Free-tier users receive
 * terminal and JSON output only.
 */
export function createReportTool(): ToolDefinition {
  return {
    name: 'compliance-report',
    description:
      'Generate an HTML or PDF compliance report for a project directory (requires Soul Pro)',
    product: 'compliance',
    inputSchema: ReportInputSchema,
    requiresApproval: false,
    execute: async (input: unknown): Promise<ToolResult> => {
      // Tier gate: free tier cannot generate HTML/PDF reports
      requireTier('pro', 'HTML/PDF reports');

      const parsed = ReportInputSchema.parse(input);

      // Run the scan
      const result = await runScan({
        directory: parsed.directory,
        frameworks: parsed.frameworks,
      });

      // Load rules to get total count
      const rules = loadRules({ frameworks: parsed.frameworks });
      const totalRules = rules.length;

      // Generate HTML
      const html = generateHtml(result, totalRules);

      if (parsed.format === 'pdf') {
        const pdfPath =
          parsed.output ?? join(parsed.directory, 'compliance-report.pdf');
        await generatePdf(html, pdfPath);

        return {
          success: true,
          output: `PDF compliance report generated at ${pdfPath}`,
          structured: {
            format: 'pdf',
            path: pdfPath,
            totalFindings: result.summary.total,
            totalRules,
          },
          artifacts: [{ type: 'report', path: pdfPath }],
        };
      }

      // HTML output
      const htmlPath =
        parsed.output ?? join(parsed.directory, 'compliance-report.html');
      writeFileSync(htmlPath, html, 'utf-8');

      return {
        success: true,
        output: `HTML compliance report generated at ${htmlPath}`,
        structured: {
          format: 'html',
          path: htmlPath,
          totalFindings: result.summary.total,
          totalRules,
        },
        artifacts: [{ type: 'report', path: htmlPath, content: html }],
      };
    },
  };
}
