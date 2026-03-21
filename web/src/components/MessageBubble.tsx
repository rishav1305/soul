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
      className="sm:opacity-0 sm:group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
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
      className="sm:opacity-0 sm:group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
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
      className="sm:opacity-0 sm:group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
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

  // Extract chat mode prefix from user messages (e.g. "/brainstorm ..." → mode="Brainstorm", displayContent without prefix)
  const chatModeLabel = useMemo(() => {
    if (!isUser || !message.content) return null;
    const match = message.content.match(/^\/(code|architect|brainstorm)\s/i);
    return match ? match[1].charAt(0).toUpperCase() + match[1].slice(1) : null;
  }, [isUser, message.content]);
  const displayContent = useMemo(() => {
    if (!chatModeLabel || !message.content) return message.content;
    return message.content.replace(/^\/\w+\s/, '');
  }, [chatModeLabel, message.content]);

  // Legacy tool_use messages — render as compact tool call
  if (isToolUse) {
    const toolData = parseToolUse(message.content);
    if (toolData) {
      return (
        <div data-testid="message-bubble" className="flex flex-col items-start animate-fade-in ml-2 sm:ml-[34px]">
          <div className="pl-1 border-l border-border-subtle w-full max-w-full sm:max-w-[85%]">
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

  // Mobile: assistant messages render without bubble/avatar, just content + footer
  // Desktop: full bubble with avatar header
  return (
    <div
      data-testid="message-bubble"
      className={`group flex flex-col ${isUser ? 'items-end' : 'items-start'} animate-fade-in`}
    >
      {/* Assistant header — desktop only: golden diamond + model */}
      {!isUser && (
        <div className="hidden sm:flex items-center gap-1.5 mb-1">
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none"
            className={isStreaming ? 'drop-shadow-[0_0_6px_var(--color-soul-glow)]' : ''}>
            <defs>
              <linearGradient id="diamond-gold" x1="2" y1="0" x2="14" y2="16">
                <stop offset="0%" stopColor="#f0c040" />
                <stop offset="50%" stopColor="#d4a018" />
                <stop offset="100%" stopColor="#b8860b" />
              </linearGradient>
            </defs>
            <path d="M8 0L14 8L8 16L2 8Z" fill="url(#diamond-gold)">
              {isStreaming && <animate attributeName="opacity" values="0.7;1;0.7" dur="2s" repeatCount="indefinite" />}
            </path>
          </svg>
          {badge && <span className="text-[9px] font-mono text-fg-muted">{badge}</span>}
          {message.usage && (
            <>
              <span className="text-[9px] text-fg-muted/50">{'\u00B7'}</span>
              <span className="text-[9px] font-mono text-fg-muted/70">
                {formatTokens(message.usage.inputTokens)} {'\u2192'} {formatTokens(message.usage.outputTokens)}
              </span>
            </>
          )}
        </div>
      )}

      {/* Message content */}
      <div className={`relative min-w-0 max-w-full sm:max-w-[85%] ${
        isUser
          ? `bg-[#12123a] border border-[#1e1e50] px-2 ${chatModeLabel ? 'pt-4' : 'py-1.5'} pb-1.5 sm:px-4 sm:py-3 ${chatModeLabel ? 'sm:pt-4' : ''} text-fg text-[12px] sm:text-base rounded-[16px_16px_4px_16px]`
          : 'px-1 py-0 sm:bg-surface sm:border sm:border-border-subtle sm:px-4 sm:py-3 text-fg text-[12px] sm:text-base sm:rounded-[4px_14px_14px_14px]'
      }`}>
        {/* Chat mode label — sits on the border like a fieldset legend */}
        {isUser && chatModeLabel && (
          <span className="absolute -top-2.5 left-3 px-2 py-0.5 text-[10px] font-mono font-medium uppercase tracking-wider bg-[#12123a] text-[#7c6aed] border border-[#1e1e50] rounded-md">
            {chatModeLabel}
          </span>
        )}

        {/* Thinking block above text for assistant */}
        {!isUser && message.thinking && (
          <ThinkingBlock content={message.thinking} isStreaming={isStreaming && !message.content} />
        )}

        {/* Text content */}
        {message.content && (
          isUser ? (
            <p className="whitespace-pre-wrap break-words text-fg leading-relaxed">
              {searchQuery ? <HighlightText text={displayContent} query={searchQuery} /> : displayContent}
            </p>
          ) : (
            <div className="leading-[1.15] sm:leading-[1.7]">
              <Markdown content={message.content} />
            </div>
          )
        )}

        {/* Streaming indicator */}
        {isStreaming && !isUser && (
          message.content ? (
            <div className="flex items-center gap-2 mt-1.5">
              <span className="streaming-cursor" />
              <span className="text-[10px] font-mono text-fg-muted">
                {message.content.split(/\s+/).filter(Boolean).length} words
              </span>
            </div>
          ) : !message.thinking ? (
            <div className="flex items-center gap-2 py-1">
              <span className="streaming-cursor" />
            </div>
          ) : null
        )}

        {/* Tool calls below text */}
        {message.toolCalls && message.toolCalls.length > 0 && (
          <ToolCallGroup toolCalls={message.toolCalls} />
        )}

      </div>

      {/* Footer */}
      <div className={`flex items-center gap-2 mt-1 sm:mt-1.5 text-[10px] text-fg-muted`}>
        {/* User: timestamp + edit/retry */}
        {isUser && <span>{timeStr}</span>}
        {/* Assistant: model + tokens (serves as "response complete" signal) */}
        {!isUser && !isStreaming && (
          <>
            {badge && <span className="font-mono text-fg-muted/70">{badge}</span>}
            {message.usage && (
              <>
                <span className="text-fg-muted/40">{'\u00B7'}</span>
                <span className="font-mono text-fg-muted/60">
                  {formatTokens(message.usage.inputTokens)} {'\u2192'} {formatTokens(message.usage.outputTokens)}
                </span>
              </>
            )}
          </>
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
