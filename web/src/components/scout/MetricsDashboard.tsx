import { useState, useEffect, useCallback } from 'react';
import { api } from '../../lib/api';

interface MetricsDashboardProps {
  onRefresh?: () => void;
}

interface TopPost {
  topic: string;
  impressions: number;
}

interface MetricsResult {
  total_posts: number;
  total_impressions: number;
  avg_engagement_rate: number;
  top_performing: TopPost[];
  analysis: string;
  recommendations: string[];
}

type PlatformFilter = 'all' | 'linkedin' | 'x';

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toLocaleString();
}

export function MetricsDashboard({ onRefresh }: MetricsDashboardProps) {
  const [platform, setPlatform] = useState<PlatformFilter>('all');
  const [metrics, setMetrics] = useState<MetricsResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchMetrics = useCallback(async (p: PlatformFilter) => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.post<MetricsResult>('/api/ai/content-metrics', {
        platform: p === 'all' ? undefined : p,
      });
      setMetrics(result);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchMetrics(platform);
  }, [platform, fetchMetrics]);

  const handleRefresh = () => {
    fetchMetrics(platform);
    onRefresh?.();
  };

  const filters: { label: string; value: PlatformFilter }[] = [
    { label: 'All', value: 'all' },
    { label: 'LinkedIn', value: 'linkedin' },
    { label: 'X', value: 'x' },
  ];

  return (
    <div className="space-y-4" data-testid="metrics-dashboard">
      {/* Filter bar */}
      <div className="flex items-center justify-between">
        <div className="flex gap-1" data-testid="metrics-platform-filter">
          {filters.map(f => (
            <button
              key={f.value}
              onClick={() => setPlatform(f.value)}
              className={`px-3 py-1.5 text-xs rounded transition-colors ${
                platform === f.value
                  ? 'bg-soul text-deep font-medium'
                  : 'bg-elevated text-fg-secondary hover:bg-overlay'
              }`}
              data-testid={`metrics-filter-${f.value}`}
            >
              {f.label}
            </button>
          ))}
        </div>
        <button
          onClick={handleRefresh}
          className="px-3 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors"
          data-testid="metrics-refresh-btn"
        >
          Refresh
        </button>
      </div>

      {error && (
        <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="metrics-error">
          {error}
        </div>
      )}

      {loading && (
        <div className="text-center py-8 text-fg-muted" data-testid="metrics-loading">
          Generating...
        </div>
      )}

      {metrics && !loading && (
        <>
          {/* Stat cards */}
          <div className="grid grid-cols-3 gap-3" data-testid="metrics-stat-cards">
            <div className="bg-surface rounded-lg p-4" data-testid="metrics-stat-posts">
              <div className="text-2xl font-bold text-fg">{formatNumber(metrics.total_posts)}</div>
              <div className="text-xs text-fg-muted mt-1">Total Posts</div>
            </div>
            <div className="bg-surface rounded-lg p-4" data-testid="metrics-stat-impressions">
              <div className="text-2xl font-bold text-fg">{formatNumber(metrics.total_impressions)}</div>
              <div className="text-xs text-fg-muted mt-1">Total Impressions</div>
            </div>
            <div className="bg-surface rounded-lg p-4" data-testid="metrics-stat-engagement">
              <div className="text-2xl font-bold text-soul">{metrics.avg_engagement_rate.toFixed(1)}%</div>
              <div className="text-xs text-fg-muted mt-1">Avg Engagement Rate</div>
            </div>
          </div>

          {/* Top Performing */}
          <div className="bg-surface rounded-lg p-4" data-testid="metrics-top-performing">
            <h3 className="text-sm font-medium text-fg-muted mb-3">Top Performing</h3>
            {metrics.top_performing.length === 0 ? (
              <div className="text-xs text-fg-muted">No data yet</div>
            ) : (
              <div className="space-y-2">
                {metrics.top_performing.slice(0, 3).map((post, i) => (
                  <div key={i} className="flex items-center justify-between bg-elevated rounded-lg p-3" data-testid={`metrics-top-post-${i}`}>
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-bold text-soul w-5">#{i + 1}</span>
                      <span className="text-sm text-fg">{post.topic}</span>
                    </div>
                    <span className="text-xs text-fg-muted">{formatNumber(post.impressions)} impressions</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* AI Analysis */}
          <div className="bg-surface rounded-lg p-4" data-testid="metrics-analysis">
            <h3 className="text-sm font-medium text-fg-muted mb-2">AI Analysis</h3>
            <p className="text-sm text-fg">{metrics.analysis}</p>
          </div>

          {/* Recommendations */}
          <div className="bg-surface rounded-lg p-4" data-testid="metrics-recommendations">
            <h3 className="text-sm font-medium text-fg-muted mb-2">Recommendations</h3>
            {metrics.recommendations.length === 0 ? (
              <div className="text-xs text-fg-muted">No recommendations</div>
            ) : (
              <ul className="space-y-1.5">
                {metrics.recommendations.map((rec, i) => (
                  <li key={i} className="text-sm text-fg flex items-start gap-2" data-testid={`metrics-recommendation-${i}`}>
                    <span className="text-soul text-xs mt-0.5">&#x2022;</span>
                    <span>{rec}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </>
      )}
    </div>
  );
}
