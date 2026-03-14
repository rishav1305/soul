import { formatRelativeTime } from '../lib/utils';
import { usePerformance } from '../hooks/usePerformance';
import type { TaskActivity } from '../lib/types';

interface ActivityTimelineProps {
  activities: TaskActivity[];
}

const EVENT_ICONS: Record<string, string> = {
  'task.created': 'text-green-400',
  'task.started': 'text-blue-400',
  'task.stopped': 'text-red-400',
  'task.blocked': 'text-red-400',
  'executor.classify': 'text-purple-400',
  'executor.worktree': 'text-cyan-400',
  'executor.agent_start': 'text-blue-400',
  'executor.agent_done': 'text-green-400',
  'executor.verify_l1': 'text-yellow-400',
  'executor.commit': 'text-green-400',
  'executor.complete': 'text-green-400',
  'agent.tool_call': 'text-fg-muted',
};

export function ActivityTimeline({ activities }: ActivityTimelineProps) {
  usePerformance('ActivityTimeline');
  if (activities.length === 0) {
    return <p className="text-sm text-fg-muted">No activity yet.</p>;
  }

  return (
    <div data-testid="activity-timeline" className="space-y-3">
      {activities.map(act => {
        const color = EVENT_ICONS[act.eventType] || 'text-fg-muted';
        let detail = '';
        try {
          const parsed = JSON.parse(act.data);
          detail = Object.entries(parsed)
            .map(([k, v]) => `${k}: ${typeof v === 'object' ? JSON.stringify(v) : v}`)
            .join(', ');
        } catch {
          detail = act.data;
        }

        return (
          <div key={act.id} data-testid={`activity-${act.id}`} className="flex gap-3 text-sm">
            <div className="shrink-0 mt-1">
              <span className={`inline-block w-2 h-2 rounded-full ${color.replace('text-', 'bg-')}`} />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className={`font-mono text-xs ${color}`}>{act.eventType}</span>
                <span className="text-[10px] text-fg-muted">{formatRelativeTime(act.createdAt)}</span>
              </div>
              {detail && (
                <p className="text-xs text-fg-muted mt-0.5 break-all">{detail}</p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
