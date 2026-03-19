import { useState, useEffect } from 'react';
import { useScout } from '../hooks/useScout';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import { PipelineBoard } from '../components/scout/PipelineBoard';
import { LeadDetail } from '../components/scout/LeadDetail';
import { AnalyticsView } from '../components/scout/AnalyticsView';
import { ActionsView } from '../components/scout/ActionsView';
import { SyncStatus } from '../components/scout/SyncStatus';
import { AgentActivity } from '../components/scout/AgentActivity';
import { ProfilePanel } from '../components/scout/ProfilePanel';
import { IntelligenceView } from '../components/scout/IntelligenceView';
import { PriorityQueue } from '../components/scout/PriorityQueue';
import type { ScoutLead } from '../hooks/useScout';

export function ScoutPage() {
  usePerformance('ScoutPage');
  const {
    leads, analytics, sweepStatus, profile, optimizations, agentRuns, scoredLeads,
    loading, error, activeTab, setActiveTab, refresh,
    updateLead, triggerSweep, syncPlatform,
  } = useScout();

  const [selectedLead, setSelectedLead] = useState<ScoutLead | null>(null);

  useEffect(() => { reportUsage('page.view', { page: 'scout' }); }, []);

  const tabs = ['priority', 'pipeline', 'analytics', 'actions', 'profile', 'intelligence'] as const;

  const handleApproveOptimization = (id: number) => {
    updateLead(id, {}).catch(() => { /* error surfaced via hook */ });
  };

  const handleRejectOptimization = (id: number) => {
    updateLead(id, {}).catch(() => { /* error surfaced via hook */ });
  };

  const handleTriggerSweep = () => {
    triggerSweep(['linkedin', 'github', 'naukri', 'wellfound']).catch(() => { /* error surfaced via hook */ });
  };

  return (
    <div data-testid="scout-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-fg">Scout</h2>
        <div className="flex items-center gap-2">
          {activeTab === 'actions' && (
            <button
              onClick={handleTriggerSweep}
              className="px-3 py-1 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
              data-testid="scout-sweep-btn"
            >
              Run Sweep
            </button>
          )}
          <button onClick={refresh} className="px-3 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors" data-testid="scout-refresh">Refresh</button>
        </div>
      </div>

      {/* Tab nav */}
      <nav className="tab-scroll flex gap-1 border-b border-border-subtle pb-px" data-testid="scout-tabs">
        {tabs.map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className={`px-3 py-1.5 text-sm rounded-t transition-colors capitalize ${activeTab === tab ? 'bg-surface text-fg' : 'text-fg-muted hover:text-zinc-200'}`}
            data-testid={`tab-${tab}`}>
            {tab}
          </button>
        ))}
      </nav>

      {error && <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="scout-error">{error}</div>}

      {/* Tab content */}
      {activeTab === 'priority' && (
        <PriorityQueue leads={leads} />
      )}

      {activeTab === 'pipeline' && (
        <>
          {selectedLead ? (
            <LeadDetail
              lead={selectedLead}
              onUpdate={async (id, data) => {
                await updateLead(id, data);
                setSelectedLead(null);
              }}
              onClose={() => setSelectedLead(null)}
            />
          ) : (
            <PipelineBoard leads={leads} onSelectLead={setSelectedLead} />
          )}
        </>
      )}

      {activeTab === 'analytics' && analytics && (
        <AnalyticsView analytics={analytics} />
      )}

      {activeTab === 'actions' && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <AgentActivity sweepStatus={sweepStatus} />
            <SyncStatus onSync={syncPlatform} />
          </div>
          <ActionsView
            leads={leads}
            optimizations={optimizations}
            onApproveOptimization={handleApproveOptimization}
            onRejectOptimization={handleRejectOptimization}
          />
        </div>
      )}

      {activeTab === 'profile' && profile && (
        <ProfilePanel profile={profile} />
      )}

      {activeTab === 'intelligence' && (
        <IntelligenceView
          scoredLeads={scoredLeads}
          agentRuns={agentRuns}
          leads={leads}
          analytics={analytics}
        />
      )}

      {loading && (
        <div className="text-center py-8 text-fg-muted">Loading...</div>
      )}
    </div>
  );
}
