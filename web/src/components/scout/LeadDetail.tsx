import { useState } from 'react';
import type { ScoutLead } from '../../hooks/useScout';

const STAGES = ['discovered', 'applied', 'screening', 'interviewing', 'negotiating', 'closed'] as const;

interface LeadDetailProps {
  lead: ScoutLead;
  onUpdate: (id: number, data: Partial<ScoutLead>) => Promise<void>;
  onClose: () => void;
}

export function LeadDetail({ lead, onUpdate, onClose }: LeadDetailProps) {
  const [editing, setEditing] = useState(false);
  const [notes, setNotes] = useState(lead.notes);
  const [saving, setSaving] = useState(false);

  const currentStageIdx = STAGES.indexOf(lead.stage as typeof STAGES[number]);
  const nextStage = currentStageIdx >= 0 && currentStageIdx < STAGES.length - 1 ? STAGES[currentStageIdx + 1] : null;

  const handleAdvance = async () => {
    if (!nextStage) return;
    setSaving(true);
    try {
      await onUpdate(lead.id, { stage: nextStage });
    } finally {
      setSaving(false);
    }
  };

  const handleSaveNotes = async () => {
    setSaving(true);
    try {
      await onUpdate(lead.id, { notes });
      setEditing(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="bg-surface rounded-lg p-4 space-y-4" data-testid="lead-detail">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-fg">{lead.title}</h3>
        <button onClick={onClose} className="text-fg-muted hover:text-fg text-sm transition-colors" data-testid="lead-detail-close">Close</button>
      </div>

      <div className="grid grid-cols-2 gap-3 text-sm">
        <div>
          <span className="text-fg-muted text-xs">Company</span>
          <div className="text-fg">{lead.company || '-'}</div>
        </div>
        <div>
          <span className="text-fg-muted text-xs">Type</span>
          <div className="text-fg capitalize">{lead.type}</div>
        </div>
        <div>
          <span className="text-fg-muted text-xs">Source</span>
          <div className="text-fg">{lead.source}</div>
        </div>
        <div>
          <span className="text-fg-muted text-xs">Compensation</span>
          <div className="text-fg">{lead.compensation || '-'}</div>
        </div>
        <div>
          <span className="text-fg-muted text-xs">Contact</span>
          <div className="text-fg">{lead.contact || '-'}</div>
        </div>
        <div>
          <span className="text-fg-muted text-xs">Location</span>
          <div className="text-fg">{lead.location || '-'}</div>
        </div>
      </div>

      {/* Notes */}
      <div>
        <div className="flex items-center justify-between mb-1">
          <span className="text-fg-muted text-xs">Notes</span>
          <button
            onClick={() => setEditing(!editing)}
            className="text-xs text-soul hover:text-soul/80 transition-colors"
            data-testid="lead-detail-edit-notes"
          >
            {editing ? 'Cancel' : 'Edit'}
          </button>
        </div>
        {editing ? (
          <div className="space-y-2">
            <textarea
              value={notes}
              onChange={e => setNotes(e.target.value)}
              className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-soul/50 resize-none"
              rows={3}
              data-testid="lead-detail-notes-input"
            />
            <button
              onClick={handleSaveNotes}
              disabled={saving}
              className="px-3 py-1 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
              data-testid="lead-detail-save-notes"
            >
              {saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        ) : (
          <div className="text-sm text-fg">{lead.notes || 'No notes'}</div>
        )}
      </div>

      {/* Stage advancement */}
      <div className="flex items-center gap-2 pt-2 border-t border-border-subtle">
        <span className="text-xs text-fg-muted">Stage: <span className="text-fg capitalize">{lead.stage}</span></span>
        {nextStage && (
          <button
            onClick={handleAdvance}
            disabled={saving}
            className="ml-auto px-3 py-1 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 disabled:opacity-50 transition-colors"
            data-testid="lead-detail-advance"
          >
            {saving ? 'Advancing...' : `Advance to ${nextStage}`}
          </button>
        )}
      </div>
    </div>
  );
}
