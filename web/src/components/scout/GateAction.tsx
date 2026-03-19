interface GateActionProps {
  actions: string[];
  onAction: (action: string) => void;
  loading?: boolean;
  disabled?: boolean;
}

const actionStyles: Record<string, string> = {
  approve: 'bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30',
  edit: 'bg-blue-500/20 text-blue-400 hover:bg-blue-500/30',
  skip: 'bg-zinc-500/20 text-zinc-400 hover:bg-zinc-500/30',
  send: 'bg-soul/20 text-soul hover:bg-soul/30',
  reject: 'bg-red-500/20 text-red-400 hover:bg-red-500/30',
};

const defaultStyle = 'bg-elevated text-fg-muted hover:bg-overlay';

export function GateAction({ actions, onAction, loading = false, disabled = false }: GateActionProps) {
  return (
    <div className="flex flex-wrap gap-1.5" data-testid="gate-action-bar">
      {actions.map(action => (
        <button
          key={action}
          onClick={() => onAction(action)}
          disabled={loading || disabled}
          className={`px-2.5 py-1 text-xs rounded capitalize transition-colors disabled:opacity-50 ${actionStyles[action] ?? defaultStyle}`}
          data-testid={`gate-action-${action}`}
        >
          {loading ? 'Working...' : action}
        </button>
      ))}
    </div>
  );
}
