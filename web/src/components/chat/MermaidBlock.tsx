import { useEffect, useRef, useState } from 'react';

let mermaidId = 0;
let mermaidReady: Promise<typeof import('mermaid')['default']> | null = null;

function getMermaid() {
  if (!mermaidReady) {
    mermaidReady = import('mermaid').then((mod) => {
      mod.default.initialize({
        startOnLoad: false,
        theme: 'dark',
        themeVariables: {
          darkMode: true,
          background: 'transparent',
          primaryColor: '#a78bfa',
          primaryTextColor: '#e4e4e7',
          lineColor: '#71717a',
          secondaryColor: '#27272a',
        },
      });
      return mod.default;
    });
  }
  return mermaidReady;
}

interface MermaidBlockProps {
  children: string;
}

export default function MermaidBlock({ children }: MermaidBlockProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [svg, setSvg] = useState<string>('');

  useEffect(() => {
    const id = `mermaid-${++mermaidId}`;
    let cancelled = false;

    getMermaid().then((m) =>
      m.render(id, children.trim())
    ).then(({ svg: rendered }) => {
      if (!cancelled) setSvg(rendered);
    }).catch((err) => {
      if (!cancelled) setError(String(err));
    });

    return () => { cancelled = true; };
  }, [children]);

  if (error) {
    return (
      <div className="my-3 p-3 rounded-lg border border-border-subtle bg-elevated">
        <div className="text-[10px] text-fg-muted mb-1">Mermaid parse error</div>
        <pre className="text-xs text-stage-blocked whitespace-pre-wrap">{error}</pre>
        <pre className="mt-2 text-xs text-fg-muted whitespace-pre-wrap">{children}</pre>
      </div>
    );
  }

  if (!svg) return null;

  return (
    <div
      ref={containerRef}
      className="my-3 p-4 rounded-lg border border-border-subtle bg-elevated/40 overflow-x-auto [&_svg]:max-w-full"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
