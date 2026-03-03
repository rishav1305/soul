import { useState } from 'react';
import { useScout } from '../../hooks/useScout.ts';
import { useProfile } from '../../hooks/useProfile.ts';
import SyncStatus from './scout/SyncStatus.tsx';
import Opportunities from './scout/Opportunities.tsx';
import ApplicationTracker from './scout/ApplicationTracker.tsx';
import WeeklyMetrics from './scout/WeeklyMetrics.tsx';
import FollowUps from './scout/FollowUps.tsx';
import ProfilePanel from './scout/ProfilePanel.tsx';

type Tab = 'dashboard' | 'profile';

const PHASE_LABELS: Record<string, string> = {
  syncing: 'Syncing platforms...',
  sweeping: 'Sweeping opportunities...',
  loading: 'Loading report...',
};

export default function ScoutPanel() {
  const { report, loading, refreshing, refreshPhase, refreshReport } = useScout();
  const { profile, loading: profileLoading, pulling, error: profileError, pullFromSupabase } = useProfile();
  const [activeTab, setActiveTab] = useState<Tab>('dashboard');

  const busy = loading || refreshing;

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border-subtle shrink-0">
        <div className="flex items-center gap-2">
          <span className="text-soul text-base">&#9830;</span>
          <h2 className="text-sm font-semibold text-fg font-display">Scout</h2>
        </div>
        {activeTab === 'dashboard' && (
          <button
            type="button"
            onClick={() => refreshReport()}
            disabled={busy}
            className="px-2.5 py-1 rounded-lg text-[10px] font-medium bg-soul/10 text-soul hover:bg-soul/20 transition-colors disabled:opacity-50 cursor-pointer"
          >
            {refreshing ? (PHASE_LABELS[refreshPhase!] ?? 'Refreshing...') : loading ? 'Loading...' : 'Refresh'}
          </button>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 px-4 py-1.5 border-b border-border-subtle shrink-0">
        <button
          type="button"
          onClick={() => setActiveTab('dashboard')}
          className={`text-[11px] px-2.5 py-1 rounded-md transition-colors cursor-pointer ${
            activeTab === 'dashboard'
              ? 'bg-elevated text-fg font-medium'
              : 'text-fg-muted hover:text-fg hover:bg-elevated/50'
          }`}
        >
          Dashboard
        </button>
        <button
          type="button"
          onClick={() => setActiveTab('profile')}
          className={`text-[11px] px-2.5 py-1 rounded-md transition-colors cursor-pointer ${
            activeTab === 'profile'
              ? 'bg-elevated text-fg font-medium'
              : 'text-fg-muted hover:text-fg hover:bg-elevated/50'
          }`}
        >
          Profile
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {activeTab === 'dashboard' && (
          <>
            {!report && !busy && (
              <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-6">
                <span className="text-3xl text-fg-muted">&#9830;</span>
                <p className="text-xs text-fg-muted">
                  No scout report available. Click Refresh to load the latest data.
                </p>
              </div>
            )}

            {busy && !report && (
              <div className="flex items-center justify-center h-full">
                <div className="flex items-center gap-2 text-xs text-fg-muted">
                  <span className="w-3 h-3 border-2 border-soul/40 border-t-soul rounded-full animate-spin" />
                  {refreshPhase ? (PHASE_LABELS[refreshPhase] ?? 'Working...') : 'Fetching report...'}
                </div>
              </div>
            )}

            {report && (
              <div className="px-4 py-3 space-y-5">
                {refreshing && (
                  <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-soul/5 border border-soul/15 text-xs text-soul">
                    <span className="w-3 h-3 border-2 border-soul/40 border-t-soul rounded-full animate-spin shrink-0" />
                    {PHASE_LABELS[refreshPhase!] ?? 'Refreshing...'}
                  </div>
                )}

                <SyncStatus
                  platforms={report.sync.details}
                  lastRun={report.sync.last_run}
                />

                <Opportunities opportunities={report.sweep.opportunities} />

                <ApplicationTracker
                  applications={report.applications.recent}
                  byStatus={report.applications.by_status}
                />

                <WeeklyMetrics metrics={report.metrics} />

                <FollowUps followUps={report.follow_ups} />
              </div>
            )}
          </>
        )}

        {activeTab === 'profile' && (
          <ProfilePanel
            profile={profile}
            loading={profileLoading}
            pulling={pulling}
            error={profileError}
            onPull={pullFromSupabase}
          />
        )}
      </div>
    </div>
  );
}
