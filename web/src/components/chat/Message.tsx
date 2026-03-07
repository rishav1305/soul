// web/src/components/chat/Message.tsx
import React, { useState, useMemo, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
// Fix missing spaces where sentences run together (e.g. "restart.Good" → "restart. Good")
function fixMissingSpaces(text: string): string {
  return text.replace(/([.!?:;,])([A-Z])/g, '$1 $2');
}
import type { ChatMessage, ToolCallMessage } from '../../lib/types.ts';
import ToolCall from './ToolCall.tsx';
import CodeBlock from './CodeBlock.tsx';
import MermaidBlock from './MermaidBlock.tsx';

function formatRelativeTime(date: Date): string {
  const now = Date.now();
  const diff = now - date.getTime();
  if (diff < 60_000) return 'just now';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function modelLabel(model?: string): string | null {
  if (!model) return null;
  const m = model.toLowerCase();
  if (m.includes('opus')) return 'Opus';
  if (m.includes('sonnet')) return 'Sonnet';
  if (m.includes('haiku')) return 'Haiku';
  return model.split('-')[0]; // fallback: first segment
}

// Custom renderers for ReactMarkdown
const markdownComponents = {
  code({ className, children, ...props }: any) {
    const match = /language-(\w+)/.exec(className || '');
    const lang = match?.[1];
    const isInline = !match && !className;
    const text = String(children).replace(/\n$/, '');
    if (isInline) {
      return <CodeBlock inline>{text}</CodeBlock>;
    }
    if (lang === 'mermaid') {
      return <MermaidBlock>{text}</MermaidBlock>;
    }
    return <CodeBlock language={lang}>{text}</CodeBlock>;
  },
  pre({ children }: any) {
    return <>{children}</>;
  },
  img({ src, alt, ...props }: any) {
    if (!src) return null;
    return (
      <div className="my-3 rounded-lg overflow-hidden border border-border-subtle inline-block max-w-full">
        <img src={src} alt={alt || ''} className="max-w-full max-h-96 object-contain" loading="lazy" {...props} />
        {alt && <div className="px-2 py-1 text-[10px] text-fg-muted bg-elevated/60">{alt}</div>}
      </div>
    );
  },
};

interface MessageProps {
  message: ChatMessage;
  onRetry?: (messageId: string) => void;
  onEdit?: (messageId: string, newContent: string) => void;
  isStreaming?: boolean;
  searchQuery?: string;
}

function ThinkingBlock({ content }: { content: string }) {
  const [expanded, setExpanded] = useState(false);
  const lines = content.split('\n').length;
  return (
    <div className="mb-3">
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-1.5 text-xs text-fg-muted hover:text-fg transition-colors cursor-pointer font-mono"
      >
        <span>💭</span>
        <span>Thinking ({lines} lines)</span>
        <span className="text-[10px]">{expanded ? '▾' : '▸'}</span>
      </button>
      {expanded && (
        <div className="mt-1 p-3 rounded border border-border-subtle bg-elevated/40 max-h-48 overflow-y-auto">
          <pre className="text-xs text-fg-muted font-mono whitespace-pre-wrap leading-relaxed">
            {content}
          </pre>
        </div>
      )}
    </div>
  );
}

function ToolCallGroup({ toolCalls }: { toolCalls: ToolCallMessage[] }) {
  const [groupExpanded, setGroupExpanded] = useState(false);
  const allDone = toolCalls.every(tc => tc.status !== 'running');
  const runningCount = toolCalls.filter(tc => tc.status === 'running').length;
  const errorCount = toolCalls.filter(tc => tc.status === 'error').length;

  if (toolCalls.length === 1) {
    return (
      <div className="mt-2 pl-1 border-l border-border-subtle">
        <ToolCall toolCall={toolCalls[0]} />
      </div>
    );
  }

  const groupLabel = !allDone
    ? `${runningCount} running…`
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
        <span className={!allDone ? 'animate-pulse text-fg-muted' : errorCount > 0 ? 'text-stage-blocked' : 'text-stage-done'}>
          {!allDone ? '◌' : errorCount > 0 ? '✗' : '✓'}
        </span>
        <span>{groupLabel}</span>
        <span className="text-[10px]">{groupExpanded ? '▾' : '▸'}</span>
      </button>
      {groupExpanded && (
        <div className="ml-3 mt-0.5 space-y-0.5">
          {toolCalls.map(tc => <ToolCall key={tc.id} toolCall={tc} />)}
        </div>
      )}
    </div>
  );
}

function CopyMessageBtn({ content }: { content: string }) {
  const [copied, setCopied] = useState(false);
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
    setTimeout(() => setCopied(false), 1500);
  }, [content]);

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="opacity-0 group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
      title="Copy message"
    >
      {copied ? (
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="var(--color-stage-done)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
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
          className="w-full bg-elevated border border-border-subtle rounded-lg px-3 py-2 text-sm text-fg resize-none focus:outline-none focus:border-soul/40"
          rows={3}
          autoFocus
        />
        <div className="flex gap-2 mt-1">
          <button type="button" onClick={() => { onEdit(text); setEditing(false); }}
            className="text-xs text-soul hover:underline cursor-pointer">Submit</button>
          <button type="button" onClick={() => { setText(content); setEditing(false); }}
            className="text-xs text-fg-muted hover:text-fg cursor-pointer">Cancel</button>
        </div>
      </div>
    );
  }

  return (
    <button
      type="button"
      onClick={() => setEditing(true)}
      className="opacity-0 group-hover:opacity-100 transition-opacity text-fg-muted hover:text-fg cursor-pointer"
      title="Edit and resend"
    >
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M11.5 1.5l3 3L5 14H2v-3L11.5 1.5z" />
      </svg>
    </button>
  );
}

