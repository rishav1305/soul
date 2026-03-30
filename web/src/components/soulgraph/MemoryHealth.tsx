/** ChromaDB collection health dashboard widget.
 *  Calls GET /memory/health and displays collection stats.
 */

export interface CollectionHealth {
  [name: string]: number;
}

export interface MemoryHealthData {
  chromadb: 'up' | 'down';
  collections: CollectionHealth;
  last_index: string;
}

interface MemoryHealthProps {
  health: MemoryHealthData | null;
  loading?: boolean;
  error?: string | null;
  onRefresh?: () => void;
}

function statusColor(status: 'up' | 'down'): string {
  return status === 'up' ? 'text-emerald-400' : 'text-red-400';
}

function statusBg(status: 'up' | 'down'): string {
  return status === 'up' ? 'bg-emerald-400/10' : 'bg-red-400/10';
}

function formatTimestamp(ts: string): string {
  if (!ts) return 'Never';
  const d = new Date(ts);
  const now = new Date();
  const diffMs = now.getTime() - d.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return 'Just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  return d.toLocaleDateString();
}

export function MemoryHealth({ health, loading = false, error = null, onRefresh }: MemoryHealthProps) {
  const totalDocs = health
    ? Object.values(health.collections).reduce((sum, count) => sum + count, 0)
    : 0;

  return (
    <div className="bg-surface rounded-lg p-4 space-y-3" data-testid="memory-health">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-fg">ChromaDB Memory</span>
          {health && (
            <span
              className={`px-2 py-0.5 text-xs rounded-full font-medium ${statusBg(health.chromadb)} ${statusColor(health.chromadb)}`}
              data-testid="memory-health-status"
            >
              {health.chromadb === 'up' ? 'Online' : 'Offline'}
            </span>
          )}
        </div>
        {onRefresh && (
          <button
            onClick={onRefresh}
            disabled={loading}
            className="text-xs text-fg-muted hover:text-fg transition-colors disabled:opacity-50"
            data-testid="memory-health-refresh"
            aria-label="Refresh health status"
          >
            {loading ? '...' : '↻ Refresh'}
          </button>
        )}
      </div>

      {/* Error */}
      {error && (
        <div
          className="text-xs text-red-400 bg-red-500/10 rounded px-2 py-1"
          data-testid="memory-health-error"
          role="alert"
        >
          {error}
        </div>
      )}

      {/* Loading */}
      {loading && !health && (
        <div className="text-xs text-fg-muted" data-testid="memory-health-loading">
          Loading health data...
        </div>
      )}

      {/* Stats */}
      {health && (
        <>
          {/* Collection counts */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
            <div className="bg-elevated rounded p-2" data-testid="memory-health-total">
              <div className="text-xs text-fg-muted">Total Docs</div>
              <div className="text-lg font-bold text-fg">{totalDocs}</div>
            </div>
            {Object.entries(health.collections).map(([name, count]) => (
              <div
                key={name}
                className="bg-elevated rounded p-2"
                data-testid={`memory-health-collection-${name}`}
              >
                <div className="text-xs text-fg-muted truncate" title={name}>
                  {name.replace('soul_', '')}
                </div>
                <div className="text-lg font-bold text-fg">{count}</div>
              </div>
            ))}
          </div>

          {/* Last index time */}
          <div className="flex items-center justify-between text-xs text-fg-muted">
            <span>Last indexed</span>
            <span data-testid="memory-health-last-index" className="font-mono">
              {formatTimestamp(health.last_index)}
            </span>
          </div>
        </>
      )}

      {/* Empty state — no health data and no error */}
      {!health && !loading && !error && (
        <div className="text-xs text-fg-muted" data-testid="memory-health-empty">
          No health data available. ChromaDB may not be configured.
        </div>
      )}
    </div>
  );
}
