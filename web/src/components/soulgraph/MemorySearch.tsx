import { useState, useCallback } from 'react';

/** Shape of a single memory search result from /memory/query */
export interface MemoryResult {
  doc_id: string;
  content: string;
  metadata: {
    agent?: string;
    type?: string;
    name?: string;
    description?: string;
    category?: string;
    source_path?: string;
    updated_at?: string;
    source_machine?: string;
    chunk_index?: number;
    total_chunks?: number;
  };
  score: number;
}

/** Wrapper response from POST /memory/query */
export interface MemoryQueryResponse {
  results: MemoryResult[];
  query: string;
  collection: string;
  total_in_collection: number;
  latency_ms: number;
}

/** Query parameters for the memory search API */
export interface MemoryQuery {
  collection: string;
  query: string;
  agent_filter?: string;
  type_filter?: string;
  top_k: number;
}

interface MemorySearchProps {
  /** Called when user submits a search — returns results from /memory/query */
  onSearch: (query: MemoryQuery) => Promise<MemoryResult[]>;
  /** Available agent names for the filter dropdown */
  agents?: string[];
  /** Loading indicator controlled by parent */
  loading?: boolean;
  /** Error message from parent */
  error?: string | null;
}

const COLLECTIONS = [
  { id: 'soul_agent_memory', label: 'Agent Memory' },
  { id: 'soul_shared_kb', label: 'Shared Knowledge' },
  { id: 'soul_briefs', label: 'Briefs' },
] as const;

const MEMORY_TYPES = [
  { id: '', label: 'All Types' },
  { id: 'user', label: 'User' },
  { id: 'feedback', label: 'Feedback' },
  { id: 'project', label: 'Project' },
  { id: 'reference', label: 'Reference' },
] as const;

function scoreColor(score: number): string {
  if (score >= 0.8) return 'text-emerald-400';
  if (score >= 0.5) return 'text-amber-400';
  return 'text-fg-muted';
}

function scoreBg(score: number): string {
  if (score >= 0.8) return 'bg-emerald-400/10';
  if (score >= 0.5) return 'bg-amber-400/10';
  return 'bg-zinc-500/10';
}

function typeIcon(type?: string): string {
  switch (type) {
    case 'user': return '👤';
    case 'feedback': return '💬';
    case 'project': return '📋';
    case 'reference': return '🔗';
    default: return '📄';
  }
}

