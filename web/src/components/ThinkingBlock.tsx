import { useState } from 'react';

interface ThinkingBlockProps {
  content: string;
  isStreaming: boolean;
}

export function ThinkingBlock({ content, isStreaming }: ThinkingBlockProps) {
  const [expanded, setExpanded] = useState(false);
  const lines = content.split('\n').length;

  return (
    <div className="mb-3">
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded(!expanded)}
        data-testid="thinking-toggle"
        className="flex items-center gap-1.5 text-xs text-fg-muted hover:text-fg transition-colors cursor-pointer font-mono"
      >
        <span>{'\uD83D\uDCAD'}</span>
        <span>Thinking{isStreaming ? '...' : ''} ({lines} lines)</span>
        <span className="text-[10px]">{expanded ? '\u25BE' : '\u25B8'}</span>
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
