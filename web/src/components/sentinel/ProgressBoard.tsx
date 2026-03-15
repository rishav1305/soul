import type { SentinelProgress } from '../../hooks/useSentinel';

interface ProgressBoardProps {
  progress: SentinelProgress;
}

function scoreColor(completed: number, total: number): string {
  if (total === 0) return 'text-fg-muted';
  const pct = completed / total;
  if (pct >= 0.8) return 'text-emerald-400';
  if (pct >= 0.5) return 'text-amber-400';
  if (pct > 0) return 'text-orange-400';
  return 'text-fg-muted';
}

function pointsBadgeColor(points: number, maxPoints: number): string {
  if (maxPoints === 0) return 'bg-overlay text-fg-secondary';
  const pct = points / maxPoints;
  if (pct >= 0.8) return 'bg-emerald-500/20 text-emerald-400';
  if (pct >= 0.5) return 'bg-amber-500/20 text-amber-400';
  if (pct > 0) return 'bg-orange-500/20 text-orange-400';
  return 'bg-overlay text-fg-secondary';
}

export function ProgressBoard({ progress }: ProgressBoardProps) {
  const completionPct = progress.challengesTotal > 0
    ? Math.round((progress.challengesCompleted / progress.challengesTotal) * 100)
    : 0;

  return (
    <div className="space-y-6" data-testid="progress-board">
      {/* Summary cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div className="bg-surface rounded-lg p-4 text-center" data-testid="total-points">
          <div className="text-2xl font-bold text-soul">{progress.totalPoints}</div>
          <div className="text-xs text-fg-muted mt-1">Total Points</div>
        </div>
        <div className="bg-surface rounded-lg p-4 text-center" data-testid="challenges-completed">
          <div className={`text-2xl font-bold ${scoreColor(progress.challengesCompleted, progress.challengesTotal)}`}>
            {progress.challengesCompleted}/{progress.challengesTotal}
          </div>
          <div className="text-xs text-fg-muted mt-1">Challenges Completed</div>
        </div>
        <div className="bg-surface rounded-lg p-4 text-center" data-testid="completion-pct">
          <div className={`text-2xl font-bold ${completionPct >= 80 ? 'text-emerald-400' : completionPct >= 50 ? 'text-amber-400' : 'text-fg'}`}>
            {completionPct}%
          </div>
          <div className="text-xs text-fg-muted mt-1">Completion</div>
        </div>
      </div>

      {/* Progress bar */}
      <div className="bg-surface rounded-lg p-4 space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-sm text-fg-muted">Overall Progress</span>
          <span className="text-xs text-fg-secondary">{progress.challengesCompleted} of {progress.challengesTotal}</span>
        </div>
        <div className="w-full h-2 bg-elevated rounded-full overflow-hidden">
          <div
            className="h-full bg-soul rounded-full transition-all"
            style={{ width: `${completionPct}%` }}
            data-testid="progress-bar"
          />
        </div>
      </div>

      {/* Category breakdown */}
      <div className="space-y-3">
        <h3 className="text-sm font-medium text-fg">Category Breakdown</h3>
        {progress.categoryBreakdown.length === 0 ? (
          <div className="text-xs text-fg-muted">No category data available.</div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {progress.categoryBreakdown.map(cat => {
              const catPct = cat.total > 0 ? Math.round((cat.completed / cat.total) * 100) : 0;
              return (
                <div key={cat.category} className="bg-surface rounded-lg p-3 space-y-2" data-testid={`category-${cat.category}`}>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-fg capitalize">{cat.category}</span>
                    <span className={`px-2 py-0.5 text-[10px] rounded-full ${pointsBadgeColor(cat.points, cat.maxPoints)}`}>
                      {cat.points}/{cat.maxPoints} pts
                    </span>
                  </div>
                  <div className="w-full h-1.5 bg-elevated rounded-full overflow-hidden">
                    <div
                      className="h-full bg-soul rounded-full transition-all"
                      style={{ width: `${catPct}%` }}
                    />
                  </div>
                  <div className="text-[10px] text-fg-muted">
                    {cat.completed}/{cat.total} completed
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
