import { useState } from 'react';
import type { MessageBubbleProps } from '../lib/types';
import { Markdown } from './Markdown';
import { ToolCallBlock } from './ToolCallBlock';

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
      if (
        typeof obj.name === 'string' &&
        typeof obj.input === 'object' &&
        obj.input !== null
      ) {
        return { name: obj.name, input: obj.input as Record<string, unknown> };
      }
    }
  } catch {
    // not valid JSON
  }
  return null;
}

export function MessageBubble({ message, isStreaming }: MessageBubbleProps) {
  const isUser = message.role === 'user';
  const isToolUse = message.role === 'tool_use';
  const isToolResult = message.role === 'tool_result';
  const [toolExpanded, setToolExpanded] = useState(false);

  if (isToolUse) {
    const toolData = parseToolUse(message.content);
    if (toolData) {
      return (
        <div
          data-testid="message-bubble"
          className="flex flex-col max-w-[80%] self-start items-start"
        >
          <span className="text-xs text-zinc-500 mb-1 px-1">Tool Call</span>
          <div className="w-full">
            <ToolCallBlock
              name={toolData.name}
              input={toolData.input}
              result={null}
              isExpanded={toolExpanded}
              onToggle={() => setToolExpanded((prev) => !prev)}
            />
          </div>
        </div>
      );
    }
  }

  if (isToolResult) {
    return (
      <div
        data-testid="message-bubble"
        className="flex flex-col max-w-[80%] self-start items-start"
      >
        <span className="text-xs text-zinc-500 mb-1 px-1">Tool Result</span>
        <div className="rounded-lg px-4 py-3 bg-zinc-900 border border-zinc-700">
          <p className="whitespace-pre-wrap break-words text-zinc-400 text-sm">
            {message.content || 'No output'}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div
      data-testid="message-bubble"
      className={`flex flex-col max-w-[80%] ${isUser ? 'self-end items-end' : 'self-start items-start'}`}
    >
      <span className="text-xs text-zinc-500 mb-1 px-1">
        {isUser ? 'You' : 'Assistant'}
      </span>
      <div
        className={`rounded-lg px-4 py-3 ${isUser ? 'bg-zinc-800' : 'bg-zinc-900'}`}
      >
        {isUser ? (
          <p className="whitespace-pre-wrap break-words text-zinc-100">
            {message.content}
          </p>
        ) : (
          <div className="relative">
            <Markdown content={message.content} />
            {isStreaming && (
              <span className="animate-pulse inline">&#9612;</span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
