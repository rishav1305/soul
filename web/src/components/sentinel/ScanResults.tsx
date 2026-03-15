import { useState } from 'react';
import type { ScanResult } from '../../hooks/useSentinel';

interface ScanResultsProps {
  results: ScanResult[];
  onScan: (product: string) => Promise<void>;
}

const severityColor: Record<string, string> = {
  critical: 'bg-red-500/20 text-red-400',
  high: 'bg-orange-500/20 text-orange-400',
  medium: 'bg-amber-500/20 text-amber-400',
  low: 'bg-blue-500/20 text-blue-400',
};

const severityOrder: Record<string, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
};

const products = ['chat', 'tasks', 'tutor', 'projects', 'observe'];

export function ScanResults({ results, onScan }: ScanResultsProps) {
  const [selectedProduct, setSelectedProduct] = useState<string>('chat');
  const [scanning, setScanning] = useState(false);

  const sorted = [...results].sort((a, b) => (severityOrder[a.severity] ?? 4) - (severityOrder[b.severity] ?? 4));

  const handleScan = async () => {
    if (scanning) return;
    setScanning(true);
    try {
      await onScan(selectedProduct);
    } finally {
      setScanning(false);
    }
  };

  return (
    <div className="space-y-4" data-testid="scan-results">
      {/* Scan controls */}
      <div className="flex gap-2 items-center flex-wrap">
        <select
          value={selectedProduct}
          onChange={e => setSelectedProduct(e.target.value)}
          className="soul-select"
          data-testid="scan-product-select"
        >
          {products.map(p => (
            <option key={p} value={p}>{p}</option>
          ))}
        </select>
        <button
          onClick={handleScan}
          disabled={scanning}
          className="px-4 py-1.5 text-xs rounded-lg bg-soul text-deep font-medium hover:bg-soul/85 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          data-testid="scan-button"
        >
          {scanning ? 'Scanning...' : 'Run Scan'}
        </button>
        {results.length > 0 && (
          <span className="text-xs text-fg-muted ml-auto">{results.length} finding{results.length !== 1 ? 's' : ''}</span>
        )}
      </div>

      {/* Results table */}
      {sorted.length === 0 ? (
        <div className="text-sm text-fg-muted py-4 text-center">
          {scanning ? 'Scanning...' : 'No scan results yet. Select a product and run a scan.'}
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm" data-testid="scan-table">
            <thead>
              <tr className="text-left text-xs text-fg-muted border-b border-border-subtle">
                <th className="pb-2 pr-3 w-24">Severity</th>
                <th className="pb-2 pr-3 w-48">Title</th>
                <th className="pb-2 pr-3">Description</th>
                <th className="pb-2">Recommendation</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((result, i) => (
                <tr key={i} className="border-b border-border-subtle/50" data-testid={`scan-row-${i}`}>
                  <td className="py-2 pr-3">
                    <span className={`inline-block px-2 py-0.5 text-[10px] rounded-full ${severityColor[result.severity] ?? severityColor.low}`}>
                      {result.severity}
                    </span>
                  </td>
                  <td className="py-2 pr-3 text-fg font-medium">{result.title}</td>
                  <td className="py-2 pr-3 text-fg-secondary">{result.description}</td>
                  <td className="py-2 text-fg-muted">{result.recommendation}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
