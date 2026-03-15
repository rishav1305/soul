import { useState } from 'react';
import type { BenchCategory, RunConfig } from '../../hooks/useBench';

interface BenchRunnerProps {
  categories: BenchCategory[];
  loading: boolean;
  onRun: (config: RunConfig) => Promise<void>;
}

export function BenchRunner({ categories, loading, onRun }: BenchRunnerProps) {
  const [endpoint, setEndpoint] = useState('');
  const [selectedCategories, setSelectedCategories] = useState<string[]>([]);
  const [gpu, setGpu] = useState(false);
  const [maxTokens, setMaxTokens] = useState(1024);

  const toggleCategory = (id: string) => {
    setSelectedCategories(prev =>
      prev.includes(id) ? prev.filter(c => c !== id) : [...prev, id]
    );
  };

  const handleStart = async () => {
    if (!endpoint.trim()) return;
    await onRun({
      endpoint: endpoint.trim(),
      categories: selectedCategories,
      gpu,
      max_tokens: maxTokens,
    });
  };

  return (
    <div className="space-y-4" data-testid="bench-runner">
      {/* Model endpoint */}
      <div className="bg-surface rounded-lg p-4 space-y-2">
        <label className="text-sm font-medium text-fg-muted">Model Endpoint URL</label>
        <input
          type="text"
          value={endpoint}
          onChange={e => setEndpoint(e.target.value)}
          placeholder="https://api.example.com/v1/chat"
          className="w-full px-3 py-2 text-sm bg-elevated border border-border-default rounded text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul"
          data-testid="model-endpoint"
        />
      </div>

      {/* Categories */}
      <div className="bg-surface rounded-lg p-4 space-y-2">
        <label className="text-sm font-medium text-fg-muted">Categories</label>
        {categories.length === 0 ? (
          <div className="text-xs text-fg-muted">No categories available.</div>
        ) : (
          <div className="flex flex-wrap gap-2">
            {categories.map(cat => (
              <label
                key={cat.id}
                className={`flex items-center gap-2 px-3 py-1.5 text-sm rounded cursor-pointer transition-colors ${
                  selectedCategories.includes(cat.id)
                    ? 'bg-soul/20 text-soul'
                    : 'bg-elevated text-fg-secondary hover:bg-overlay'
                }`}
                data-testid={`category-${cat.id}`}
              >
                <input
                  type="checkbox"
                  checked={selectedCategories.includes(cat.id)}
                  onChange={() => toggleCategory(cat.id)}
                  className="sr-only"
                />
                {cat.name} ({cat.prompt_count})
              </label>
            ))}
          </div>
        )}
      </div>

      {/* Settings */}
      <div className="bg-surface rounded-lg p-4 space-y-3">
        <div className="flex items-center justify-between">
          <label className="text-sm font-medium text-fg-muted">GPU Acceleration</label>
          <button
            onClick={() => setGpu(!gpu)}
            className={`relative w-10 h-5 rounded-full transition-colors ${gpu ? 'bg-emerald-500' : 'bg-elevated'}`}
            data-testid="gpu-toggle"
          >
            <span
              className={`absolute top-0.5 w-4 h-4 rounded-full bg-fg transition-transform ${gpu ? 'translate-x-5' : 'translate-x-0.5'}`}
            />
          </button>
        </div>
        <div className="space-y-1">
          <label className="text-sm font-medium text-fg-muted">Max Tokens</label>
          <input
            type="number"
            value={maxTokens}
            onChange={e => setMaxTokens(Number(e.target.value))}
            min={1}
            max={8192}
            className="w-full px-3 py-2 text-sm bg-elevated border border-border-default rounded text-fg focus:outline-none focus:border-soul"
            data-testid="max-tokens"
          />
        </div>
      </div>

      {/* Start button */}
      <button
        onClick={handleStart}
        disabled={loading || !endpoint.trim()}
        className="w-full px-4 py-2.5 text-sm font-medium rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors disabled:opacity-50"
        data-testid="start-benchmark"
      >
        {loading ? 'Running...' : 'Start Benchmark'}
      </button>

      {/* Progress indicator */}
      {loading && (
        <div className="stream-bar rounded" data-testid="bench-progress" />
      )}
    </div>
  );
}
