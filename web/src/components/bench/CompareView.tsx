import { useState } from 'react';
import type { BenchResultSummary, CompareData } from '../../hooks/useBench';

interface CompareViewProps {
  results: BenchResultSummary[];
  compareData: CompareData | null;
  loading: boolean;
  onCompare: (id1: string, id2: string) => Promise<void>;
}

function MetricRow({ label, val1, val2, higherBetter }: { label: string; val1: number; val2: number; higherBetter: boolean }) {
  const delta = val2 - val1;
  const winner1 = higherBetter ? val1 > val2 : val1 < val2;
  const winner2 = higherBetter ? val2 > val1 : val2 < val1;

  return (
    <tr className="border-b border-border-subtle">
      <td className="py-2 px-3 text-fg-muted text-sm">{label}</td>
      <td className={`py-2 px-3 text-right font-mono text-sm ${winner1 ? 'text-emerald-400 font-bold' : 'text-fg-secondary'}`}>
        {typeof val1 === 'number' ? val1.toFixed(2) : val1}
      </td>
      <td className={`py-2 px-3 text-right font-mono text-sm ${winner2 ? 'text-emerald-400 font-bold' : 'text-fg-secondary'}`}>
        {typeof val2 === 'number' ? val2.toFixed(2) : val2}
      </td>
      <td className={`py-2 px-3 text-right font-mono text-sm ${delta > 0 ? 'text-emerald-400' : delta < 0 ? 'text-red-400' : 'text-fg-muted'}`}>
        {delta > 0 ? '+' : ''}{delta.toFixed(2)}
      </td>
    </tr>
  );
}

export function CompareView({ results, compareData, loading, onCompare }: CompareViewProps) {
  const [id1, setId1] = useState('');
  const [id2, setId2] = useState('');

  const handleCompare = async () => {
    if (!id1 || !id2 || id1 === id2) return;
    await onCompare(id1, id2);
  };

  return (
    <div className="space-y-4" data-testid="compare-view">
      {/* Selectors */}
      <div className="bg-surface rounded-lg p-4 space-y-3">
        <h4 className="text-sm font-medium text-fg-muted">Select Results to Compare</h4>
        <div className="flex flex-col sm:flex-row gap-3">
          <select
            value={id1}
            onChange={e => setId1(e.target.value)}
            className="soul-select flex-1"
            data-testid="compare-select-1"
          >
            <option value="">Select result 1</option>
            {results.map(r => (
              <option key={r.id} value={r.id}>{r.model_name} ({new Date(r.timestamp).toLocaleDateString()})</option>
            ))}
          </select>
          <span className="text-fg-muted self-center text-sm">vs</span>
          <select
            value={id2}
            onChange={e => setId2(e.target.value)}
            className="soul-select flex-1"
            data-testid="compare-select-2"
          >
            <option value="">Select result 2</option>
            {results.map(r => (
              <option key={r.id} value={r.id}>{r.model_name} ({new Date(r.timestamp).toLocaleDateString()})</option>
            ))}
          </select>
          <button
            onClick={handleCompare}
            disabled={loading || !id1 || !id2 || id1 === id2}
            className="px-4 py-2 text-sm rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors disabled:opacity-50"
            data-testid="compare-btn"
          >
            {loading ? 'Comparing...' : 'Compare'}
          </button>
        </div>
      </div>

      {/* Comparison results */}
      {compareData && (
        <div className="space-y-4">
          {/* Metrics table */}
          <div className="bg-surface rounded-lg p-4">
            <h4 className="text-sm font-medium text-fg-muted mb-3">Metrics Comparison</h4>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="text-xs text-fg-muted border-b border-border-subtle">
                    <th className="text-left py-2 px-3 font-medium">Metric</th>
                    <th className="text-right py-2 px-3 font-medium">{compareData.result1.model_name}</th>
                    <th className="text-right py-2 px-3 font-medium">{compareData.result2.model_name}</th>
                    <th className="text-right py-2 px-3 font-medium">Delta</th>
                  </tr>
                </thead>
                <tbody>
                  <MetricRow label="Accuracy (%)" val1={compareData.result1.accuracy} val2={compareData.result2.accuracy} higherBetter={true} />
                  <MetricRow label="Latency (s)" val1={compareData.result1.latency_s} val2={compareData.result2.latency_s} higherBetter={false} />
                  <MetricRow label="Tokens/sec" val1={compareData.result1.tokens_per_sec} val2={compareData.result2.tokens_per_sec} higherBetter={true} />
                  <MetricRow label="Peak RAM (MB)" val1={compareData.result1.peak_ram_mb} val2={compareData.result2.peak_ram_mb} higherBetter={false} />
                  <MetricRow label="CARS RAM" val1={compareData.result1.cars_ram} val2={compareData.result2.cars_ram} higherBetter={true} />
                  <MetricRow label="CARS Size" val1={compareData.result1.cars_size} val2={compareData.result2.cars_size} higherBetter={true} />
                </tbody>
              </table>
            </div>
          </div>

          {/* Per-category comparison */}
          <div className="bg-surface rounded-lg p-4">
            <h4 className="text-sm font-medium text-fg-muted mb-3">Per-Category Comparison</h4>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-xs text-fg-muted border-b border-border-subtle">
                    <th className="text-left py-2 px-3 font-medium">Category</th>
                    <th className="text-right py-2 px-3 font-medium">{compareData.result1.model_name}</th>
                    <th className="text-right py-2 px-3 font-medium">{compareData.result2.model_name}</th>
                    <th className="text-right py-2 px-3 font-medium">Delta</th>
                  </tr>
                </thead>
                <tbody>
                  {compareData.result1.category_scores.map(cs1 => {
                    const cs2 = compareData.result2.category_scores.find(c => c.category === cs1.category);
                    const acc2 = cs2?.accuracy ?? 0;
                    const delta = acc2 - cs1.accuracy;
                    return (
                      <tr key={cs1.category} className="border-b border-border-subtle">
                        <td className="py-2 px-3 text-fg">{cs1.category}</td>
                        <td className="py-2 px-3 text-right text-fg-secondary font-mono">{cs1.accuracy.toFixed(1)}%</td>
                        <td className="py-2 px-3 text-right text-fg-secondary font-mono">{acc2.toFixed(1)}%</td>
                        <td className={`py-2 px-3 text-right font-mono ${delta > 0 ? 'text-emerald-400' : delta < 0 ? 'text-red-400' : 'text-fg-muted'}`}>
                          {delta > 0 ? '+' : ''}{delta.toFixed(1)}%
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
