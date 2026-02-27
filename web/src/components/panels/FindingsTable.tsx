import { useState, useMemo } from 'react';
import type { FindingMessage } from '../../lib/types.ts';

interface FindingsTableProps {
  findings: FindingMessage[];
}

type SortKey = 'severity' | 'file';
type SortDir = 'asc' | 'desc';

const SEVERITY_ORDER: Record<string, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
  info: 4,
};

const SEVERITY_LABELS = ['critical', 'high', 'medium', 'low', 'info'] as const;

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-red-500',
  high: 'bg-red-400',
  medium: 'bg-amber-500',
  low: 'bg-sky-500',
  info: 'bg-zinc-500',
};

const SEVERITY_TEXT_COLORS: Record<string, string> = {
  critical: 'text-red-400',
  high: 'text-red-400',
  medium: 'text-amber-400',
  low: 'text-sky-400',
  info: 'text-zinc-400',
};

export default function FindingsTable({ findings }: FindingsTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>('severity');
  const [sortDir, setSortDir] = useState<SortDir>('asc');
  const [filters, setFilters] = useState<Set<string>>(
    new Set(SEVERITY_LABELS),
  );
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir('asc');
    }
  };

  const toggleFilter = (sev: string) => {
    setFilters((prev) => {
      const next = new Set(prev);
      if (next.has(sev)) {
        next.delete(sev);
      } else {
        next.add(sev);
      }
      return next;
    });
  };

  const sorted = useMemo(() => {
    const filtered = findings.filter((f) =>
      filters.has(f.severity.toLowerCase()),
    );

    return filtered.sort((a, b) => {
      let cmp = 0;
      if (sortKey === 'severity') {
        cmp =
          (SEVERITY_ORDER[a.severity.toLowerCase()] ?? 4) -
          (SEVERITY_ORDER[b.severity.toLowerCase()] ?? 4);
      } else {
        cmp = (a.file ?? '').localeCompare(b.file ?? '');
      }
      return sortDir === 'asc' ? cmp : -cmp;
    });
  }, [findings, filters, sortKey, sortDir]);

  if (findings.length === 0) {
    return (
      <div className="p-6 text-center text-zinc-600 text-sm">
        No findings yet. Run a scan to get started.
      </div>
    );
  }

  return (
    <div className="p-4 space-y-3">
      {/* Filter toggles */}
      <div className="flex gap-1.5 flex-wrap">
        {SEVERITY_LABELS.map((sev) => (
          <button
            key={sev}
            onClick={() => toggleFilter(sev)}
            className={`px-2 py-0.5 rounded text-[10px] font-medium uppercase transition-colors ${
              filters.has(sev)
                ? `${SEVERITY_COLORS[sev]} text-white`
                : 'bg-zinc-800 text-zinc-500'
            }`}
          >
            {sev}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="text-xs">
        {/* Header */}
        <div className="flex items-center gap-2 px-2 py-1.5 border-b border-zinc-800 text-zinc-500 font-medium">
          <button
            onClick={() => toggleSort('severity')}
            className="w-16 text-left hover:text-zinc-300 transition-colors"
          >
            Sev {sortKey === 'severity' ? (sortDir === 'asc' ? '\u2191' : '\u2193') : ''}
          </button>
          <span className="w-16">ID</span>
          <span className="flex-1">Title</span>
          <button
            onClick={() => toggleSort('file')}
            className="w-32 text-right hover:text-zinc-300 transition-colors"
          >
            File {sortKey === 'file' ? (sortDir === 'asc' ? '\u2191' : '\u2193') : ''}
          </button>
        </div>

        {/* Rows */}
        {sorted.map((f) => (
          <div key={f.id}>
            <button
              onClick={() =>
                setExpandedId(expandedId === f.id ? null : f.id)
              }
              className="w-full flex items-center gap-2 px-2 py-2 hover:bg-zinc-800/50 transition-colors text-left"
            >
              <span className="w-16">
                <span
                  className={`inline-block w-2 h-2 rounded-full ${SEVERITY_COLORS[f.severity.toLowerCase()] ?? 'bg-zinc-500'}`}
                />
              </span>
              <span className="w-16 text-zinc-500 font-mono">{f.id}</span>
              <span className="flex-1 text-zinc-300 truncate">{f.title}</span>
              <span className="w-32 text-right text-zinc-600 font-mono truncate">
                {f.file ?? ''}
                {f.line != null ? `:${f.line}` : ''}
              </span>
            </button>

            {expandedId === f.id && (
              <div className="px-4 py-3 bg-zinc-900/50 border-t border-zinc-800/50 space-y-2">
                <div className="flex items-center gap-2">
                  <span
                    className={`text-[10px] font-medium uppercase ${SEVERITY_TEXT_COLORS[f.severity.toLowerCase()] ?? 'text-zinc-400'}`}
                  >
                    {f.severity}
                  </span>
                </div>
                {f.evidence && (
                  <pre className="text-zinc-400 text-[11px] whitespace-pre-wrap bg-zinc-950 rounded p-2">
                    {f.evidence}
                  </pre>
                )}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
