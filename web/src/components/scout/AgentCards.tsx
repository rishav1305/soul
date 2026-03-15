import type { ScoutAgentRun } from '../../hooks/useScout';

const statusColor: Record<string, string> = {
  running: 'bg-blue-500/20 text-blue-400',
  completed: 'bg-emerald-500/20 text-emerald-400',
  failed: 'bg-red-500/20 text-red-400',
};

interface AgentCardsProps {
  runs: ScoutAgentRun[];
}

export function AgentCards({ runs }: AgentCardsProps) {
  if (runs.length === 0) {
    return (
      <div className="bg-surface rounded-lg p-4" data-testid="agent-cards">
        <h3 className="text-sm font-medium text-fg-muted mb-2">Agent History</h3>
        <div className="text-xs text-fg-muted">No agent runs recorded</div>
      </div>
    );
  }

  return (
    <div data-testid="agent-cards">
      <h3 className="text-sm font-medium text-fg-muted mb-3">Agent History</h3>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
        {runs.map(run => (
          <div key={run.id} className="bg-surface rounded-lg p-3 space-y-2" data-testid={`agent-run-${run.id}`}>
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-fg">{run.platform}</span>
              <span className={`px-1.5 py-0.5 text-[10px] rounded-full ${statusColor[run.status] ?? 'bg-overlay text-fg-secondary'}`}>
                {run.status}
              </span>
            </div>
            <div className="text-xs text-fg-muted capitalize">{run.mode}</div>
            <div className="flex items-center justify-between text-xs text-fg-muted">
              <span>{new Date(run.created_at).toLocaleDateString()}</span>
              <span>{run.results_found} found</span>
            </div>
            {run.summary && (
              <p className="text-xs text-fg-muted line-clamp-2">{run.summary}</p>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
