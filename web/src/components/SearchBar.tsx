import { useEffect, useRef } from 'react';

interface SearchBarProps {
  query: string;
  onChange: (q: string) => void;
  onClose: () => void;
  matchCount: number;
}

export function SearchBar({ query, onChange, onClose, matchCount }: SearchBarProps) {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  return (
    <div
      data-testid="search-bar"
      role="search"
      aria-label="Search messages"
      className="flex items-center gap-2 px-4 py-2 bg-surface border-b border-border-subtle shrink-0"
    >
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" className="text-fg-muted shrink-0" aria-hidden="true">
        <circle cx="7" cy="7" r="5" stroke="currentColor" strokeWidth="1.5" />
        <path d="M11 11l3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
      <input
        ref={inputRef}
        data-testid="search-input"
        type="text"
        value={query}
        onChange={e => onChange(e.target.value)}
        onKeyDown={e => { if (e.key === 'Escape') onClose(); }}
        placeholder="Search messages..."
        aria-label="Search messages"
        className="flex-1 bg-transparent text-sm text-fg placeholder-fg-muted outline-none"
      />
      {query && (
        <span data-testid="search-match-count" className="text-xs text-fg-muted" aria-live="polite">
          {matchCount} {matchCount === 1 ? 'match' : 'matches'}
        </span>
      )}
      <button
        data-testid="search-close"
        onClick={onClose}
        className="p-1 text-fg-muted hover:text-fg transition-colors"
        aria-label="Close search"
      >
        &#10005;
      </button>
    </div>
  );
}
