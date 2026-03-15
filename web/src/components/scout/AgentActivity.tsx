import type { ScoutSweepStatus } from '../../hooks/useScout';

interface AgentActivityProps {
  sweepStatus: ScoutSweepStatus | null;
}

export function AgentActivity({ sweepStatus }: AgentActivityProps) {
  if (!sweepStatus || !sweepStatus.running) {
    return (
      <div className="bg-surface rounded-lg p-4" data-testid="agent-activity">
        <h3 className="text-sm font-medium text-fg-muted mb-2">Agent Activity</h3>
        <div className="text-xs text-fg-muted">No active agent runs</div>
      </div>
    );
  }

  return (
    <div className="bg-surface rounded-lg p-4 space-y-3" data-testid="agent-activity">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-fg-muted">Agent Activity</h3>
        <span className="px-2 py-0.5 text-[10px] rounded-full bg-blue-500/20 text-blue-400 animate-pulse">Running</span>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm">
          <span className="text-fg-muted">Platforms</span>
          <span className="text-fg">{sweepStatus.platforms.join(', ')}</span>
        </div>
        <div className="flex items-center justify-between text-sm">
          <span className="text-fg-muted">Results Found</span>
          <span className="text-fg">{sweepStatus.results_found}</span>
        </div>
        <div className="flex items-center justify-between text-sm">
          <span className="text-fg-muted">Started</span>
          <span className="text-fg">{new Date(sweepStatus.started_at).toLocaleTimeString()}</span>
        </div>
      </div>

      {/* Progress bar */}
      <div className="space-y-1">
        <div className="flex items-center justify-between text-xs text-fg-muted">
          <span>Progress</span>
          <span>{sweepStatus.progress}%</span>
        </div>
        <div className="w-full h-2 bg-elevated rounded-full overflow-hidden">
          <div
            className="h-full bg-blue-500 rounded-full transition-all"
            style={{ width: `${sweepStatus.progress}%` }}
            data-testid="agent-progress-bar"
          />
        </div>
      </div>
    </div>
  );
}
