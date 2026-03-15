import type { BenchResultSummary } from '../../hooks/useBench';

interface ResultsTableProps {
  results: BenchResultSummary[];
  onSelect: (id: string) => void;
}

export function ResultsTable({ results, onSelect }: ResultsTableProps) {
  if (results.length === 0) {
    return <div className="text-sm text-fg-muted py-4">No benchmark results yet.</div>;
  }

  return (
    <div className="overflow-x-auto" data-testid="results-table">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-xs text-fg-muted border-b border-border-subtle">
            <th className="text-left py-2 px-3 font-medium">Model</th>
            <th className="text-left py-2 px-3 font-medium hidden sm:table-cell">Timestamp</th>
            <th className="text-right py-2 px-3 font-medium">Accuracy</th>
            <th className="text-right py-2 px-3 font-medium">Latency</th>
            <th className="text-right py-2 px-3 font-medium hidden md:table-cell">CARS RAM</th>
            <th className="text-right py-2 px-3 font-medium hidden md:table-cell">CARS Size</th>
          </tr>
        </thead>
        <tbody>
          {results.map(result => (
            <tr
              key={result.id}
              onClick={() => onSelect(result.id)}
              className="border-b border-border-subtle cursor-pointer hover:bg-surface transition-colors"
              data-testid={`result-row-${result.id}`}
            >
              <td className="py-2 px-3 text-fg font-medium">{result.model_name}</td>
              <td className="py-2 px-3 text-fg-muted hidden sm:table-cell">
                {new Date(result.timestamp).toLocaleString()}
              </td>
              <td className="py-2 px-3 text-right">
                <span className={result.accuracy >= 80 ? 'text-emerald-400' : result.accuracy >= 50 ? 'text-amber-400' : 'text-red-400'}>
                  {result.accuracy.toFixed(1)}%
                </span>
              </td>
              <td className="py-2 px-3 text-right text-fg-secondary font-mono">
                {result.latency_s.toFixed(2)}s
              </td>
              <td className="py-2 px-3 text-right text-fg-secondary font-mono hidden md:table-cell">
                {result.cars_ram}
              </td>
              <td className="py-2 px-3 text-right text-fg-secondary font-mono hidden md:table-cell">
                {result.cars_size}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