function MessageMeta({ message, onRetry, onEdit, isStreaming }: {
  message: ChatMessage;
  onRetry?: (id: string) => void;
  onEdit?: (id: string, text: string) => void;
  isStreaming?: boolean;
}) {
  const timeStr = useMemo(() => formatRelativeTime(message.timestamp), [message.timestamp]);
  const badge = modelLabel(message.model);
  const isUser = message.role === 'user';

  return (
    <div className={`flex items-center gap-2 mt-1.5 text-[10px] text-fg-muted ${isUser ? 'justify-end' : 'justify-start'}`}>
      <span>{timeStr}</span>
      {badge && !isUser && (
        <span className="px-1.5 py-0.5 rounded bg-soul/10 text-soul font-mono">{badge}</span>
      )}
      {message.content && <CopyMessageBtn content={message.content} />}
      {isUser && onRetry && !isStreaming && (
        <RetryBtn onClick={() => onRetry(message.id)} />
      )}
      {isUser && onEdit && !isStreaming && (
        <EditBtn content={message.content} onEdit={(newText) => onEdit(message.id, newText)} />
      )}
    </div>
  );
}

function highlightText(text: string, query: string): React.ReactNode {
  if (!query.trim()) return text;
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const parts = text.split(new RegExp(`(${escaped})`, 'gi'));
  return parts.map((part, i) =>
    part.toLowerCase() === query.toLowerCase()
      ? <mark key={i} className="bg-soul/30 text-fg rounded px-0.5">{part}</mark>
      : part
  );
}

export default function Message({ message, onRetry, onEdit, isStreaming, searchQuery }: MessageProps) {
  const isUser = message.role === 'user';

  // PM notification — render as a styled card instead of a normal bubble
  if (message.pmNotification) {
    const severityStyles = {
      error: 'border-l-2 border-red-500 bg-red-500/5',
      warning: 'border-l-2 border-amber-500 bg-amber-500/5',
      info: 'border-l-2 border-zinc-500 bg-zinc-500/5',
    };
    return (
      <div className={`rounded-lg px-3 py-2 my-1 text-xs font-body ${severityStyles[message.pmNotification.severity]}`}>
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
          {message.content}
        </ReactMarkdown>
      </div>
    );
  }

  return (
    <div className={`group flex flex-col ${isUser ? 'items-end' : 'items-start'} animate-fade-in`}>
      <div className={`flex gap-2.5 ${isUser ? 'flex-row-reverse' : 'flex-row'} max-w-[85%]`}>
        {/* Role icon */}
        <div className="shrink-0 mt-3">
          {isUser ? (
            <div className="w-6 h-6 rounded-full bg-elevated flex items-center justify-center">
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="var(--color-fg-muted)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
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
        <div className={`min-w-0 px-4 py-3 ${
          isUser
            ? 'bg-elevated border-l-2 border-soul/40 text-fg rounded-2xl rounded-br-md'
            : 'text-fg rounded-2xl rounded-bl-md'
        }`}>
        {/* Thinking block above text content for assistant messages */}
        {!isUser && message.thinking && <ThinkingBlock content={message.thinking} />}

        {/* Text content always first */}
        {message.content && (
          isUser ? (
            <div className="prose prose-sm prose-soul max-w-none break-words prose-p:my-1 prose-li:my-0">
              <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>{message.content}</ReactMarkdown>
            </div>
          ) : (
            <div className="prose prose-sm prose-soul max-w-none break-words">
              <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>{fixMissingSpaces(message.content)}</ReactMarkdown>
            </div>
          )
        )}

        {/* Tool calls as compact group below text */}
        {message.toolCalls && message.toolCalls.length > 0 && (
          <ToolCallGroup toolCalls={message.toolCalls} />
        )}
        </div>
      </div>
      <div className={isUser ? '' : 'ml-[34px]'}>
        <MessageMeta message={message} onRetry={onRetry} onEdit={onEdit} isStreaming={isStreaming} />
      </div>
    </div>
  );
}
