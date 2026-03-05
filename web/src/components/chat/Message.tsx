// web/src/components/chat/Message.tsx
import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ChatMessage, ToolCallMessage } from '../../lib/types.ts';
import ToolCall from './ToolCall.tsx';

interface MessageProps {
  message: ChatMessage;
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

export default function Message({ message }: MessageProps) {
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
        <ReactMarkdown remarkPlugins={[remarkGfm]}>
          {message.content}
        </ReactMarkdown>
      </div>
    );
  }

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} animate-fade-in`}>
      <div className={`max-w-[80%] px-4 py-3 ${
        isUser
          ? 'bg-elevated border-l-2 border-soul/40 text-fg rounded-2xl rounded-br-md'
          : 'text-fg rounded-2xl rounded-bl-md'
      }`}>
        {/* Thinking block above text content for assistant messages */}
        {!isUser && message.thinking && <ThinkingBlock content={message.thinking} />}

        {/* Text content always first */}
        {message.content && (
          isUser ? (
            <div className="whitespace-pre-wrap break-words text-sm leading-relaxed">
              {message.content}
            </div>
          ) : (
            <div className="prose prose-sm prose-soul max-w-none break-words">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{message.content}</ReactMarkdown>
            </div>
          )
        )}

        {/* Tool calls as compact group below text */}
        {message.toolCalls && message.toolCalls.length > 0 && (
          <ToolCallGroup toolCalls={message.toolCalls} />
        )}
      </div>
    </div>
  );
}
