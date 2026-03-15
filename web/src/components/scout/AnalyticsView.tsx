import type { ScoutAnalytics } from '../../hooks/useScout';

interface AnalyticsViewProps {
  analytics: ScoutAnalytics;
}

export function AnalyticsView({ analytics }: AnalyticsViewProps) {
  const maxWeekCount = Math.max(...analytics.weekly_trend.map(w => w.count), 1);

  return (
    <div className="space-y-6" data-testid="analytics-view">
      {/* Summary stats */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="bg-surface rounded-lg p-4" data-testid="stat-total-leads">
          <div className="text-xs text-fg-muted mb-1">Total Leads</div>
          <div className="text-2xl font-bold text-fg">{analytics.total_leads}</div>
        </div>
        <div className="bg-surface rounded-lg p-4" data-testid="stat-active-leads">
          <div className="text-xs text-fg-muted mb-1">Active Leads</div>
          <div className="text-2xl font-bold text-emerald-400">{analytics.active_leads}</div>
        </div>
        <div className="bg-surface rounded-lg p-4" data-testid="stat-sources">
          <div className="text-xs text-fg-muted mb-1">Sources</div>
          <div className="text-2xl font-bold text-fg">{Object.keys(analytics.by_source).length}</div>
        </div>
        <div className="bg-surface rounded-lg p-4" data-testid="stat-types">
          <div className="text-xs text-fg-muted mb-1">Types</div>
          <div className="text-2xl font-bold text-fg">{Object.keys(analytics.by_type).length}</div>
        </div>
      </div>

      {/* By Type */}
      <div className="bg-surface rounded-lg p-4" data-testid="analytics-by-type">
        <h3 className="text-sm font-medium text-fg-muted mb-3">By Type</h3>
        <div className="space-y-2">
          {Object.entries(analytics.by_type).map(([type, count]) => (
            <div key={type} className="flex items-center justify-between text-sm">
              <span className="text-fg capitalize">{type}</span>
              <span className="text-fg-muted">{count}</span>
            </div>
          ))}
        </div>
      </div>

      {/* By Source */}
      <div className="bg-surface rounded-lg p-4" data-testid="analytics-by-source">
        <h3 className="text-sm font-medium text-fg-muted mb-3">By Source</h3>
        <div className="space-y-2">
          {Object.entries(analytics.by_source).map(([source, count]) => (
            <div key={source} className="flex items-center justify-between text-sm">
              <span className="text-fg">{source}</span>
              <span className="text-fg-muted">{count}</span>
            </div>
          ))}
        </div>
      </div>

      {/* By Stage */}
      <div className="bg-surface rounded-lg p-4" data-testid="analytics-by-stage">
        <h3 className="text-sm font-medium text-fg-muted mb-3">By Stage</h3>
        <div className="space-y-2">
          {Object.entries(analytics.by_stage).map(([stage, count]) => (
            <div key={stage} className="flex items-center justify-between text-sm">
              <span className="text-fg capitalize">{stage}</span>
              <span className="text-fg-muted">{count}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Conversion */}
      <div className="bg-surface rounded-lg p-4" data-testid="analytics-conversion">
        <h3 className="text-sm font-medium text-fg-muted mb-3">Conversion Funnel</h3>
        <div className="space-y-2">
          {analytics.conversion.map(c => (
            <div key={c.stage} className="flex items-center gap-3 text-sm">
              <span className="text-fg capitalize w-28 truncate">{c.stage}</span>
              <div className="flex-1 h-2 bg-elevated rounded-full overflow-hidden">
                <div className="h-full bg-soul rounded-full transition-all" style={{ width: `${c.rate}%` }} />
              </div>
              <span className="text-fg-muted text-xs w-16 text-right">{c.count} ({c.rate}%)</span>
            </div>
          ))}
        </div>
      </div>

      {/* Weekly Trend */}
      <div className="bg-surface rounded-lg p-4" data-testid="analytics-weekly-trend">
        <h3 className="text-sm font-medium text-fg-muted mb-3">Weekly Trend</h3>
        <div className="flex items-end gap-1 h-24">
          {analytics.weekly_trend.map(w => (
            <div key={w.week} className="flex-1 flex flex-col items-center gap-1">
              <div
                className="w-full bg-soul/60 rounded-t transition-all"
                style={{ height: `${(w.count / maxWeekCount) * 100}%`, minHeight: w.count > 0 ? '4px' : '0px' }}
              />
              <span className="text-[9px] text-fg-muted truncate w-full text-center">{w.week}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
