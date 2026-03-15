import type { ScoutScoredLead, ScoutAgentRun, ScoutLead, ScoutAnalytics } from '../../hooks/useScout';
import { HotLeadsTable } from './HotLeadsTable';
import { AgentCards } from './AgentCards';
import { DigestSummary } from './DigestSummary';

interface IntelligenceViewProps {
  scoredLeads: ScoutScoredLead[];
  agentRuns: ScoutAgentRun[];
  leads: ScoutLead[];
  analytics: ScoutAnalytics | null;
}

export function IntelligenceView({ scoredLeads, agentRuns, leads, analytics }: IntelligenceViewProps) {
  return (
    <div className="space-y-6" data-testid="intelligence-view">
      <DigestSummary leads={leads} analytics={analytics} />
      <HotLeadsTable leads={scoredLeads} />
      <AgentCards runs={agentRuns} />
    </div>
  );
}
