import { useScout } from '../../hooks/useScout.ts';
import SyncStatus from './scout/SyncStatus.tsx';
import Opportunities from './scout/Opportunities.tsx';
import ApplicationTracker from './scout/ApplicationTracker.tsx';
import WeeklyMetrics from './scout/WeeklyMetrics.tsx';
import FollowUps from './scout/FollowUps.tsx';

export default function ScoutPanel() {
  const { report, loading, refreshReport } = useScout();

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border-subtle shrink-0">
        <div className="flex items-center gap-2">
          <span className="text-soul text-base">&#9830;</span>
          <h2 className="text-sm font-semibold text-fg font-display">Scout</h2>
        </div>
        <button
          type="button"
          onClick={() => refreshReport()}
          disabled={loading}
          className="px-2.5 py-1 rounded-lg text-[10px] font-medium bg-soul/10 text-soul hover:bg-soul/20 transition-colors disabled:opacity-50 cursor-pointer"
        >
          {loading ? 'Loading...' : 'Refresh'}
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {!report && !loading && (
          <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-6">
            <span className="text-3xl text-fg-muted">&#9830;</span>
            <p className="text-xs text-fg-muted">
              No scout report available. Click Refresh to load the latest data.
            </p>
          </div>
        )}

        {loading && !report && (
          <div className="flex items-center justify-center h-full">
            <div className="flex items-center gap-2 text-xs text-fg-muted">
              <span className="w-3 h-3 border-2 border-soul/40 border-t-soul rounded-full animate-spin" />
              Fetching report...
            </div>
          </div>
        )}

        {report && (
          <div className="px-4 py-3 space-y-5">
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
      </div>
    </div>
  );
}
