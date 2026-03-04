// web/src/components/chat/Message.tsx
import { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ChatMessage, ToolCallMessage } from '../../lib/types.ts';
import ToolCall from './ToolCall.tsx';

interface MessageProps {
  message: ChatMessage;
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

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} animate-fade-in`}>
      <div className={`max-w-[80%] px-4 py-3 ${
        isUser
          ? 'bg-elevated border-l-2 border-soul/40 text-fg rounded-2xl rounded-br-md'
          : 'text-fg rounded-2xl rounded-bl-md'
      }`}>
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
