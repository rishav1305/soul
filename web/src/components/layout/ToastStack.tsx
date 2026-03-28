import type { StageNotification, TaskStage } from '../../lib/types.ts';

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'text-stage-backlog bg-stage-backlog/15',
  brainstorm: 'text-stage-brainstorm bg-stage-brainstorm/15',
  active: 'text-stage-active bg-stage-active/15',
  blocked: 'text-stage-blocked bg-stage-blocked/15',
  validation: 'text-stage-validation bg-stage-validation/15',
  done: 'text-stage-done bg-stage-done/15',
};

interface ToastStackProps {
  notifications: StageNotification[];
  onDismiss: (id: string) => void;
}

function relTime(date: Date | string): string {
  const d = date instanceof Date ? date : new Date(date);
  const diff = Math.floor((Date.now() - d.getTime()) / 1000);
  if (diff < 5) return 'just now';
  if (diff < 60) return `${diff}s ago`;
  return `${Math.floor(diff / 60)}m ago`;
}

export default function ToastStack({ notifications, onDismiss }: ToastStackProps) {
  if (notifications.length === 0) return null;

  return (
    <div data-testid="toast-stack" role="status" aria-live="polite" className="fixed top-4 right-4 z-[9000] flex flex-col gap-2 pointer-events-none">
      {notifications.map((n) => (
        <div
          key={n.id}
          data-testid={`toast-${n.id}`}
          className="pointer-events-auto animate-fade-in flex items-start gap-3 w-80 bg-surface border border-border-default rounded-xl px-4 py-3 shadow-xl shadow-black/40"
        >
          {/* Soul diamond */}
          <span className="text-soul text-lg shrink-0 mt-0.5">&#9670;</span>

          <div className="flex-1 min-w-0">
            <div className="text-xs font-display font-semibold text-fg mb-1 truncate">
              {n.taskTitle}
            </div>
            <div className="flex items-center gap-1.5 flex-wrap">
              <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${STAGE_COLORS[n.fromStage]}`}>
                {n.fromStage}
              </span>
              <svg width="10" height="10" viewBox="0 0 10 10" fill="none" className="text-fg-muted shrink-0">
                <path d="M2 5h6M6 3l2 2-2 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${STAGE_COLORS[n.toStage]}`}>
                {n.toStage}
              </span>
              <span className="text-[10px] text-fg-muted ml-auto">{relTime(n.time)}</span>
            </div>
          </div>

          {/* Dismiss */}
          <button
            type="button"
            onClick={() => onDismiss(n.id)}
            data-testid={`toast-dismiss-${n.id}`}
            aria-label="Dismiss notification"
            className="text-fg-muted hover:text-fg transition-colors text-sm leading-none mt-0.5 cursor-pointer shrink-0"
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
