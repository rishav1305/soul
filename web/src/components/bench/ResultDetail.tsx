import { useState } from 'react';
import type { BenchResultDetail as ResultDetailType } from '../../hooks/useBench';

interface ResultDetailProps {
  result: ResultDetailType;
  onClose: () => void;
}

export function ResultDetail({ result, onClose }: ResultDetailProps) {
  const [expandedPrompt, setExpandedPrompt] = useState<string | null>(null);

  return (
    <div className="space-y-4" data-testid={`result-detail-${result.id}`}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold text-fg">{result.model_name}</h3>
        <button
          onClick={onClose}
          className="px-3 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors"
          data-testid="result-detail-close"
        >
          Close
        </button>
      </div>

      {/* Summary stats */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        <div className="bg-surface rounded-lg p-3 text-center">
          <div className="text-xs text-fg-muted">Accuracy</div>
          <div className={`text-lg font-bold mt-1 ${result.accuracy >= 80 ? 'text-emerald-400' : result.accuracy >= 50 ? 'text-amber-400' : 'text-red-400'}`}>
            {result.accuracy.toFixed(1)}%
          </div>
        </div>
        <div className="bg-surface rounded-lg p-3 text-center">
          <div className="text-xs text-fg-muted">Latency</div>
          <div className="text-lg font-bold text-fg mt-1">{result.latency_s.toFixed(2)}s</div>
        </div>
        <div className="bg-surface rounded-lg p-3 text-center">
          <div className="text-xs text-fg-muted">Tokens/sec</div>
          <div className="text-lg font-bold text-fg mt-1">{result.tokens_per_sec.toFixed(1)}</div>
        </div>
        <div className="bg-surface rounded-lg p-3 text-center">
          <div className="text-xs text-fg-muted">Peak RAM</div>
          <div className="text-lg font-bold text-fg mt-1">{result.peak_ram_mb} MB</div>
        </div>
        <div className="bg-surface rounded-lg p-3 text-center">
          <div className="text-xs text-fg-muted">CARS RAM</div>
          <div className="text-lg font-bold text-fg mt-1">{result.cars_ram}</div>
        </div>
        <div className="bg-surface rounded-lg p-3 text-center">
          <div className="text-xs text-fg-muted">CARS Size</div>
          <div className="text-lg font-bold text-fg mt-1">{result.cars_size}</div>
        </div>
      </div>

      {/* Per-category accuracy */}
      <div className="bg-surface rounded-lg p-4 space-y-2">
        <h4 className="text-sm font-medium text-fg-muted">Per-Category Accuracy</h4>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-fg-muted border-b border-border-subtle">
                <th className="text-left py-1.5 px-2 font-medium">Category</th>
                <th className="text-right py-1.5 px-2 font-medium">Correct</th>
                <th className="text-right py-1.5 px-2 font-medium">Total</th>
                <th className="text-right py-1.5 px-2 font-medium">Accuracy</th>
              </tr>
            </thead>
            <tbody>
              {result.category_scores.map(cs => (
                <tr key={cs.category} className="border-b border-border-subtle">
                  <td className="py-1.5 px-2 text-fg">{cs.category}</td>
                  <td className="py-1.5 px-2 text-right text-fg-secondary">{cs.prompts_correct}</td>
                  <td className="py-1.5 px-2 text-right text-fg-secondary">{cs.prompts_total}</td>
                  <td className="py-1.5 px-2 text-right">
                    <span className={cs.accuracy >= 80 ? 'text-emerald-400' : cs.accuracy >= 50 ? 'text-amber-400' : 'text-red-400'}>
                      {cs.accuracy.toFixed(1)}%
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Per-prompt details */}
      <div className="bg-surface rounded-lg p-4 space-y-2">
        <h4 className="text-sm font-medium text-fg-muted">Prompt Details</h4>
        <div className="space-y-1">
          {result.prompt_details.map(pd => (
            <div key={pd.id}>
              <button
                onClick={() => setExpandedPrompt(expandedPrompt === pd.id ? null : pd.id)}
                className="w-full flex items-center justify-between px-3 py-2 text-sm bg-elevated rounded hover:bg-overlay transition-colors text-left"
                data-testid={`prompt-toggle-${pd.id}`}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <span className={`shrink-0 w-2 h-2 rounded-full ${pd.correct ? 'bg-emerald-400' : 'bg-red-400'}`} />
                  <span className="text-fg truncate">{pd.prompt}</span>
                </div>
                <span className="text-xs text-fg-muted shrink-0 ml-2">{pd.latency_ms}ms</span>
              </button>
              {expandedPrompt === pd.id && (
                <div className="mt-1 ml-5 space-y-2 text-xs animate-fade-in">
                  <div className="bg-deep rounded p-2">
                    <span className="text-fg-muted">Category:</span>{' '}
                    <span className="text-fg-secondary">{pd.category}</span>
                  </div>
                  <div className="bg-deep rounded p-2">
                    <span className="text-fg-muted">Expected:</span>
                    <pre className="text-fg-secondary mt-1 whitespace-pre-wrap">{pd.expected}</pre>
                  </div>
                  <div className="bg-deep rounded p-2">
                    <span className="text-fg-muted">Actual:</span>
                    <pre className="text-fg-secondary mt-1 whitespace-pre-wrap">{pd.actual}</pre>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
