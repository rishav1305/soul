import { useState, useCallback, useMemo, useRef, useEffect } from 'react';
import type { MessageBubbleProps } from '../lib/types';
import { formatRelativeTime, formatTokens } from '../lib/utils';
import { usePerformance } from '../hooks/usePerformance';
import { Markdown } from './Markdown';
import { ThinkingBlock } from './ThinkingBlock';
import { ToolCallBlock } from './ToolCallBlock';
import type { ToolCallData } from '../lib/types';

interface ToolUseContent {
  name: string;
  input: Record<string, unknown>;
}

function parseToolUse(content: string): ToolUseContent | null {
  try {
    const parsed: unknown = JSON.parse(content);
    if (
      typeof parsed === 'object' &&
      parsed !== null &&
      'name' in parsed &&
      'input' in parsed
    ) {
      const obj = parsed as { name: unknown; input: unknown };
      if (typeof obj.name === 'string' && typeof obj.input === 'object' && obj.input !== null) {
        return { name: obj.name, input: obj.input as Record<string, unknown> };
      }
    }
  } catch { /* not valid JSON */ }
  return null;
}

function legacyToolCallData(toolData: ToolUseContent): ToolCallData {
  return {
    id: `legacy-${toolData.name}-${Date.now()}`,
    name: toolData.name,
    input: toolData.input,
    status: 'running',
  };
}

function modelLabel(model: string): string {
  if (model.includes('opus')) return 'Opus';
  if (model.includes('sonnet')) return 'Sonnet';
  if (model.includes('haiku')) return 'Haiku';
  return model.split('-')[0] ?? model;
}

// --- Compact sub-components ---

function CopyBtn({ content }: { content: string }) {
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const handleCopy = useCallback(() => {
    const fallback = () => {
      const ta = document.createElement('textarea');
      ta.value = content;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    };
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(content).catch(fallback);
    } else {
      fallback();
    }
    setCopied(true);
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => setCopied(false), 1500);
  }, [content]);

  return (
    <button
      type="button"
      onClick={handleCopy}
      data-testid="copy-message-btn"
      className="opacity-0 group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
      title="Copy message"
    >
      {copied ? (
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-green-500">
          <path d="M3 8l3 3 7-7" />
        </svg>
      ) : (
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <rect x="5" y="5" width="9" height="9" rx="1.5" />
          <path d="M3 11V3a1.5 1.5 0 0 1 1.5-1.5H11" />
        </svg>
      )}
    </button>
  );
}

function RetryBtn({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      data-testid="retry-message-btn"
      className="opacity-0 group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
      title="Retry"
    >
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M1 4v5h5" /><path d="M3.5 11A6 6 0 1 0 3 5l-2 2" />
      </svg>
    </button>
  );
}

function EditBtn({ content, onEdit }: { content: string; onEdit: (text: string) => void }) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(content);

  if (editing) {
    return (
      <div className="mt-2 w-full">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          data-testid="edit-message-textarea"
          className="w-full bg-elevated border border-border-subtle rounded-lg px-3 py-2 text-sm text-fg resize-none focus:outline-none focus:border-soul/40"
          rows={3}
          autoFocus
        />
        <div className="flex gap-2 mt-1">
          <button type="button" onClick={() => { onEdit(text); setEditing(false); }}
            data-testid="edit-submit-btn"
            className="text-xs text-soul hover:underline cursor-pointer">Submit</button>
          <button type="button" onClick={() => { setText(content); setEditing(false); }}
            data-testid="edit-cancel-btn"
            className="text-xs text-fg-muted hover:text-fg cursor-pointer">Cancel</button>
        </div>
      </div>
    );
  }

  return (
    <button
      type="button"
      onClick={() => setEditing(true)}
      data-testid="edit-message-btn"
      className="opacity-0 group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
      title="Edit and resend"
    >
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M11.5 1.5l3 3L5 14H2v-3L11.5 1.5z" />
      </svg>
    </button>
  );
}

