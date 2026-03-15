import type { ScoutLead } from '../../hooks/useScout';

const stageColor: Record<string, string> = {
  discovered: 'bg-zinc-500/20 text-zinc-400',
  applied: 'bg-blue-500/20 text-blue-400',
  screening: 'bg-amber-500/20 text-amber-400',
  interviewing: 'bg-purple-500/20 text-purple-400',
  negotiating: 'bg-cyan-500/20 text-cyan-400',
  closed: 'bg-emerald-500/20 text-emerald-400',
  rejected: 'bg-red-500/20 text-red-400',
};

interface LeadCardProps {
  lead: ScoutLead;
  onClick: (lead: ScoutLead) => void;
}

export function LeadCard({ lead, onClick }: LeadCardProps) {
  return (
    <button
      onClick={() => onClick(lead)}
      className="w-full bg-surface rounded-lg p-3 hover:bg-elevated transition-colors text-left space-y-2"
      data-testid={`lead-card-${lead.id}`}
    >
      <div className="flex items-start justify-between gap-2">
        <span className="text-sm font-medium text-fg truncate">{lead.title}</span>
        <span className={`shrink-0 px-1.5 py-0.5 text-[10px] rounded-full ${stageColor[lead.stage] ?? 'bg-overlay text-fg-secondary'}`}>
          {lead.stage}
        </span>
      </div>
      {lead.company && (
        <div className="text-xs text-fg-muted">{lead.company}</div>
      )}
      <div className="flex items-center justify-between text-xs text-fg-muted">
        <span>{lead.source}</span>
        <span className={`font-medium ${lead.match_score >= 80 ? 'text-emerald-400' : lead.match_score >= 50 ? 'text-amber-400' : 'text-fg-muted'}`}>
          {lead.match_score}%
        </span>
      </div>
    </button>
  );
}
