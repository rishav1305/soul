import type { StageNotification, TaskStage } from '../lib/types.ts';

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'text-stage-backlog bg-stage-backlog/15',
  brainstorm: 'text-stage-brainstorm bg-stage-brainstorm/15',
  active: 'text-stage-active bg-stage-active/15',
  blocked: 'text-stage-blocked bg-stage-blocked/15',
  validation: 'text-stage-validation bg-stage-validation/15',
  done: 'text-stage-done bg-stage-done/15',
};

interface ToastStackProps {
  toasts: StageNotification[];
  onDismiss: (id: string) => void;
}

export default function ToastStack({ toasts, onDismiss }: ToastStackProps) {
  if (toasts.length === 0) return null;

  return (
    <div className="fixed top-4 right-4 z-[9000] flex flex-col gap-2 pointer-events-none">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className="pointer-events-auto flex items-center gap-3 bg-surface border border-border-default rounded-xl px-4 py-3 shadow-xl shadow-black/40 animate-slide-right min-w-[280px] max-w-[360px]"
        >
          {/* Soul icon */}
          <span className="text-soul text-base shrink-0">&#9670;</span>

          {/* Content */}
          <div className="flex-1 min-w-0">
            <p className="text-sm font-display font-medium text-fg truncate">{toast.taskTitle}</p>
            <div className="flex items-center gap-1.5 mt-1">
              <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${STAGE_COLORS[toast.fromStage]}`}>
                {toast.fromStage}
              </span>
              <svg width="10" height="10" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" className="text-fg-muted shrink-0">
                <path d="M3 8h10M9 4l4 4-4 4" />
              </svg>
              <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${STAGE_COLORS[toast.toStage]}`}>
                {toast.toStage}
              </span>
            </div>
          </div>

          {/* Dismiss */}
          <button
            type="button"
            onClick={() => onDismiss(toast.id)}
            className="w-6 h-6 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-elevated transition-colors shrink-0 cursor-pointer"
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