function ToolCallGroup({ toolCalls }: { toolCalls: ToolCallData[] }) {
  const [groupExpanded, setGroupExpanded] = useState(false);
  const allDone = toolCalls.every(tc => tc.status !== 'running');
  const runningCount = toolCalls.filter(tc => tc.status === 'running').length;
  const errorCount = toolCalls.filter(tc => tc.status === 'error').length;

  if (toolCalls.length === 1) {
    return (
      <div className="mt-2 pl-1 border-l border-border-subtle">
        <ToolCallBlock tool={toolCalls[0]} />
      </div>
    );
  }

  const groupLabel = !allDone
    ? `${runningCount} running\u2026`
    : errorCount > 0
    ? `${toolCalls.length} calls (${errorCount} failed)`
    : `${toolCalls.length} tool calls`;

  return (
    <div className="mt-2 pl-1 border-l border-border-subtle">
      <button
        type="button"
        onClick={() => setGroupExpanded(!groupExpanded)}
        className="flex items-center gap-1.5 text-xs text-fg-muted hover:text-fg transition-colors cursor-pointer py-0.5 font-mono"
      >
        <span className={!allDone ? 'animate-pulse text-fg-muted' : errorCount > 0 ? 'text-red-500' : 'text-green-500'}>
          {!allDone ? '\u25CC' : errorCount > 0 ? '\u2717' : '\u2713'}
        </span>
        <span>{groupLabel}</span>
        <span className="text-[10px]">{groupExpanded ? '\u25BE' : '\u25B8'}</span>
      </button>
      {groupExpanded && (
        <div className="ml-3 mt-0.5 space-y-0.5">
          {toolCalls.map(tc => <ToolCallBlock key={tc.id} tool={tc} />)}
        </div>
      )}
    </div>
  );
}

