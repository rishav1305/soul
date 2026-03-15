import { useEffect } from 'react';
import { useBench } from '../hooks/useBench';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import { BenchRunner } from '../components/bench/BenchRunner';
import { SmokeTest } from '../components/bench/SmokeTest';
import { ResultsTable } from '../components/bench/ResultsTable';
import { ResultDetail } from '../components/bench/ResultDetail';
import { CompareView } from '../components/bench/CompareView';

export function BenchPage() {
  usePerformance('BenchPage');
  const {
    categories, results, selectedResult, compareData,
    loading, error, activeTab, setActiveTab,
    refresh, fetchResultDetail, runBenchmark, runSmoke, compare,
  } = useBench();

  useEffect(() => { reportUsage('page.view', { page: 'bench' }); }, []);

  const tabs = ['run', 'results', 'compare'] as const;

  return (
    <div data-testid="bench-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-fg">Bench</h2>
        <button onClick={refresh} className="px-3 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors" data-testid="bench-refresh">Refresh</button>
      </div>

      {/* Tab nav */}
      <nav className="tab-scroll flex gap-1 border-b border-border-subtle pb-px" data-testid="bench-tabs">
        {tabs.map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className={`px-3 py-1.5 text-sm rounded-t transition-colors capitalize ${activeTab === tab ? 'bg-surface text-fg' : 'text-fg-muted hover:text-zinc-200'}`}
            data-testid={`tab-${tab}`}>
            {tab}
          </button>
        ))}
      </nav>

      {error && <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="bench-error">{error}</div>}

      {/* Tab content */}
      {activeTab === 'run' && (
        <div className="space-y-4">
          <BenchRunner categories={categories} loading={loading} onRun={runBenchmark} />
          <SmokeTest onRunSmoke={runSmoke} />
        </div>
      )}

      {activeTab === 'results' && (
        selectedResult ? (
          <ResultDetail result={selectedResult} onClose={() => fetchResultDetail('')} />
        ) : (
          <ResultsTable results={results} onSelect={fetchResultDetail} />
        )
      )}

      {activeTab === 'compare' && (
        <CompareView results={results} compareData={compareData} loading={loading} onCompare={compare} />
      )}

      {loading && categories.length === 0 && results.length === 0 && (
        <div className="text-center py-8 text-fg-muted">Loading...</div>
      )}
    </div>
  );
}
