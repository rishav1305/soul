// web/src/components/chat/ToolCall.tsx
import { useState } from 'react';
import type { ToolCallMessage } from '../../lib/types.ts';

interface ToolCallProps {
  toolCall: ToolCallMessage;
}

function briefSummary(toolCall: ToolCallMessage): string {
  if (toolCall.status === 'running') return 'running…';
  if (toolCall.status === 'error') return 'failed';
  const findings = toolCall.findings?.length ?? 0;
  if (findings > 0) return `${findings} issue${findings !== 1 ? 's' : ''}`;
  // Extract a short summary from output (first non-empty line, max 60 chars)
  if (toolCall.output) {
    const first = toolCall.output.split('\n').find(l => l.trim());
    if (first) return first.length > 60 ? first.slice(0, 57) + '…' : first;
  }
  return 'done';
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

  const summary = briefSummary(toolCall);
  const hasDetails = !!(toolCall.output || (toolCall.findings?.length ?? 0) > 0);

  return (
    <div className="font-mono text-xs">
      <button
        type="button"
        onClick={() => hasDetails && setExpanded(!expanded)}
        className={`flex items-center gap-1.5 text-left w-full group py-0.5 h-7 ${hasDetails ? 'cursor-pointer' : 'cursor-default'}`}
      >
        <span className={`${statusColor} ${isRunning ? 'animate-pulse' : ''} shrink-0`}>
          {statusIcon}
        </span>
        <span className="text-fg-secondary">{toolCall.name}</span>
        <span className="text-fg-muted">·</span>
        <span className="text-fg-muted truncate flex-1">{summary}</span>
        {hasDetails && (
          <span className="text-fg-muted shrink-0 text-[10px]">
            {expanded ? '▾' : '▸'}
          </span>
        )}
      </button>

      {expanded && hasDetails && (
        <div className="ml-4 mt-1 mb-1 max-h-60 overflow-y-auto rounded border border-border-subtle">
          {toolCall.output && (
            <pre className="p-2 text-fg-muted text-[11px] whitespace-pre-wrap leading-relaxed">
              {toolCall.output.length > 2000
                ? toolCall.output.slice(0, 2000) + '\n… (truncated)'
                : toolCall.output}
            </pre>
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
