// web/src/components/chat/ToolCall.tsx
import { useState, useMemo } from 'react';
import type { ToolCallMessage } from '../../lib/types.ts';
import DiffBlock from './DiffBlock.tsx';

interface ToolCallProps {
  toolCall: ToolCallMessage;
}

// Tool-specific icons (small inline SVGs)
const TOOL_ICONS: Record<string, string> = {
  code_read: '📄', code_write: '✏️', code_edit: '✏️',
  code_search: '🔍', code_grep: '🔍', code_exec: '⚡',
  task_update: '📋', task_create: '📋',
  e2e_assert: '🧪', e2e_dom: '🧪',
};

function toolIcon(name: string): string {
  return TOOL_ICONS[name] || '⚙️';
}

/** Extract structured info from tool input for display */
function toolContext(name: string, input: unknown): string | null {
  if (!input || typeof input !== 'object') return null;
  const inp = input as Record<string, unknown>;

  switch (name) {
    case 'code_read':
    case 'code_write':
    case 'code_edit':
      if (inp.file || inp.path) return String(inp.file || inp.path);
      break;
    case 'code_search':
    case 'code_grep':
      if (inp.query || inp.pattern) return `"${String(inp.query || inp.pattern)}"`;
      break;
    case 'code_exec':
      if (inp.command) {
        const cmd = String(inp.command);
        return cmd.length > 80 ? cmd.slice(0, 77) + '…' : cmd;
      }
      break;
    case 'task_update':
      if (inp.stage) return `→ ${String(inp.stage)}`;
      break;
  }
  return null;
}

function briefSummary(toolCall: ToolCallMessage): string {
  if (toolCall.status === 'running') return 'running…';
  if (toolCall.status === 'error') return 'failed';
  const findings = toolCall.findings?.length ?? 0;
  if (findings > 0) return `${findings} issue${findings !== 1 ? 's' : ''}`;
  if (toolCall.output) {
    const first = toolCall.output.split('\n').find(l => l.trim());
    if (first) return first.length > 60 ? first.slice(0, 57) + '…' : first;
  }
  return 'done';
}

function isDiffOutput(name: string, output: string): boolean {
  if (name !== 'code_edit' && name !== 'code_write') return false;
  return output.includes('\n+') && output.includes('\n-') && (output.includes('@@') || output.includes('---'));
}

function extractImagePath(output: string): string | null {
  const match = output.match(/\/(api\/screenshots\/[^\s]+|[^\s]+\.(png|jpg|jpeg|gif|webp|svg))/i);
  return match ? (match[0].startsWith('/') ? match[0] : '/' + match[0]) : null;
}

