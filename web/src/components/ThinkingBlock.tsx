import { useState } from 'react';

interface ThinkingBlockProps {
  content: string;
  isStreaming: boolean;
}

function LightbulbIcon({ streaming }: { streaming: boolean }) {
  return (
    <svg
      width="12" height="12" viewBox="0 0 16 16" fill="none"
      stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"
      className={streaming ? 'text-soul drop-shadow-[0_0_3px_var(--color-soul-glow)]' : 'text-fg-muted'}
    >
      <path d="M8 2a4.5 4.5 0 0 1 2.5 8.2V12h-5v-1.8A4.5 4.5 0 0 1 8 2z" />
      <path d="M6 14h4" />
    </svg>
  );
}

export function ThinkingBlock({ content, isStreaming }: ThinkingBlockProps) {
  const [expanded, setExpanded] = useState(isStreaming);

  // Collapsed pill (completed thinking)
  if (!isStreaming && !expanded) {
    const duration = content.length > 500 ? `${(content.length / 200).toFixed(1)}s` : `${content.split('\n').length} lines`;
    return (
      <div className="mb-2">
        <button
          type="button"
          onClick={() => setExpanded(true)}
          data-testid="thinking-toggle"
          className="flex items-center gap-1.5 text-[11px] text-fg-muted hover:text-fg-secondary transition-colors cursor-pointer bg-surface border border-border-subtle rounded-md px-2.5 py-1"
        >
          <LightbulbIcon streaming={false} />
          <span className="font-mono">Extended thinking</span>
          <span className="text-fg-muted">·</span>
          <span className="font-mono">{duration}</span>
          <svg width="8" height="8" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <path d="M4 6l4 4 4-4" />
          </svg>
        </button>
      </div>
    );
  }

  // Expanded (streaming or user expanded)
  return (
    <div className="mb-2">
      <div className={`border rounded-lg p-3 max-w-[380px] ${
        isStreaming ? 'border-soul/25 bg-surface' : 'border-border-subtle bg-surface'
      }`}>
        <button
          type="button"
          onClick={() => !isStreaming && setExpanded(false)}
          data-testid="thinking-toggle"
          className="flex items-center gap-1.5 w-full text-left cursor-pointer mb-1.5"
        >
          <LightbulbIcon streaming={isStreaming} />
          <span className={`text-[11px] font-mono font-semibold tracking-wide ${isStreaming ? 'text-soul' : 'text-fg-muted'}`}>
            Extended thinking{isStreaming ? '...' : ''}
          </span>
          <div className="flex-1" />
          {!isStreaming && (
            <svg width="8" height="8" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="text-fg-muted">
              <path d="M12 10l-4-4-4 4" />
            </svg>
          )}
        </button>
        <div className="border-l-2 border-soul/20 pl-2 max-h-48 overflow-y-auto">
          <pre className="text-[11px] text-fg-muted font-mono whitespace-pre-wrap leading-relaxed">
            {content}
            {isStreaming && <span className="inline-block w-1 h-1 rounded-full bg-soul ml-1 align-middle animate-pulse" />}
          </pre>
        </div>
      </div>
    </div>
  );
}
