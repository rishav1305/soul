import { useEffect } from 'react';
import { useNavigate } from 'react-router';
import { useProjects } from '../hooks/useProjects';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import { ProjectCard } from '../components/ProjectCard';
import type { ProjectDashboard, ProjectKeyword, ProjectSummary } from '../lib/types';

// Status colors matching ProjectCard
const statusColor: Record<string, string> = {
  backlog: 'bg-overlay text-fg-secondary',
  active: 'bg-blue-500/20 text-blue-400',
  measuring: 'bg-amber-500/20 text-amber-400',
  documenting: 'bg-purple-500/20 text-purple-400',
  shipped: 'bg-emerald-500/20 text-emerald-400',
};

// --- Dashboard Tab ---
function DashboardTab({ dashboard }: { dashboard: ProjectDashboard }) {
  const shippedPercent = dashboard.total_projects > 0
    ? Math.round((dashboard.shipped / dashboard.total_projects) * 100)
    : 0;
  const keywordsShippedPercent = dashboard.keywords_total > 0
    ? Math.round((dashboard.keywords_shipped / dashboard.keywords_total) * 100)
    : 0;

  return (
    <div className="space-y-6" data-testid="projects-dashboard">
      {/* Overall progress */}
      <div className="bg-surface rounded-lg p-4 space-y-3">
        <div className="flex items-center justify-between">
          <span className="text-sm text-fg-muted">Overall Progress</span>
          <span className="text-lg font-bold text-emerald-400">{dashboard.shipped}/{dashboard.total_projects} shipped</span>
        </div>
        <div className="w-full h-2 bg-elevated rounded-full overflow-hidden">
          <div className="h-full bg-emerald-500 rounded-full transition-all" style={{ width: `${shippedPercent}%` }} data-testid="shipped-progress-bar" />
        </div>
      </div>

      {/* Status cards */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3" data-testid="status-cards">
        {(['backlog', 'active', 'measuring', 'documenting', 'shipped'] as const).map(status => {
          const count = dashboard[status];
          return (
            <div key={status} className="bg-surface rounded-lg p-3 text-center" data-testid={`status-card-${status}`}>
              <div className="text-2xl font-bold text-fg">{count}</div>
              <span className={`inline-block mt-1 px-2 py-0.5 text-xs rounded-full ${statusColor[status]}`}>
                {status}
              </span>
            </div>
          );
        })}
      </div>

      {/* Keyword coverage */}
      <div className="bg-surface rounded-lg p-4 space-y-3">
        <div className="flex items-center justify-between">
          <span className="text-sm text-fg-muted">Keyword Coverage</span>
          <span className="text-sm font-medium text-fg">{dashboard.keywords_shipped}/{dashboard.keywords_total} shipped</span>
        </div>
        <div className="w-full h-1.5 bg-elevated rounded-full overflow-hidden">
          <div className="h-full bg-emerald-500 rounded-full transition-all" style={{ width: `${keywordsShippedPercent}%` }} />
        </div>
        <div className="flex gap-3 text-xs text-fg-muted">
          <span>Claimed: {dashboard.keywords_claimed}</span>
          <span>Building: {dashboard.keywords_building}</span>
          <span>Shipped: {dashboard.keywords_shipped}</span>
        </div>
      </div>

      {/* Hours + Readiness */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div className="bg-surface rounded-lg p-4" data-testid="hours-card">
          <h3 className="text-sm font-medium text-fg-muted mb-2">Hours</h3>
          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-bold text-fg">{dashboard.hours_actual}</span>
            <span className="text-sm text-fg-muted">/ {dashboard.hours_estimated} estimated</span>
          </div>
        </div>
        <div className="bg-surface rounded-lg p-4" data-testid="readiness-card">
          <h3 className="text-sm font-medium text-fg-muted mb-2">Avg Readiness</h3>
          <span className={`text-2xl font-bold ${dashboard.avg_readiness >= 4 ? 'text-emerald-400' : dashboard.avg_readiness >= 2.5 ? 'text-amber-400' : 'text-red-400'}`}>
            {dashboard.avg_readiness.toFixed(1)}/5
          </span>
        </div>
      </div>
    </div>
  );
}

// --- Projects Tab ---
function ProjectsTab({ projects, navigate }: { projects: ProjectSummary[]; navigate: (path: string) => void }) {
  return (
    <div data-testid="projects-list">
      {projects.length === 0 ? (
        <div className="text-sm text-fg-muted py-4">No projects found.</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {projects.map(project => (
            <ProjectCard
              key={project.id}
              project={project}
              onClick={() => navigate(`/projects/${project.id}`)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

// --- Timeline Tab ---
function TimelineTab({ projects }: { projects: ProjectSummary[] }) {
  // Group by phase, then show within a 10-week grid
  const phases = [1, 2, 3, 4];
  const weeks = Array.from({ length: 10 }, (_, i) => i + 1);

  return (
    <div className="space-y-4" data-testid="projects-timeline">
      <div className="overflow-x-auto">
        <div className="min-w-[600px]">
          {/* Week headers */}
          <div className="grid grid-cols-[80px_repeat(10,1fr)] gap-1 mb-2">
            <div className="text-xs text-fg-muted font-medium">Phase</div>
            {weeks.map(w => (
              <div key={w} className="text-xs text-fg-muted text-center font-medium">W{w}</div>
            ))}
          </div>

          {/* Phase rows */}
          {phases.map(phase => {
            const phaseProjects = projects.filter(p => p.phase === phase);
            return (
              <div key={phase} className="grid grid-cols-[80px_repeat(10,1fr)] gap-1 mb-1" data-testid={`timeline-phase-${phase}`}>
                <div className="text-xs text-fg-muted flex items-center">Phase {phase}</div>
                {weeks.map(w => {
                  const weekProjects = phaseProjects.filter(p => p.week_planned === w);
                  return (
                    <div key={w} className="min-h-[32px] flex flex-col gap-0.5">
                      {weekProjects.map(p => (
                        <div
                          key={p.id}
                          className={`px-1.5 py-0.5 text-[10px] rounded truncate ${statusColor[p.status] ?? 'bg-overlay text-fg-secondary'}`}
                          title={p.name}
                          data-testid={`timeline-project-${p.id}`}
                        >
                          {p.name}
                        </div>
                      ))}
                    </div>
                  );
                })}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// --- Keywords Tab ---
function KeywordsTab({ keywords }: { keywords: ProjectKeyword[] }) {
  const shipped = keywords.filter(k => k.status === 'shipped');
  const building = keywords.filter(k => k.status === 'building');
  const claimed = keywords.filter(k => k.status === 'claimed');

  return (
    <div className="space-y-6" data-testid="projects-keywords">
      {/* Shipped */}
      <div>
        <h3 className="text-sm font-medium text-fg-muted mb-2">Shipped ({shipped.length})</h3>
        <div className="flex flex-wrap gap-1.5">
          {shipped.length === 0 ? (
            <span className="text-xs text-fg-muted">None yet</span>
          ) : (
            shipped.map(k => (
              <span key={k.id} className="px-2 py-0.5 text-xs rounded-full bg-emerald-500/20 text-emerald-400" data-testid={`keyword-${k.id}`}>
                {k.keyword}
              </span>
            ))
          )}
        </div>
      </div>

      {/* Building */}
      <div>
        <h3 className="text-sm font-medium text-fg-muted mb-2">Building ({building.length})</h3>
        <div className="flex flex-wrap gap-1.5">
          {building.length === 0 ? (
            <span className="text-xs text-fg-muted">None</span>
          ) : (
            building.map(k => (
              <span key={k.id} className="px-2 py-0.5 text-xs rounded-full bg-amber-500/20 text-amber-400" data-testid={`keyword-${k.id}`}>
                {k.keyword}
              </span>
            ))
          )}
        </div>
      </div>

      {/* Claimed */}
      <div>
        <h3 className="text-sm font-medium text-fg-muted mb-2">Claimed ({claimed.length})</h3>
        <div className="flex flex-wrap gap-1.5">
          {claimed.length === 0 ? (
            <span className="text-xs text-fg-muted">None</span>
          ) : (
            claimed.map(k => (
              <span key={k.id} className="px-2 py-0.5 text-xs rounded-full bg-overlay text-fg-secondary" data-testid={`keyword-${k.id}`}>
                {k.keyword}
              </span>
            ))
          )}
        </div>
      </div>
    </div>
  );
}

// --- Main Page ---
export function ProjectsPage() {
  usePerformance('ProjectsPage');
  const navigate = useNavigate();
  const { dashboard, keywords, loading, error, activeTab, setActiveTab, refresh } = useProjects();

  useEffect(() => { reportUsage('page.view', { page: 'projects' }); }, []);

  const tabs = ['dashboard', 'projects', 'timeline', 'keywords'] as const;

  return (
    <div data-testid="projects-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-fg">Projects</h2>
        <button onClick={refresh} className="px-3 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors" data-testid="projects-refresh">Refresh</button>
      </div>

      {/* Tab nav */}
      <nav className="tab-scroll flex gap-1 border-b border-border-subtle pb-px" data-testid="projects-tabs">
        {tabs.map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className={`px-3 py-1.5 text-sm rounded-t transition-colors capitalize ${activeTab === tab ? 'bg-surface text-fg' : 'text-fg-muted hover:text-zinc-200'}`}
            data-testid={`tab-${tab}`}>
            {tab}
          </button>
        ))}
      </nav>

      {error && <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="projects-error">{error}</div>}

      {/* Tab content */}
      {activeTab === 'dashboard' && dashboard && <DashboardTab dashboard={dashboard} />}
      {activeTab === 'projects' && dashboard && <ProjectsTab projects={dashboard.projects} navigate={(p: string) => navigate(p)} />}
      {activeTab === 'timeline' && dashboard && <TimelineTab projects={dashboard.projects} />}
      {activeTab === 'keywords' && <KeywordsTab keywords={keywords} />}
      {loading && !dashboard && keywords.length === 0 && (
        <div className="text-center py-8 text-fg-muted">Loading...</div>
      )}
    </div>
  );
}