export function MemorySearch({ onSearch, agents = [], loading = false, error = null }: MemorySearchProps) {
  const [query, setQuery] = useState('');
  const [collection, setCollection] = useState<string>(COLLECTIONS[0].id);
  const [agentFilter, setAgentFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [topK, setTopK] = useState(5);
  const [results, setResults] = useState<MemoryResult[]>([]);
  const [searched, setSearched] = useState(false);

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    const q: MemoryQuery = {
      collection,
      query: query.trim(),
      top_k: topK,
    };
    if (agentFilter) q.agent_filter = agentFilter;
    if (typeFilter) q.type_filter = typeFilter;

    const data = await onSearch(q);
    setResults(data);
    setSearched(true);
  }, [query, collection, agentFilter, typeFilter, topK, onSearch]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSearch();
      }
    },
    [handleSearch],
  );

  return (
    <div className="space-y-4" data-testid="memory-search">
      {/* Search input */}
      <div className="bg-surface rounded-lg p-4 space-y-3">
        <label className="text-sm font-medium text-fg-muted">Semantic Memory Search</label>
        <div className="flex gap-2">
          <input
            type="text"
            value={query}
            onChange={e => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search agent memories, knowledge base, briefs..."
            className="flex-1 px-3 py-2 text-sm bg-elevated border border-border-default rounded text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul"
            data-testid="memory-search-input"
            aria-label="Search query"
          />
          <button
            onClick={handleSearch}
            disabled={!query.trim() || loading}
            className="px-4 py-2 text-sm font-medium bg-soul text-white rounded disabled:opacity-50 disabled:cursor-not-allowed hover:bg-soul/90 transition-colors"
            data-testid="memory-search-submit"
          >
            {loading ? 'Searching...' : 'Search'}
          </button>
        </div>
      </div>

      {/* Filters row */}
      <div className="bg-surface rounded-lg p-4">
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {/* Collection */}
          <div className="space-y-1">
            <label className="text-xs text-fg-muted">Collection</label>
            <select
              value={collection}
              onChange={e => setCollection(e.target.value)}
              className="w-full px-2 py-1.5 text-sm bg-elevated border border-border-default rounded text-fg focus:outline-none focus:border-soul"
              data-testid="memory-collection-filter"
              aria-label="Collection filter"
            >
              {COLLECTIONS.map(c => (
                <option key={c.id} value={c.id}>{c.label}</option>
              ))}
            </select>
          </div>

          {/* Agent filter */}
          <div className="space-y-1">
            <label className="text-xs text-fg-muted">Agent</label>
            <select
              value={agentFilter}
              onChange={e => setAgentFilter(e.target.value)}
              className="w-full px-2 py-1.5 text-sm bg-elevated border border-border-default rounded text-fg focus:outline-none focus:border-soul"
              data-testid="memory-agent-filter"
              aria-label="Agent filter"
            >
              <option value="">All Agents</option>
              {agents.map(a => (
                <option key={a} value={a}>{a}</option>
              ))}
            </select>
          </div>

          {/* Type filter */}
          <div className="space-y-1">
            <label className="text-xs text-fg-muted">Type</label>
            <select
              value={typeFilter}
              onChange={e => setTypeFilter(e.target.value)}
              className="w-full px-2 py-1.5 text-sm bg-elevated border border-border-default rounded text-fg focus:outline-none focus:border-soul"
              data-testid="memory-type-filter"
              aria-label="Type filter"
            >
              {MEMORY_TYPES.map(t => (
                <option key={t.id} value={t.id}>{t.label}</option>
              ))}
            </select>
          </div>

          {/* Top K */}
          <div className="space-y-1">
            <label className="text-xs text-fg-muted">Results</label>
            <select
              value={topK}
              onChange={e => setTopK(Number(e.target.value))}
              className="w-full px-2 py-1.5 text-sm bg-elevated border border-border-default rounded text-fg focus:outline-none focus:border-soul"
              data-testid="memory-topk-filter"
              aria-label="Results count"
            >
              {[3, 5, 10, 20].map(n => (
                <option key={n} value={n}>{n} results</option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div
          className="bg-red-500/10 border border-red-500/30 rounded-lg p-3 text-sm text-red-400"
          data-testid="memory-search-error"
          role="alert"
        >
          {error}
        </div>
      )}

      {/* Results */}
      <div className="space-y-2" data-testid="memory-search-results">
        {searched && results.length === 0 && !loading && (
          <div
            className="bg-surface rounded-lg p-6 text-center text-sm text-fg-muted"
            data-testid="memory-search-empty"
          >
            No results found. Try a different query or broaden your filters.
          </div>
        )}

        {results.map(result => (
          <div
            key={result.doc_id}
            className="bg-surface rounded-lg p-4 space-y-2 hover:bg-overlay transition-colors"
            data-testid={`memory-result-${result.doc_id}`}
          >
            {/* Header: name + score */}
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2 min-w-0">
                <span className="text-sm" aria-hidden="true">{typeIcon(result.metadata.type)}</span>
                <span className="text-sm font-medium text-fg truncate" data-testid="memory-result-name">
                  {result.metadata.name || result.doc_id}
                </span>
                {result.metadata.agent && (
                  <span
                    className="px-1.5 py-0.5 text-xs rounded bg-soul/10 text-soul font-mono"
                    data-testid="memory-result-agent"
                  >
                    {result.metadata.agent}
                  </span>
                )}
                {result.metadata.type && (
                  <span
                    className="px-1.5 py-0.5 text-xs rounded bg-zinc-500/10 text-fg-muted"
                    data-testid="memory-result-type"
                  >
                    {result.metadata.type}
                  </span>
                )}
              </div>
              <span
                className={`px-2 py-0.5 text-xs rounded font-mono ${scoreBg(result.score)} ${scoreColor(result.score)}`}
                data-testid="memory-result-score"
              >
                {(result.score * 100).toFixed(0)}%
              </span>
            </div>

            {/* Description */}
            {result.metadata.description && (
              <div className="text-xs text-fg-muted" data-testid="memory-result-description">
                {result.metadata.description}
              </div>
            )}

            {/* Content preview (truncated) */}
            <div
              className="text-sm text-fg-secondary line-clamp-3 whitespace-pre-wrap"
              data-testid="memory-result-content"
            >
              {result.content}
            </div>

            {/* Footer: source path + updated */}
            <div className="flex items-center justify-between text-xs text-fg-muted">
              {result.metadata.source_path && (
                <span className="font-mono truncate max-w-[60%]" data-testid="memory-result-path">
                  {result.metadata.source_path}
                </span>
              )}
              {result.metadata.updated_at && (
                <span data-testid="memory-result-updated">
                  {new Date(result.metadata.updated_at).toLocaleDateString()}
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
