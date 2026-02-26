import { readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { z } from 'zod';
import { createPatch } from 'diff';
import { requireTier } from '@soul/core';
import type { ToolDefinition, ToolResult } from '@soul/plugins';
import { runScan } from './scan.js';
import type { Finding, ScanResult } from '../types.js';

/**
 * A generated remediation patch for a single finding.
 */
export interface RemediationPatch {
  findingId: string;
  file: string;
  description: string;
  patch: string;
}

/**
 * Result of the fix operation.
 */
export interface FixResult {
  patches: RemediationPatch[];
  applied: boolean;
  summary: {
    totalFixable: number;
    patchesGenerated: number;
    patchesApplied: number;
  };
}

// ── Remediation generators ─────────────────────────────────────────────

/**
 * Derive an environment variable name from a secret's context.
 *
 * Inspects the line containing the secret for an assignment like
 * `const API_KEY = '...'` and converts the variable name to an
 * upper-case env var name. Falls back to `SECRET_VALUE`.
 */
export function deriveEnvVarName(line: string): string {
  // Match patterns like: const VARIABLE_NAME = '...' or let variableName = '...'
  const assignmentMatch = line.match(
    /(?:const|let|var|export\s+(?:const|let|var))\s+(\w+)\s*=/,
  );
  if (assignmentMatch) {
    // Convert camelCase to UPPER_SNAKE_CASE
    return assignmentMatch[1]
      .replace(/([a-z])([A-Z])/g, '$1_$2')
      .toUpperCase();
  }

  // Match object property patterns: key: 'value' or key = 'value'
  const propMatch = line.match(/(\w+)\s*[:=]\s*['"`]/);
  if (propMatch) {
    return propMatch[1].replace(/([a-z])([A-Z])/g, '$1_$2').toUpperCase();
  }

  return 'SECRET_VALUE';
}

/**
 * Generate a remediation for a hardcoded secret: replace the secret value
 * with `process.env.VARIABLE_NAME`.
 */
export function generateSecretFix(
  filePath: string,
  finding: Finding,
): RemediationPatch | null {
  if (!finding.file || finding.line == null) return null;

  let content: string;
  try {
    content = readFileSync(filePath, 'utf-8');
  } catch {
    return null;
  }

  const lines = content.split('\n');
  const lineIdx = finding.line - 1;
  if (lineIdx < 0 || lineIdx >= lines.length) return null;

  const originalLine = lines[lineIdx];

  // Find the quoted secret on this line and replace it with process.env.VAR
  const envVar = deriveEnvVarName(originalLine);

  // Replace quoted string values (single, double, or backtick)
  // Match the largest quoted string on the line
  const newLine = originalLine.replace(
    /(['"`])([^'"`]{4,})\1/,
    `process.env.${envVar}`,
  );

  if (newLine === originalLine) return null;

  const newLines = [...lines];
  newLines[lineIdx] = newLine;

  const newContent = newLines.join('\n');
  const patch = createPatch(finding.file, content, newContent, 'original', 'remediated');

  return {
    findingId: finding.id,
    file: finding.file,
    description: `Replace hardcoded secret with process.env.${envVar}`,
    patch,
  };
}

/**
 * Generate a remediation for a missing .gitignore entry: append
 * security-relevant patterns (.env, *.pem, *.key) to the .gitignore file.
 */
export function generateGitignoreFix(
  directory: string,
  finding: Finding,
): RemediationPatch | null {
  const gitignorePath = join(directory, '.gitignore');
  const relativePath = '.gitignore';

  let content: string;
  try {
    content = readFileSync(gitignorePath, 'utf-8');
  } catch {
    // .gitignore doesn't exist — we create one
    content = '';
  }

  const existingLines = new Set(
    content
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean),
  );

  const entriesToAdd = ['.env', '*.pem', '*.key'];
  const missing = entriesToAdd.filter((e) => !existingLines.has(e));

  if (missing.length === 0) return null;

  const newContent = content.endsWith('\n') || content === ''
    ? content + missing.join('\n') + '\n'
    : content + '\n' + missing.join('\n') + '\n';

  const patch = createPatch(relativePath, content, newContent, 'original', 'remediated');

  return {
    findingId: finding.id,
    file: relativePath,
    description: `Add ${missing.join(', ')} to .gitignore`,
    patch,
  };
}

/**
 * Generate a remediation for weak hashing: replace MD5/SHA1 with SHA-256.
 */
export function generateWeakHashFix(
  filePath: string,
  finding: Finding,
): RemediationPatch | null {
  if (!finding.file || finding.line == null) return null;

  let content: string;
  try {
    content = readFileSync(filePath, 'utf-8');
  } catch {
    return null;
  }

  const lines = content.split('\n');
  const lineIdx = finding.line - 1;
  if (lineIdx < 0 || lineIdx >= lines.length) return null;

  const originalLine = lines[lineIdx];

  // Replace createHash('md5') or createHash('sha1') with createHash('sha256')
  const newLine = originalLine.replace(
    /createHash\s*\(\s*['"](?:md5|sha1)['"]\s*\)/gi,
    "createHash('sha256')",
  );

  if (newLine === originalLine) return null;

  const newLines = [...lines];
  newLines[lineIdx] = newLine;

  const newContent = newLines.join('\n');
  const patch = createPatch(finding.file, content, newContent, 'original', 'remediated');

  return {
    findingId: finding.id,
    file: finding.file,
    description: 'Replace weak hash algorithm (MD5/SHA1) with SHA-256',
    patch,
  };
}

/**
 * Generate a remediation for unpinned dependencies: strip ^ and ~ prefixes.
 */
export function generateUnpinnedDepsFix(
  filePath: string,
  finding: Finding,
): RemediationPatch | null {
  if (!finding.file) return null;

  let content: string;
  try {
    content = readFileSync(filePath, 'utf-8');
  } catch {
    return null;
  }

  // Replace version prefixes ^ and ~ in dependency version strings
  const newContent = content.replace(
    /(":\s*")([~^])(\d)/g,
    '$1$3',
  );

  if (newContent === content) return null;

  const patch = createPatch(finding.file, content, newContent, 'original', 'remediated');

  return {
    findingId: finding.id,
    file: finding.file,
    description: 'Pin dependency versions by removing ^ and ~ prefixes',
    patch,
  };
}

// ── Categorisation ─────────────────────────────────────────────────────

type FindingCategory = 'secret' | 'gitignore' | 'weak-hash' | 'unpinned-deps';

/**
 * Determine the remediation category for a finding based on its analyzer and evidence.
 */
export function categoriseFinding(finding: Finding): FindingCategory | null {
  // Secret findings from the secret-scanner
  if (finding.analyzer === 'secret-scanner') return 'secret';

  // .env not in gitignore — from config-checker or git-analyzer
  if (finding.evidence?.includes('.gitignore') || finding.evidence?.includes('gitignore')) {
    return 'gitignore';
  }

  // Weak hash from ast-analyzer (evidence contains 'Weak hash' or ID starts with CRYPTO)
  if (finding.evidence?.toLowerCase().includes('weak hash') ||
      (finding.id?.startsWith('CRYPTO') && finding.evidence?.toLowerCase().includes('createhash'))) {
    return 'weak-hash';
  }

  // Unpinned deps — from config-checker or dep-auditor
  if (finding.evidence?.includes('Unpinned')) return 'unpinned-deps';

  return null;
}

// ── Core fix logic ─────────────────────────────────────────────────────

/**
 * Generate remediation patches for all fixable findings.
 */
export function generatePatches(
  directory: string,
  findings: Finding[],
): RemediationPatch[] {
  const fixable = findings.filter((f) => f.fixable);
  const patches: RemediationPatch[] = [];
  // Track which files have already been patched to avoid duplicate patches
  const patchedFiles = new Set<string>();

  for (const finding of fixable) {
    const category = categoriseFinding(finding);
    if (!category) continue;

    let patch: RemediationPatch | null = null;

    switch (category) {
      case 'secret': {
        if (!finding.file) break;
        const filePath = resolve(directory, finding.file);
        patch = generateSecretFix(filePath, finding);
        break;
      }
      case 'gitignore': {
        if (patchedFiles.has('.gitignore')) break;
        patch = generateGitignoreFix(directory, finding);
        if (patch) patchedFiles.add('.gitignore');
        break;
      }
      case 'weak-hash': {
        if (!finding.file) break;
        const filePath = resolve(directory, finding.file);
        patch = generateWeakHashFix(filePath, finding);
        break;
      }
      case 'unpinned-deps': {
        if (!finding.file) break;
        if (patchedFiles.has(finding.file)) break;
        const filePath = resolve(directory, finding.file);
        patch = generateUnpinnedDepsFix(filePath, finding);
        if (patch) patchedFiles.add(finding.file);
        break;
      }
    }

    if (patch) {
      patches.push(patch);
    }
  }

  return patches;
}

/**
 * Apply generated patches by reading the original file, performing the
 * replacement, and writing the result back. Pure Node.js — no git.
 */
export function applyPatches(
  directory: string,
  patches: RemediationPatch[],
): number {
  let applied = 0;

  // Group patches by file so we apply to the latest version each time
  const byFile = new Map<string, RemediationPatch[]>();
  for (const patch of patches) {
    const existing = byFile.get(patch.file) ?? [];
    existing.push(patch);
    byFile.set(patch.file, existing);
  }

  for (const [file, filePatches] of byFile) {
    const filePath = resolve(directory, file);

    let content: string;
    try {
      content = readFileSync(filePath, 'utf-8');
    } catch {
      // File may not exist yet (e.g. creating .gitignore)
      content = '';
    }

    let newContent = content;
    for (const patch of filePatches) {
      // Apply the patch by matching on the description to determine the fix type
      if (patch.description.includes('process.env.')) {
        // Secret fix: replace quoted strings on the relevant line
        const envVar = patch.description.match(/process\.env\.(\w+)/)?.[1] ?? 'SECRET_VALUE';
        newContent = newContent.replace(
          /(['"`])([^'"`]{4,})\1/,
          `process.env.${envVar}`,
        );
      } else if (patch.description.includes('.gitignore')) {
        // Gitignore fix: append entries
        const entriesToAdd = ['.env', '*.pem', '*.key'];
        const existingLines = new Set(
          newContent.split('\n').map((l) => l.trim()).filter(Boolean),
        );
        const missing = entriesToAdd.filter((e) => !existingLines.has(e));
        if (missing.length > 0) {
          newContent = newContent.endsWith('\n') || newContent === ''
            ? newContent + missing.join('\n') + '\n'
            : newContent + '\n' + missing.join('\n') + '\n';
        }
      } else if (patch.description.includes('SHA-256')) {
        // Weak hash fix
        newContent = newContent.replace(
          /createHash\s*\(\s*['"](?:md5|sha1)['"]\s*\)/gi,
          "createHash('sha256')",
        );
      } else if (patch.description.includes('Pin dependency')) {
        // Unpinned deps fix
        newContent = newContent.replace(
          /(":\s*")([~^])(\d)/g,
          '$1$3',
        );
      }
    }

    if (newContent !== content) {
      writeFileSync(filePath, newContent, 'utf-8');
      applied++;
    }
  }

  return applied;
}

/**
 * Run the full fix pipeline: scan → filter fixable → generate patches → optionally apply.
 */
export async function runFix(
  directory: string,
  dryRun: boolean = true,
  scanResult?: ScanResult,
): Promise<FixResult> {
  // Run scan if no result provided
  const result = scanResult ?? await runScan({ directory });
  const fixable = result.findings.filter((f) => f.fixable);

  // Generate patches
  const patches = generatePatches(directory, result.findings);

  let patchesApplied = 0;
  if (!dryRun && patches.length > 0) {
    patchesApplied = applyPatches(directory, patches);
  }

  return {
    patches,
    applied: !dryRun,
    summary: {
      totalFixable: fixable.length,
      patchesGenerated: patches.length,
      patchesApplied,
    },
  };
}

// ── Tool definition ────────────────────────────────────────────────────

const FixInputSchema = z.object({
  directory: z.string().describe('Absolute path to the directory to scan and fix'),
  dryRun: z
    .boolean()
    .default(true)
    .describe('If true (default), return patches without applying them'),
});

/**
 * Create a ToolDefinition for the auto-remediation fix tool.
 *
 * This tool is gated behind the `pro` tier.
 */
export function createFixTool(): ToolDefinition {
  return {
    name: 'compliance-fix',
    description:
      'Auto-remediate fixable compliance findings by generating and optionally applying patches (requires Soul Pro)',
    product: 'compliance',
    inputSchema: FixInputSchema,
    requiresApproval: true,
    execute: async (input: unknown): Promise<ToolResult> => {
      // Tier gate: free tier cannot use auto-remediation
      requireTier('pro', 'Auto-remediation');

      const parsed = FixInputSchema.parse(input);
      const fixResult = await runFix(parsed.directory, parsed.dryRun);

      const output = formatFixOutput(fixResult);

      return {
        success: true,
        output,
        structured: fixResult as unknown as Record<string, unknown>,
        artifacts: fixResult.patches.map((p) => ({
          type: 'file' as const,
          path: p.file,
          content: p.patch,
        })),
      };
    },
  };
}

/**
 * Format fix result as a human-readable string.
 */
function formatFixOutput(result: FixResult): string {
  const lines: string[] = [];

  lines.push('Auto-Remediation Results');
  lines.push('=======================');
  lines.push(`Total fixable findings: ${result.summary.totalFixable}`);
  lines.push(`Patches generated: ${result.summary.patchesGenerated}`);
  lines.push(`Mode: ${result.applied ? 'applied' : 'dry-run'}`);

  if (result.applied) {
    lines.push(`Files modified: ${result.summary.patchesApplied}`);
  }

  lines.push('');

  if (result.patches.length > 0) {
    lines.push('Patches');
    lines.push('-------');
    for (const patch of result.patches) {
      lines.push(`[${patch.findingId}] ${patch.file}: ${patch.description}`);
      lines.push(patch.patch);
      lines.push('');
    }
  } else {
    lines.push('No auto-fixable findings detected.');
  }

  return lines.join('\n');
}