export function MessageBubble({ message, isStreaming, onEdit, onRetry, searchQuery }: MessageBubbleProps) {
  usePerformance('MessageBubble');

  const isUser = message.role === 'user';
  const isToolUse = message.role === 'tool_use';
  const isToolResult = message.role === 'tool_result';

  const timeStr = useMemo(() => formatRelativeTime(message.createdAt), [message.createdAt]);
  const badge = message.model && !isUser ? modelLabel(message.model) : null;

  // Legacy tool_use messages — render as compact tool call
  if (isToolUse) {
    const toolData = parseToolUse(message.content);
    if (toolData) {
      return (
        <div data-testid="message-bubble" className="flex flex-col items-start animate-fade-in ml-[34px]">
          <div className="pl-1 border-l border-border-subtle w-full max-w-[85%]">
            <ToolCallBlock tool={legacyToolCallData(toolData)} />
          </div>
        </div>
      );
    }
  }

  // Legacy tool_result messages
  if (isToolResult) {
    return (
      <div data-testid="message-bubble" className="flex flex-col items-start animate-fade-in ml-[34px]">
        <div className="pl-1 border-l border-border-subtle">
          <div className="text-xs font-mono text-fg-muted flex items-center gap-1.5 h-7">
            <span className="text-green-500">{'\u2713'}</span>
            <span className="text-fg-secondary">result</span>
            <span className="text-fg-muted">{'\u00B7'}</span>
            <span className="text-fg-muted truncate">{message.content ? (message.content.length > 60 ? message.content.slice(0, 57) + '\u2026' : message.content.split('\n')[0]) : 'No output'}</span>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div
      data-testid="message-bubble"
      className={`group flex flex-col ${isUser ? 'items-end' : 'items-start'} animate-fade-in`}
    >
      <div className={`flex gap-2.5 ${isUser ? 'flex-row-reverse' : 'flex-row w-full'} max-w-[85%]`}>
        {/* Role avatar */}
        <div className="shrink-0 mt-3">
          {isUser ? (
            <div className="w-6 h-6 rounded-full bg-elevated flex items-center justify-center">
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-fg-muted">
                <circle cx="8" cy="5" r="3" />
                <path d="M2 14c0-3.3 2.7-5 6-5s6 1.7 6 5" />
              </svg>
            </div>
          ) : (
            <div className={`w-6 h-6 rounded-full bg-soul/15 flex items-center justify-center ${isStreaming ? 'diamond-pulse diamond-breathe' : ''}`}>
              <svg width="10" height="10" viewBox="0 0 16 16" fill="var(--color-soul)">
                <path d="M8 0L14 8L8 16L2 8Z" />
              </svg>
            </div>
          )}
        </div>

        {/* Message content */}
        <div className={`min-w-0 px-4 py-3 ${
          isUser
            ? 'bg-elevated border-l-2 border-soul/40 text-fg rounded-2xl rounded-br-md'
            : 'text-fg rounded-2xl rounded-bl-md'
        }`}>
          {/* Thinking block above text for assistant */}
          {!isUser && message.thinking && (
            <ThinkingBlock content={message.thinking} isStreaming={isStreaming && !message.content} />
          )}

          {/* Text content */}
          {message.content && (
            isUser ? (
              <p className="whitespace-pre-wrap break-words text-fg">
                {searchQuery ? <HighlightText text={message.content} query={searchQuery} /> : message.content}
              </p>
            ) : (
              <Markdown content={message.content} />
            )
          )}

          {/* Streaming indicator */}
          {isStreaming && !isUser && (
            message.content ? (
              <div className="flex items-center gap-2 mt-1.5">
                <span className="w-1.5 h-1.5 rounded-full bg-soul animate-pulse" />
                <span className="text-[10px] font-mono text-fg-muted">
                  {message.content.split(/\s+/).filter(Boolean).length} word{message.content.split(/\s+/).filter(Boolean).length !== 1 ? 's' : ''}
                </span>
              </div>
            ) : !message.thinking ? (
              <div className="flex items-center gap-2 py-1">
                <span className="text-sm text-fg-muted animate-pulse">Soul is thinking...</span>
              </div>
            ) : null
          )}

          {/* Tool calls below text */}
          {message.toolCalls && message.toolCalls.length > 0 && (
            <ToolCallGroup toolCalls={message.toolCalls} />
          )}

        </div>
      </div>

      {/* Metadata footer */}
      <div className={`flex items-center gap-2 mt-1.5 text-[10px] text-fg-muted ${isUser ? 'mr-[34px]' : 'ml-[34px]'}`}>
        <span>{timeStr}</span>
        {badge && (
          <span className="px-1.5 py-0.5 rounded bg-soul/10 text-soul font-mono">{badge}</span>
        )}
        {message.usage && !isUser && (
          <span className="text-fg-muted">
            {formatTokens(message.usage.inputTokens)} in {'\u00B7'} {formatTokens(message.usage.outputTokens)} out
          </span>
        )}
        {message.content && <CopyBtn content={message.content} />}
        {isUser && onEdit && (
          <EditBtn content={message.content} onEdit={(newText) => onEdit(message.id, newText)} />
        )}
        {isUser && onRetry && !isStreaming && (
          <RetryBtn onClick={() => onRetry(message.id)} />
        )}
      </div>
    </div>
  );
}

function HighlightText({ text, query }: { text: string; query?: string }) {
  const parts = useMemo(() => {
    if (!query) return null;
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    return text.split(new RegExp(`(${escaped})`, 'gi'));
  }, [text, query]);

  if (!parts) return <>{text}</>;
  return (
    <>
      {parts.map((part, i) =>
        part.toLowerCase() === query!.toLowerCase() ? (
          <mark key={i} className="bg-soul/30 text-inherit rounded px-0.5">{part}</mark>
        ) : (
          part
        ),
      )}
    </>
  );
}
