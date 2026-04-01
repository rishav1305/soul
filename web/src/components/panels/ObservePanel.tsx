/**
 * ObservePanel — ChromaDB memory search + health dashboard.
 * Wires MemorySearch + MemoryHealth components to the SoulGraph memory API.
 * Phase 0 Stream C Day 2 — Apr 1, 2026.
 */
import { useEffect, useState } from 'react';
import { useMemorySearch } from '../../hooks/useMemorySearch.ts';
import { MemorySearch } from '../soulgraph/MemorySearch.tsx';
import { MemoryHealth } from '../soulgraph/MemoryHealth.tsx';
import type { MemoryQuery, MemoryResult } from '../soulgraph/MemorySearch.tsx';

/** Known agents — sourced from /status endpoint. */
const KNOWN_AGENTS = [
  'banner', 'friday', 'fury', 'happy', 'hawkeye',
  'loki', 'pepper', 'shuri', 'stark', 'xavier',
];

export default function ObservePanel() {
  const {
    health,
    loading: searchLoading,
    healthLoading,
    error: searchError,
    healthError,
    search,
    fetchHealth,
  } = useMemorySearch();

  const [activeTab, setActiveTab] = useState<'search' | 'health'>('health');
  const [searchResults, setSearchResults] = useState<MemoryResult[]>([]);

  // Auto-fetch health on mount.
  useEffect(() => {
    fetchHealth();
  }, [fetchHealth]);

  const handleSearch = async (query: MemoryQuery): Promise<MemoryResult[]> => {
    const results = await search(query);
    setSearchResults(results);
    return results;
  };

  return (
    <div className="h-full flex flex-col overflow-hidden" data-testid="observe-panel">
      {/* Tab bar */}
      <div className="flex items-center gap-1 px-4 py-2 border-b border-border-subtle shrink-0">
        {(
          [
            { id: 'health', label: 'Memory Health' },
            { id: 'search', label: 'Memory Search' },
          ] as { id: 'health' | 'search'; label: string }[]
        ).map(tab => (
          <button
            key={tab.id}
            type="button"
            onClick={() => setActiveTab(tab.id)}
            data-testid={`observe-tab-${tab.id}`}
            className={`px-3 py-1.5 text-xs font-medium rounded transition-colors cursor-pointer ${
              activeTab === tab.id
                ? 'bg-soul/15 text-soul'
                : 'text-fg-muted hover:text-fg hover:bg-elevated'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {activeTab === 'health' && (
          <MemoryHealth
            health={health}
            loading={healthLoading}
            error={healthError}
            onRefresh={fetchHealth}
          />
        )}
        {activeTab === 'search' && (
          <MemorySearch
            onSearch={handleSearch}
            agents={KNOWN_AGENTS}
            loading={searchLoading}
            error={searchError}
          />
        )}
      </div>
    </div>
  );
}
