import { useState, useMemo } from 'react';
import type { ToolCallBlockProps } from '../lib/types';
import { DiffBlock } from './DiffBlock';

function ToolIcon({ name, className }: { name: string; className?: string }) {
  const cls = className ?? 'text-fg-muted';
  const props = { width: 10, height: 10, viewBox: '0 0 16 16', fill: 'none', stroke: 'currentColor', strokeWidth: 1.5, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, className: cls };

  switch (name) {
    case 'code_read':
      return <svg {...props}><path d="M3 2h7l3 3v9H3z" /><path d="M10 2v3h3" /></svg>;
    case 'code_write':
    case 'code_edit':
      return <svg {...props}><path d="M11.5 1.5l3 3L5 14H2v-3z" /></svg>;
    case 'code_search':
    case 'code_grep':
      return <svg {...props}><circle cx="7" cy="7" r="4.5" /><path d="M10.5 10.5L14 14" /></svg>;
    case 'code_exec':
      return <svg {...props}><path d="M4 4l4 4-4 4" /><path d="M10 12h4" /></svg>;
    case 'task_update':
    case 'task_create':
      return <svg {...props}><rect x="2" y="2" width="12" height="12" rx="2" /><path d="M5 8h6M8 5v6" /></svg>;
    case 'e2e_assert':
    case 'e2e_dom':
      return <svg {...props}><path d="M3 8l3 3 7-7" /></svg>;
    case 'e2e_screenshot':
      return <svg {...props}><rect x="2" y="3" width="12" height="10" rx="1" /><circle cx="8" cy="8" r="2" /></svg>;
    default:
      return <svg {...props}><circle cx="8" cy="8" r="5.5" /><path d="M8 5v3l2 1" /></svg>;
  }
}

function extractContext(name: string, input: Record<string, unknown>): string | null {
  switch (name) {
    case 'code_read':
    case 'code_write':
    case 'code_edit': {
      const path = input.path ?? input.file_path ?? input.file;
      if (typeof path === 'string') return path;
      break;
    }
    case 'code_grep':
    case 'code_search': {
      const query = input.query ?? input.pattern;
      if (typeof query === 'string') return `"${query}"`;
      break;
    }
    case 'code_exec': {
      const cmd = input.command;
      if (typeof cmd === 'string') return cmd.length > 80 ? cmd.slice(0, 77) + '\u2026' : cmd;
      break;
    }
    case 'task_update': {
      const stage = input.stage ?? input.status;
      if (typeof stage === 'string') return `\u2192 ${stage}`;
      break;
    }
  }
  return null;
}

function briefSummary(tool: { status: string; output?: string }): string {
  if (tool.status === 'running') return 'running\u2026';
  if (tool.status === 'error') return 'failed';
  if (tool.output) {
    const first = tool.output.split('\n').find(l => l.trim());
    if (first) return first.length > 60 ? first.slice(0, 57) + '\u2026' : first;
  }
  return 'done';
}

function isDiffOutput(name: string, output: string): boolean {
  if (name !== 'code_edit' && name !== 'code_write') return false;
  return output.includes('\n+') && output.includes('\n-') && (output.includes('@@') || output.includes('---'));
}

