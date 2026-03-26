import { useMemo } from 'react';
import type { ScoutLead } from '../../hooks/useScout';
import { GateAction } from './GateAction';

interface PriorityItem {
  id: number;
  leadId: number;
  type: 'prepare' | 'followup' | 'review' | 'approve' | 'stale';
  pipeline: string;
  title: string;
  company: string;
  stage: string;
  urgency: 'high' | 'medium' | 'low';
  description: string;
  actions: string[];
}

const ACTION_STAGES = new Set([
  'qualified', 'proposal-ready', 'screening', 'interviewing', 'negotiating',
]);

const urgencyOrder: Record<string, number> = {
  high: 0,
  medium: 1,
  low: 2,
};

const urgencyDot: Record<string, string> = {
  high: 'bg-red-400',
  medium: 'bg-amber-400',
  low: 'bg-zinc-400',
};

const urgencyLabel: Record<string, string> = {
  high: 'text-red-400',
  medium: 'text-amber-400',
  low: 'text-zinc-400',
};

const pipelineBadge: Record<string, string> = {
  job: 'bg-blue-500/20 text-blue-400',
  freelance: 'bg-purple-500/20 text-purple-400',
  contract: 'bg-cyan-500/20 text-cyan-400',
  consulting: 'bg-amber-500/20 text-amber-400',
  'product-dev': 'bg-emerald-500/20 text-emerald-400',
};

function getActionsForType(type: PriorityItem['type']): string[] {
  switch (type) {
    case 'prepare':
      return ['approve', 'edit', 'skip'];
    case 'followup':
      return ['send', 'edit', 'skip'];
    case 'review':
      return ['approve', 'reject', 'edit'];
    case 'approve':
      return ['approve', 'reject'];
    case 'stale':
      return ['send', 'edit', 'skip'];
  }
}

function computePriorityItems(leads: ScoutLead[]): PriorityItem[] {
  const now = Date.now();
  const items: PriorityItem[] = [];
  let nextId = 1;

  for (const lead of leads) {
    // Skip closed/rejected leads
    if (lead.stage === 'closed' || lead.stage === 'rejected') continue;

    const updatedTime = new Date(lead.updated_at).getTime();
    const daysSinceUpdate = (now - updatedTime) / (24 * 60 * 60 * 1000);

    // High-match leads at action stages
    if (lead.match_score >= 70 && ACTION_STAGES.has(lead.stage)) {
      const type = lead.stage === 'proposal-ready' ? 'approve' as const
        : lead.stage === 'qualified' ? 'prepare' as const
        : 'review' as const;
      items.push({
        id: nextId++,
        leadId: lead.id,
        type,
        pipeline: lead.type,
        title: lead.title,
        company: lead.company,
        stage: lead.stage,
        urgency: lead.match_score >= 85 ? 'high' : 'medium',
        description: `${lead.match_score}% match — ${lead.stage} stage, needs ${type}`,
        actions: getActionsForType(type),
      });
      continue;
    }

    // Stale leads (7+ days without update)
    if (daysSinceUpdate >= 7) {
      items.push({
        id: nextId++,
        leadId: lead.id,
        type: 'stale',
        pipeline: lead.type,
        title: lead.title,
        company: lead.company,
        stage: lead.stage,
        urgency: daysSinceUpdate >= 14 ? 'high' : 'medium',
        description: `No update for ${Math.floor(daysSinceUpdate)} days — at ${lead.stage}`,
        actions: getActionsForType('stale'),
      });
      continue;
    }

    // Follow-up candidates: medium-score leads at early stages
    if (lead.match_score >= 40 && lead.match_score < 70 && daysSinceUpdate >= 3) {
      items.push({
        id: nextId++,
        leadId: lead.id,
        type: 'followup',
        pipeline: lead.type,
        title: lead.title,
        company: lead.company,
        stage: lead.stage,
        urgency: 'low',
        description: `${lead.match_score}% match — consider follow-up at ${lead.stage}`,
        actions: getActionsForType('followup'),
      });
    }
  }

  // Sort by urgency
  items.sort((a, b) => (urgencyOrder[a.urgency] ?? 2) - (urgencyOrder[b.urgency] ?? 2));

  return items;
}

interface PriorityQueueProps {
  leads: ScoutLead[];
  onAction?: (leadId: number, action: string) => void;
}

export function PriorityQueue({ leads, onAction }: PriorityQueueProps) {
  const items = useMemo(() => computePriorityItems(leads), [leads]);

  const highCount = items.filter(i => i.urgency === 'high').length;
  const mediumCount = items.filter(i => i.urgency === 'medium').length;
  const lowCount = items.filter(i => i.urgency === 'low').length;

  const handleAction = (leadId: number, action: string) => {
    onAction?.(leadId, action);
  };

  return (
    <div className="space-y-4" data-testid="priority-queue">
      {/* Summary bar */}
      <div className="flex items-center gap-4 text-xs" data-testid="priority-summary">
        <span className="text-fg-muted">{items.length} items</span>
        {highCount > 0 && (
          <span className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-red-400" />
            <span className="text-red-400">{highCount} high</span>
          </span>
        )}
        {mediumCount > 0 && (
          <span className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-amber-400" />
            <span className="text-amber-400">{mediumCount} medium</span>
          </span>
        )}
        {lowCount > 0 && (
          <span className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-zinc-400" />
            <span className="text-zinc-400">{lowCount} low</span>
          </span>
        )}
      </div>

      {/* Items list */}
      {items.length === 0 ? (
        <div className="bg-surface rounded-lg p-8 text-center" data-testid="priority-empty">
          <div className="text-fg-muted text-sm">No pending action items</div>
          <div className="text-fg-muted text-xs mt-1">All leads are up to date</div>
        </div>
      ) : (
        <div className="space-y-2">
          {items.map(item => (
            <div
              key={item.id}
              className="bg-surface rounded-lg p-3 space-y-2 hover:bg-elevated/50 transition-colors"
              data-testid={`priority-item-${item.id}`}
            >
              {/* Header row */}
              <div className="flex items-start justify-between gap-2">
                <div className="flex items-center gap-2 min-w-0">
                  <span className={`shrink-0 w-2 h-2 rounded-full ${urgencyDot[item.urgency]}`} data-testid={`priority-urgency-${item.id}`} />
                  <span className="text-sm font-medium text-fg truncate">{item.title}</span>
                </div>
                <div className="flex items-center gap-1.5 shrink-0">
                  <span className={`px-1.5 py-0.5 text-[10px] rounded-full capitalize ${pipelineBadge[item.pipeline] ?? 'bg-overlay text-fg-secondary'}`}>
                    {item.pipeline}
                  </span>
                  <span className={`text-[10px] capitalize ${urgencyLabel[item.urgency]}`}>
                    {item.urgency}
                  </span>
                </div>
              </div>

              {/* Details */}
              <div className="flex items-center gap-2 text-xs">
                {item.company && <span className="text-fg-muted">{item.company}</span>}
                <span className="text-fg-muted capitalize">{item.stage}</span>
              </div>

              <div className="text-xs text-fg-secondary">{item.description}</div>

              {/* Gate actions */}
              <GateAction
                actions={item.actions}
                onAction={(action) => handleAction(item.leadId, action)}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