export default function ToolCall({ toolCall }: ToolCallProps) {
  const [expanded, setExpanded] = useState(false);
  const isRunning = toolCall.status === 'running';
  const isError = toolCall.status === 'error';

  const statusIcon = isRunning ? '◌' : isError ? '✗' : '✓';
  const statusColor = isRunning
    ? 'text-fg-muted'
    : isError
    ? 'text-stage-blocked'
    : 'text-stage-done';

  const icon = toolIcon(toolCall.name);
  const context = useMemo(() => toolContext(toolCall.name, toolCall.input), [toolCall.name, toolCall.input]);
  const summary = briefSummary(toolCall);
  const hasDetails = !!(toolCall.output || (toolCall.findings?.length ?? 0) > 0);

  // Output stats for collapsed preview
  const outputLines = toolCall.output ? toolCall.output.split('\n').length : 0;
  const isLongOutput = toolCall.output && toolCall.output.length > 500;

  const pillContent = (
    <>
      <span className={`${statusColor} ${isRunning ? 'animate-pulse' : ''} shrink-0`}>
        {statusIcon}
      </span>
      {isRunning && typeof toolCall.progress === 'number' && toolCall.progress > 0 && (
        <div className="w-12 h-1 rounded-full bg-overlay shrink-0 overflow-hidden">
          <div
            className="h-full rounded-full bg-soul transition-all duration-300"
            style={{ width: `${Math.min(toolCall.progress, 100)}%` }}
          />
        </div>
      )}
      <span className="shrink-0">{icon}</span>
      <span className="text-fg-secondary">{toolCall.name}</span>
      {context && (
        <span
          className="text-soul/70 truncate cursor-pointer hover:text-soul hover:underline"
          title={`Click to copy: ${context}`}
          onClick={(e) => {
            e.stopPropagation();
            const path = context.replace(/^"|"$/g, '');
            const fallbackCopy = () => {
              const ta = document.createElement('textarea');
              ta.value = path;
              ta.style.position = 'fixed';
              ta.style.opacity = '0';
              document.body.appendChild(ta);
              ta.select();
              try { document.execCommand('copy'); } catch (_) {}
              document.body.removeChild(ta);
            };
            if (navigator.clipboard?.writeText) {
              navigator.clipboard.writeText(path).catch(fallbackCopy);
            } else {
              fallbackCopy();
            }
          }}
        >
          {context}
        </span>
      )}
      {!context && (
        <>
          <span className="text-fg-muted">·</span>
          <span className="text-fg-muted truncate flex-1">{summary}</span>
        </>
      )}
      {hasDetails && (
        <span className="text-fg-muted shrink-0 text-[10px] ml-auto">
          {isLongOutput && !expanded && `${outputLines}L `}
          {expanded ? '▾' : '▸'}
        </span>
      )}
    </>
  );

  return (
    <div className="font-mono text-xs">
      {hasDetails ? (
        <button
          type="button"
          aria-expanded={expanded}
          onClick={() => setExpanded(!expanded)}
          className="flex items-center gap-1.5 text-left w-full h-7 group cursor-pointer"
        >
          {pillContent}
        </button>
      ) : (
        <div className="flex items-center gap-1.5 w-full h-7">
          {pillContent}
        </div>
      )}

      {expanded && hasDetails && (
        <div className="ml-4 mt-1 mb-1 rounded border border-border-subtle">
          {toolCall.output && (() => {
            const imgSrc = extractImagePath(toolCall.output);
            if (imgSrc) {
              return (
                <div className="p-2">
                  <img src={imgSrc} alt="Tool output" className="max-w-full max-h-60 rounded border border-border-subtle" loading="lazy" />
                </div>
              );
            }
            return null;
          })()}
          {toolCall.output && (
            isDiffOutput(toolCall.name, toolCall.output) ? (
              <DiffBlock content={toolCall.output.length > 3000
                ? toolCall.output.slice(0, 3000) + '\n... (truncated)'
                : toolCall.output} />
            ) : (
              <div className="max-h-60 overflow-y-auto">
                <pre className="p-2 text-fg-muted text-[11px] whitespace-pre-wrap leading-relaxed">
                  {toolCall.output.length > 3000
                    ? toolCall.output.slice(0, 3000) + `\n... (${outputLines} lines total)`
                    : toolCall.output}
                </pre>
              </div>
            )
          )}
          {(toolCall.findings?.length ?? 0) > 0 && (
            <div className="p-2 space-y-0.5">
              {toolCall.findings!.map((f) => (
                <div key={f.id} className="flex items-center gap-2 text-[11px]">
                  <span className={`shrink-0 px-1 rounded text-[9px] uppercase font-medium ${
                    f.severity === 'critical' || f.severity === 'high'
                      ? 'bg-stage-blocked/20 text-stage-blocked'
                      : f.severity === 'medium'
                      ? 'bg-stage-validation/20 text-stage-validation'
                      : 'bg-overlay text-fg-muted'
                  }`}>{f.severity}</span>
                  <span className="text-fg flex-1 truncate">{f.title}</span>
                  {f.file && (
                    <span className="text-fg-muted shrink-0">
                      {f.file}{f.line != null ? `:${f.line}` : ''}
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
