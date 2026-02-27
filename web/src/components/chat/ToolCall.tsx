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
      ? 'text-sky-400'
      : toolCall.status === 'complete'
        ? 'text-emerald-400'
        : 'text-red-400';

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
    <div className="my-2 border border-zinc-700 rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-2 bg-zinc-800/50 hover:bg-zinc-800 transition-colors text-left"
      >
        <span className={`${statusColor} ${toolCall.status === 'running' ? 'animate-spin' : ''}`}>
          {statusIcon}
        </span>
        <span className="text-sm font-mono text-zinc-300">{toolCall.name}</span>
        <span className="text-xs text-zinc-500 ml-auto">{summary}</span>
        <span className="text-zinc-500 text-xs">{expanded ? '\u25B2' : '\u25BC'}</span>
      </button>

      {toolCall.status === 'running' && toolCall.progress != null && toolCall.progress > 0 && (
        <div className="px-3 pb-2 bg-zinc-800/30">
          <div className="h-1 bg-zinc-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-sky-500 rounded-full transition-all duration-300"
              style={{ width: `${toolCall.progress}%` }}
            />
          </div>
        </div>
      )}

      {expanded && (
        <div className="px-3 py-2 bg-zinc-900/50 text-sm">
          {toolCall.output && (
            <pre className="text-zinc-400 whitespace-pre-wrap text-xs mb-2">
              {toolCall.output}
            </pre>
          )}
          {findingsCount > 0 && (
            <div className="space-y-1">
              {toolCall.findings!.map((f) => (
                <div
                  key={f.id}
                  className="flex items-center gap-2 text-xs text-zinc-400"
                >
                  <SeverityBadge severity={f.severity} />
                  <span className="text-zinc-300">{f.title}</span>
                  {f.file && (
                    <span className="text-zinc-600 ml-auto font-mono">
                      {f.file}
                      {f.line != null ? `:${f.line}` : ''}
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}
          {!toolCall.output && findingsCount === 0 && (
            <span className="text-zinc-500 text-xs">No details available</span>
          )}
        </div>
      )}
    </div>
  );
}

function SeverityBadge({ severity }: { severity: string }) {
  const colors: Record<string, string> = {
    critical: 'bg-red-500/20 text-red-400',
    high: 'bg-red-500/20 text-red-400',
    medium: 'bg-amber-500/20 text-amber-400',
    low: 'bg-sky-500/20 text-sky-400',
    info: 'bg-zinc-500/20 text-zinc-400',
  };

  const cls = colors[severity.toLowerCase()] ?? colors.info;

  return (
    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium uppercase ${cls}`}>
      {severity}
    </span>
  );
}
