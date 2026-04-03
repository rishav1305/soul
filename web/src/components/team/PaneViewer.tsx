import { useEffect, useRef } from 'react';

/**
 * PaneViewer — read-only terminal output pane with ring-buffer capping.
 *
 * Retains the last 500 lines in the DOM (ring-buffer cap, not true windowed
 * virtual scrolling). For typical 9-agent usage (9 × 500 = 4,500 DOM nodes)
 * this is acceptable. If performance degrades at higher line counts, upgrade
 * to windowed virtualisation (useRef + scroll-position math or react-virtual).
 */

interface PaneViewerProps {
  lines: string[];
  /** Auto-scroll to bottom when new lines arrive */
  autoScroll?: boolean;
  className?: string;
}

/** Filter out Claude Code ASCII startup banner lines (noise in Web UI). */
function filterBannerLines(input: string[]): string[] {
  return input.filter(line => {
    // ASCII art block characters from the Claude Code banner
    if (/[▐▛▜▝█▘]/.test(line)) return false;
    // Version line: "Claude Code v2.1.90" etc.
    if (/Claude Code v\d/.test(line)) return false;
    // "Claude Enterprise" or "Claude Max" branding line
    if (/^\s*(Claude Enterprise|Claude Max)/.test(line)) return false;
    // Model line that appears alongside the banner: "Opus 4.6 with high"
    if (/^\s*Opus \d/.test(line)) return false;
    return true;
  });
}

export function PaneViewer({ lines, autoScroll = true, className = '' }: PaneViewerProps) {
  const bottomRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const userScrolledRef = useRef(false);

  // Track user scroll to pause auto-scroll
  const handleScroll = () => {
    const el = containerRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
    userScrolledRef.current = !atBottom;
  };

  useEffect(() => {
    if (!autoScroll || userScrolledRef.current) return;
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [lines, autoScroll]);

  // Filter banner noise, then ring buffer — keep last 500 lines
  const filtered = filterBannerLines(lines);
  const visible = filtered.length > 500 ? filtered.slice(filtered.length - 500) : filtered;

  return (
    <div
      ref={containerRef}
      data-testid="pane-viewer"
      onScroll={handleScroll}
      className={`overflow-y-auto overflow-x-auto bg-deep font-mono text-xs leading-relaxed text-green-400 p-3 ${className}`}
      style={{ fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', ui-monospace, monospace" }}
      aria-label="Agent terminal output"
      role="log"
      aria-live="polite"
      aria-atomic={false}
    >
      {visible.map((line, idx) => (
        <div key={`${idx}-${line.slice(0, 12)}`} className="whitespace-pre">
          {line || '\u00A0'}
        </div>
      ))}
      <div ref={bottomRef} />
    </div>
  );
}