function extractImagePath(output: string): string | null {
  try {
    const parsed = JSON.parse(output);
    if (parsed.path && /\.(png|jpg|jpeg|gif|webp)$/i.test(parsed.path)) {
      return `/api/screenshot?path=${encodeURIComponent(parsed.path)}`;
    }
  } catch { /* not JSON */ }
  const apiMatch = output.match(/\/(api\/screenshots\/[^\s"]+)/i);
  if (apiMatch) return '/' + apiMatch[1];
  const match = output.match(/(\/[^\s"]+\.(?:png|jpg|jpeg|gif|webp))/i);
  return match?.[1] ? `/api/screenshot?path=${encodeURIComponent(match[1])}` : null;
}

const MAX_OUTPUT = 3000;

export function ToolCallBlock({ tool }: ToolCallBlockProps) {
  const [expanded, setExpanded] = useState(false);
  const isRunning = tool.status === 'running';
  const isError = tool.status === 'error';

  const statusColor = isRunning ? 'text-soul' : isError ? 'text-red-500' : 'text-green-500';
  const context = useMemo(() => extractContext(tool.name, tool.input), [tool.name, tool.input]);
  const summary = briefSummary(tool);
  const hasDetails = !!tool.output;
  const outputLines = tool.output ? tool.output.split('\n').length : 0;
  const isLongOutput = tool.output && tool.output.length > 500;

  const handleContextClick = (e: React.MouseEvent) => {
    if (!context) return;
    e.stopPropagation();
    const path = context.replace(/^"|"$/g, '');
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(path).catch(() => {});
    }
  };

  const pillContent = (
    <>
      {isRunning ? (
        <svg width="10" height="10" viewBox="0 0 16 16" fill="none" className="shrink-0 animate-spin">
          <circle cx="8" cy="8" r="5.5" stroke="currentColor" strokeWidth="1.5" strokeDasharray="7 4" className="text-soul" />
        </svg>
      ) : isError ? (
        <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="shrink-0 text-red-500">
          <path d="M4 4l8 8M12 4l-8 8" />
        </svg>
      ) : (
        <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="shrink-0 text-green-500">
          <path d="M3 8l3 3 7-7" />
        </svg>
      )}
      {isRunning && typeof tool.progress === 'number' && tool.progress > 0 && (
        <div className="w-12 h-1 rounded-full bg-overlay shrink-0 overflow-hidden">
          <div
            className="h-full rounded-full bg-soul transition-all duration-300"
            style={{ width: `${Math.min(tool.progress, 100)}%` }}
          />
        </div>
      )}
      <ToolIcon name={tool.name} className={`shrink-0 ${statusColor}`} />
      <span className="text-fg-secondary truncate max-w-[120px] sm:max-w-none">{tool.name}</span>
      {context && (
        <span
          className="text-soul/70 truncate cursor-pointer hover:text-soul hover:underline"
          title={`Click to copy: ${context}`}
          onClick={handleContextClick}
        >
          {context}
        </span>
      )}
      {!context && (
        <>
          <span className="text-fg-muted">{'\u00B7'}</span>
          <span className="text-fg-muted truncate flex-1">{summary}</span>
        </>
      )}
      {hasDetails && (
        <span className="text-fg-muted shrink-0 text-[10px] ml-auto">
          {isLongOutput && !expanded && `${outputLines}L `}
          {expanded ? '\u25BE' : '\u25B8'}
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
          data-testid="tool-call-block"
          className="flex items-center gap-1.5 text-left w-full h-8 sm:h-7 group cursor-pointer"
        >
          {pillContent}
        </button>
      ) : (
        <div data-testid="tool-call-block" className="flex items-center gap-1.5 w-full h-8 sm:h-7">
          {pillContent}
        </div>
      )}

      {expanded && hasDetails && (
        <div className="ml-4 mt-1 mb-1 rounded border border-border-subtle">
          {tool.output && (() => {
            const imgSrc = extractImagePath(tool.output!);
            if (imgSrc) {
              return (
                <div className="p-2">
                  <img src={imgSrc} alt="Screenshot" className="max-w-full rounded border border-border-subtle" loading="lazy" />
                </div>
              );
            }
            return null;
          })()}
          {tool.output && (
            isDiffOutput(tool.name, tool.output) ? (
              <DiffBlock content={tool.output.length > MAX_OUTPUT
                ? tool.output.slice(0, MAX_OUTPUT) + '\n... (truncated)'
                : tool.output} />
            ) : (
              <div className="max-h-60 overflow-y-auto">
                <pre className="p-2 text-fg-muted text-xs sm:text-[11px] whitespace-pre-wrap leading-relaxed">
                  {tool.output.length > MAX_OUTPUT
                    ? tool.output.slice(0, MAX_OUTPUT) + `\n... (${outputLines} lines total)`
                    : tool.output}
                </pre>
              </div>
            )
          )}
        </div>
      )}
    </div>
  );
}
