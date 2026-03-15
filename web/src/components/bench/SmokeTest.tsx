import { useState } from 'react';
import type { SmokeResult } from '../../hooks/useBench';

interface SmokeTestProps {
  onRunSmoke: (endpoint: string) => Promise<SmokeResult[]>;
}

export function SmokeTest({ onRunSmoke }: SmokeTestProps) {
  const [endpoint, setEndpoint] = useState('');
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<SmokeResult[]>([]);

  const handleRun = async () => {
    if (!endpoint.trim()) return;
    setRunning(true);
    try {
      const data = await onRunSmoke(endpoint.trim());
      setResults(data);
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="space-y-4" data-testid="smoke-test">
      {/* Endpoint input */}
      <div className="bg-surface rounded-lg p-4 space-y-2">
        <label className="text-sm font-medium text-fg-muted">Model Endpoint</label>
        <div className="flex gap-2">
          <input
            type="text"
            value={endpoint}
            onChange={e => setEndpoint(e.target.value)}
            placeholder="https://api.example.com/v1/chat"
            className="flex-1 px-3 py-2 text-sm bg-elevated border border-border-default rounded text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul"
            data-testid="smoke-endpoint"
          />
          <button
            onClick={handleRun}
            disabled={running || !endpoint.trim()}
            className="px-4 py-2 text-sm rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors disabled:opacity-50"
            data-testid="run-smoke"
          >
            {running ? 'Running...' : 'Run Smoke'}
          </button>
        </div>
      </div>

      {/* Results */}
      {results.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3" data-testid="smoke-results">
          {results.map((result, idx) => (
            <div
              key={idx}
              className={`bg-surface rounded-lg p-4 border ${result.passed ? 'border-emerald-500/30' : 'border-red-500/30'}`}
              data-testid={`smoke-card-${idx}`}
            >
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium text-fg">{result.name}</span>
                <span className={`px-2 py-0.5 text-xs rounded-full ${result.passed ? 'bg-emerald-500/20 text-emerald-400' : 'bg-red-500/20 text-red-400'}`}>
                  {result.passed ? 'PASS' : 'FAIL'}
                </span>
              </div>
              <pre className="text-xs text-fg-muted bg-elevated rounded p-2 overflow-x-auto whitespace-pre-wrap max-h-24">
                {result.response_preview}
              </pre>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
