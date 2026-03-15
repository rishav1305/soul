import type { ScoutOptimization } from '../../hooks/useScout';

interface ApprovalDialogProps {
  optimization: ScoutOptimization;
  onApprove: (id: number) => void;
  onReject: (id: number) => void;
  onClose: () => void;
}

export function ApprovalDialog({ optimization, onApprove, onReject, onClose }: ApprovalDialogProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" data-testid="approval-dialog">
      <div className="bg-surface rounded-lg p-6 max-w-lg w-full mx-4 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-fg">Review Optimization</h3>
          <button onClick={onClose} className="text-fg-muted hover:text-fg text-sm transition-colors" data-testid="approval-dialog-close">Close</button>
        </div>

        <div className="space-y-3">
          <div>
            <span className="text-xs text-fg-muted">Field</span>
            <div className="text-sm text-fg font-medium">{optimization.field}</div>
          </div>
          <div>
            <span className="text-xs text-fg-muted">Type</span>
            <div className="text-sm text-fg capitalize">{optimization.type}</div>
          </div>
          <div>
            <span className="text-xs text-fg-muted">Reason</span>
            <p className="text-sm text-fg">{optimization.reason}</p>
          </div>
        </div>

        {/* Before / After diff */}
        <div className="grid grid-cols-2 gap-3">
          <div className="bg-red-500/5 rounded-lg p-3 border border-red-500/20">
            <span className="text-xs text-red-400 font-medium">Current</span>
            <p className="text-sm text-fg mt-1 whitespace-pre-wrap">{optimization.current}</p>
          </div>
          <div className="bg-emerald-500/5 rounded-lg p-3 border border-emerald-500/20">
            <span className="text-xs text-emerald-400 font-medium">Suggested</span>
            <p className="text-sm text-fg mt-1 whitespace-pre-wrap">{optimization.suggested}</p>
          </div>
        </div>

        {/* Actions */}
        <div className="flex justify-end gap-2 pt-2 border-t border-border-subtle">
          <button
            onClick={() => onReject(optimization.id)}
            className="px-4 py-1.5 text-sm rounded bg-red-500/20 text-red-400 hover:bg-red-500/30 transition-colors"
            data-testid="approval-reject"
          >
            Reject
          </button>
          <button
            onClick={() => onApprove(optimization.id)}
            className="px-4 py-1.5 text-sm rounded bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors"
            data-testid="approval-approve"
          >
            Approve
          </button>
        </div>
      </div>
    </div>
  );
}
