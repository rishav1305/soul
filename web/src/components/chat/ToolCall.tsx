import { useState } from 'react';
import type { ToolCallMessage } from '../../lib/types.ts';

interface ToolCallProps {
  toolCall: ToolCallMessage;
}

export default function ToolCall({ toolCall }: ToolCallProps) {
  const [expanded, setExpanded] = useState(false);

  const statusIcon =
    toolCall.status === 'running'
      ? '\u25F3' // spinning clock
      : toolCall.status === 'complete'
        ? '\u2713' // checkmark
        : '\u2717'; // X

  const statusColor =
    toolCall.status === 'running'
      ? 'text-stage-active'
      : toolCall.status === 'complete'
        ? 'text-stage-done'
        : 'text-stage-blocked';

  const findingsCount = toolCall.findings?.length ?? 0;
  const summary =
    toolCall.status === 'complete' && findingsCount > 0
      ? `Found ${findingsCount} issue${findingsCount !== 1 ? 's' : ''}`
      : toolCall.status === 'complete'
        ? 'Complete'
        : toolCall.status === 'error'
          ? 'Failed'
          : 'Running...';

  return (
    <div className="my-2 border border-border-default rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-2 bg-elevated/60 hover:bg-elevated transition-colors text-left"
      >
        <span className={`${statusColor} ${toolCall.status === 'running' ? 'animate-spin' : ''}`}>
          {statusIcon}
        </span>
        <span className="font-mono text-sm text-fg-secondary">{toolCall.name}</span>
        <span className="text-xs text-fg-muted ml-auto">{summary}</span>
        <span className="text-fg-muted text-xs">{expanded ? '\u25B2' : '\u25BC'}</span>
      </button>

      {toolCall.status === 'running' && toolCall.progress != null && toolCall.progress > 0 && (
        <div className="px-3 pb-2 bg-elevated/60">
          <div className="h-1 bg-overlay rounded-full overflow-hidden">
            <div
              className="h-full bg-soul rounded-full transition-all duration-300"
              style={{ width: `${toolCall.progress}%` }}
            />
          </div>
        </div>
      )}

      {expanded && (
        <div className="px-3 py-2 bg-surface/60 text-sm">
          {toolCall.output && (
            <pre className="text-fg-muted font-mono whitespace-pre-wrap text-xs mb-2">
              {toolCall.output}
            </pre>
          )}
          {findingsCount > 0 && (
            <div className="space-y-1">
              {toolCall.findings!.map((f) => (
                <div
                  key={f.id}
                  className="flex items-center gap-2 text-xs text-fg-muted"
                >
                  <SeverityBadge severity={f.severity} />
                  <span className="text-fg">{f.title}</span>
                  {f.file && (
                    <span className="text-fg-muted ml-auto font-mono">
                      {f.file}
                      {f.line != null ? `:${f.line}` : ''}
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}
          {!toolCall.output && findingsCount === 0 && (
            <span className="text-fg-muted text-xs">No details available</span>
          )}
        </div>
      )}
    </div>
  );
}

function SeverityBadge({ severity }: { severity: string }) {
  const colors: Record<string, string> = {
    critical: 'bg-stage-blocked/20 text-stage-blocked',
    high: 'bg-stage-blocked/20 text-stage-blocked',
    medium: 'bg-stage-validation/20 text-stage-validation',
    low: 'bg-stage-active/20 text-stage-active',
    info: 'bg-overlay text-fg-muted',
  };

  const cls = colors[severity.toLowerCase()] ?? colors.info;

  return (
    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium uppercase ${cls}`}>
      {severity}
    </span>
  );
}
