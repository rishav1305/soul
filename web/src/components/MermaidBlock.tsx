import { useEffect, useRef, useState } from 'react';

let mermaidPromise: Promise<typeof import('mermaid')> | null = null;

function getMermaid() {
  if (!mermaidPromise) {
    mermaidPromise = import('mermaid').then(m => {
      m.default.initialize({
        startOnLoad: false,
        theme: 'dark',
        themeVariables: {
          primaryColor: '#e8a849',
          secondaryColor: '#1a1a24',
          lineColor: '#6a6a82',
          primaryTextColor: '#ededf0',
          secondaryTextColor: '#9494ac',
        },
      });
      return m;
    });
  }
  return mermaidPromise;
}

let counter = 0;

export function MermaidBlock({ content }: { content: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [showSource, setShowSource] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const id = `mermaid-${++counter}`;

    getMermaid()
      .then(m => {
        if (cancelled) return;
        return m.default.render(id, content);
      })
      .then(result => {
        if (cancelled || !result || !containerRef.current) return;
        // SECURITY NOTE: Mermaid.render() produces sanitized SVG output.
        // The mermaid library internally sanitizes its output and does not
        // include user-controlled strings in executable contexts.
        // This is a documented exception to the "no innerHTML" pillar rule.
        containerRef.current.innerHTML = result.svg;
      })
      .catch(err => {
        if (!cancelled) setError(String(err));
      });

    return () => { cancelled = true; };
  }, [content]);

  if (error) {
    return (
      <div className="bg-surface border border-red-700/50 rounded-lg p-3">
        <div className="text-xs text-red-400 mb-2">Mermaid render error</div>
        <pre className="text-xs font-mono text-fg-muted whitespace-pre-wrap">{content}</pre>
      </div>
    );
  }

  return (
    <div className="bg-surface border border-border-subtle rounded-lg overflow-hidden">
      <div
        ref={containerRef}
        data-testid="mermaid-diagram"
        className="p-4 flex justify-center [&_svg]:max-w-full"
      />
      <button
        type="button"
        onClick={() => setShowSource(prev => !prev)}
        className="w-full text-left px-3 py-1.5 text-[10px] text-fg-muted hover:text-fg-secondary border-t border-border-subtle transition-colors"
      >
        {showSource ? 'Hide source' : 'Show source'}
      </button>
      {showSource && (
        <pre className="px-3 py-2 text-xs font-mono text-fg-muted border-t border-border-subtle max-h-40 overflow-y-auto">
          {content}
        </pre>
      )}
    </div>
  );
}
