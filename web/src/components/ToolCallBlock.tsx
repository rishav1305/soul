import { useState } from 'react';
import type { ToolCallBlockProps } from '../lib/types';

interface Props extends ToolCallBlockProps {
  onToggle?: () => void;
}

export function ToolCallBlock({ name, input, result, isExpanded, onToggle }: Props) {
  const [expanded, setExpanded] = useState(isExpanded);

  const isPending = result === null;

  function handleToggle() {
    setExpanded((prev) => !prev);
    onToggle?.();
  }

  return (
    <div
      data-testid="tool-call-block"
      className="bg-zinc-900 border border-zinc-700 rounded-lg overflow-hidden"
    >
      <button
        type="button"
        onClick={handleToggle}
        className="flex items-center gap-2 w-full px-3 py-2 text-left text-sm hover:bg-zinc-800 transition-colors"
      >
        <span className="text-zinc-500 text-xs w-4 flex-shrink-0">
          {expanded ? '\u25BC' : '\u25B6'}
        </span>
        <span className="font-mono text-zinc-300 truncate">{name}</span>
        <span className="ml-auto flex-shrink-0">
          {isPending ? (
            <span className="inline-block w-3.5 h-3.5 border-2 border-zinc-500 border-t-zinc-300 rounded-full animate-spin" />
          ) : (
            <span className="text-green-500 text-sm">&#10003;</span>
          )}
        </span>
      </button>

      {expanded && (
        <div className="border-t border-zinc-700 px-3 py-2 space-y-2">
          <div>
            <div className="text-xs text-zinc-500 mb-1">Input</div>
            <pre className="bg-zinc-950 rounded p-2 text-xs font-mono text-zinc-400 overflow-x-auto whitespace-pre-wrap">
              {JSON.stringify(input, null, 2)}
            </pre>
          </div>
          <div>
            <div className="text-xs text-zinc-500 mb-1">Result</div>
            <div className="bg-zinc-950 rounded p-2 text-xs text-zinc-400 overflow-x-auto whitespace-pre-wrap">
              {result ?? 'Waiting...'}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
