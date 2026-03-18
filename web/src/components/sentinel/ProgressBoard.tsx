import type { Progress } from '../../hooks/useSentinel';

interface ProgressBoardProps {
  progress: Progress;
}

function scoreColor(completed: number, total: number): string {
  if (total === 0) return 'text-fg-muted';
  const pct = completed / total;
  if (pct >= 0.8) return 'text-emerald-400';
  if (pct >= 0.5) return 'text-amber-400';
  if (pct > 0) return 'text-orange-400';
  return 'text-fg-muted';
}

function barColor(points: number, maxPoints: number): string {
  if (maxPoints === 0) return 'bg-overlay';
  const pct = points / maxPoints;
  if (pct >= 0.8) return 'bg-emerald-400';
  if (pct >= 0.5) return 'bg-amber-400';
  if (pct > 0) return 'bg-orange-400';
  return 'bg-overlay';
}

export function ProgressBoard({ progress }: ProgressBoardProps) {
  const completionPct = progress.total_challenges > 0
    ? Math.round((progress.completed / progress.total_challenges) * 100)
    : 0;

  const categoryEntries = Object.entries(progress.categories);
  const maxCategoryVal = categoryEntries.length > 0
    ? Math.max(...categoryEntries.map(([, v]) => v))
    : 0;

  return (
    <div className="space-y-6" data-testid="progress-board">
      {/* Summary cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div className="bg-surface rounded-lg p-4 text-center" data-testid="total-points">
          <div className="text-2xl font-bold text-soul">{progress.total_points}</div>
          <div className="text-xs text-fg-muted mt-1">Total Points</div>
        </div>
        <div className="bg-surface rounded-lg p-4 text-center" data-testid="challenges-completed">
          <div className={`text-2xl font-bold ${scoreColor(progress.completed, progress.total_challenges)}`}>
            {progress.completed}/{progress.total_challenges}
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
          <span className="text-xs text-fg-secondary">{progress.completed} of {progress.total_challenges}</span>
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
        {categoryEntries.length === 0 ? (
          <div className="text-xs text-fg-muted">No category data available.</div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {categoryEntries.map(([category, points]) => {
              const catPct = maxCategoryVal > 0 ? Math.round((points / maxCategoryVal) * 100) : 0;
              return (
                <div key={category} className="bg-surface rounded-lg p-3 space-y-2" data-testid={`category-${category}`}>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-fg capitalize">{category}</span>
                    <span className="text-xs text-fg-secondary font-mono">{points} pts</span>
                  </div>
                  <div className="w-full h-1.5 bg-elevated rounded-full overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all ${barColor(points, maxCategoryVal)}`}
                      style={{ width: `${catPct}%` }}
                    />
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
