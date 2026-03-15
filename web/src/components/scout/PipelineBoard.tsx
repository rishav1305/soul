import { useState } from 'react';
import type { ScoutLead } from '../../hooks/useScout';
import { LeadCard } from './LeadCard';

const PIPELINE_STAGES = [
  { stage: 'discovered', label: 'Discovered', color: 'border-zinc-500' },
  { stage: 'applied', label: 'Applied', color: 'border-blue-500' },
  { stage: 'screening', label: 'Screening', color: 'border-amber-500' },
  { stage: 'interviewing', label: 'Interviewing', color: 'border-purple-500' },
  { stage: 'negotiating', label: 'Negotiating', color: 'border-cyan-500' },
  { stage: 'closed', label: 'Closed', color: 'border-emerald-500' },
];

const LEAD_TYPES = ['all', 'job', 'freelance', 'contract', 'consulting', 'product-dev'] as const;

interface PipelineBoardProps {
  leads: ScoutLead[];
  onSelectLead: (lead: ScoutLead) => void;
}

export function PipelineBoard({ leads, onSelectLead }: PipelineBoardProps) {
  const [typeFilter, setTypeFilter] = useState<string>('all');

  const filtered = typeFilter === 'all' ? leads : leads.filter(l => l.type === typeFilter);

  return (
    <div className="space-y-4" data-testid="pipeline-board">
      {/* Type filter */}
      <div className="flex gap-1 flex-wrap" data-testid="pipeline-type-filter">
        {LEAD_TYPES.map(type => (
          <button
            key={type}
            onClick={() => setTypeFilter(type)}
            className={`px-2.5 py-1 text-xs rounded transition-colors capitalize ${
              typeFilter === type ? 'bg-soul/20 text-soul' : 'bg-elevated text-fg-muted hover:text-fg'
            }`}
            data-testid={`filter-type-${type}`}
          >
            {type === 'all' ? 'All' : type}
          </button>
        ))}
      </div>

      {/* Kanban columns */}
      <div className="flex gap-4 overflow-x-auto pb-2">
        {PIPELINE_STAGES.map(({ stage, label, color }) => {
          const stageleads = filtered.filter(l => l.stage === stage);
          return (
            <div key={stage} className={`w-56 flex-shrink-0 flex flex-col border-t-2 ${color} rounded-lg bg-deep/50`} data-testid={`pipeline-column-${stage}`}>
              <div className="flex items-center justify-between px-3 py-2">
                <span className="text-xs font-medium text-fg-muted uppercase tracking-wider">{label}</span>
                <span className="text-xs text-fg-muted">{stageleads.length}</span>
              </div>
              <div className="flex-1 overflow-y-auto px-2 pb-2 space-y-2">
                {stageleads.map(lead => (
                  <LeadCard key={lead.id} lead={lead} onClick={onSelectLead} />
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
