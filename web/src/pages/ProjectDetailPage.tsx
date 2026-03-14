import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router';
import { useProjectDetail } from '../hooks/useProjectDetail';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import type { Milestone, ProjectDetail, ProjectMetric, ProjectReadiness, ProfileSync } from '../lib/types';

const statusColor: Record<string, string> = {
  backlog: 'bg-zinc-600 text-zinc-300',
  active: 'bg-blue-500/20 text-blue-400',
  measuring: 'bg-amber-500/20 text-amber-400',
  documenting: 'bg-purple-500/20 text-purple-400',
  shipped: 'bg-emerald-500/20 text-emerald-400',
};

const milestoneStatusColor: Record<string, string> = {
  pending: 'bg-zinc-600 text-zinc-300',
  in_progress: 'bg-blue-500/20 text-blue-400',
  done: 'bg-emerald-500/20 text-emerald-400',
  skipped: 'bg-zinc-500/20 text-zinc-500',
};

type DetailTab = 'milestones' | 'guide' | 'readiness' | 'metrics';

// --- Milestones Tab ---
function MilestonesTab({ milestones, onUpdate }: { milestones: Milestone[]; onUpdate: (id: number, fields: Record<string, unknown>) => Promise<void> }) {
  const sorted = [...milestones].sort((a, b) => a.sort_order - b.sort_order);

  const nextStatus = (current: string): string | null => {
    if (current === 'pending') return 'in_progress';
    if (current === 'in_progress') return 'done';
    return null;
  };

  return (
    <div className="space-y-2" data-testid="milestones-tab">
      {sorted.length === 0 ? (
        <div className="text-sm text-zinc-500 py-4">No milestones defined.</div>
      ) : (
        sorted.map(ms => (
          <div key={ms.id} className="bg-zinc-800 rounded-lg p-4 space-y-2" data-testid={`milestone-${ms.id}`}>
            <div className="flex items-center justify-between gap-2">
              <span className="text-sm font-medium text-zinc-100">{ms.name}</span>
              <span className={`px-2 py-0.5 text-xs rounded-full shrink-0 ${milestoneStatusColor[ms.status] ?? 'bg-zinc-600 text-zinc-300'}`}>
                {ms.status.replace('_', ' ')}
              </span>
            </div>
            {ms.description && <p className="text-xs text-zinc-300">{ms.description}</p>}
            {ms.acceptance_criteria && <p className="text-xs text-zinc-400">{ms.acceptance_criteria}</p>}
            <div className="flex gap-2">
              {nextStatus(ms.status) && (
                <button
                  onClick={() => onUpdate(ms.id, { status: nextStatus(ms.status) })}
                  className="px-3 py-1 text-xs rounded bg-zinc-700 hover:bg-zinc-600 text-zinc-300 transition-colors"
                  data-testid={`milestone-advance-${ms.id}`}
                >
                  {nextStatus(ms.status) === 'in_progress' ? 'Start' : 'Complete'}
                </button>
              )}
              {ms.status !== 'done' && ms.status !== 'skipped' && (
                <button
                  onClick={() => onUpdate(ms.id, { status: 'skipped' })}
                  className="px-3 py-1 text-xs rounded bg-zinc-700 hover:bg-zinc-600 text-zinc-500 transition-colors"
                  data-testid={`milestone-skip-${ms.id}`}
                >
                  Skip
                </button>
              )}
            </div>
          </div>
        ))
      )}
    </div>
  );
}

// --- Guide Tab ---
function GuideTab({ guide }: { guide: string }) {
  if (!guide) {
    return (
      <div className="text-sm text-zinc-500 py-4" data-testid="project-guide">No guide available.</div>
    );
  }

  // Basic markdown rendering: split on ## headers, render ``` code blocks
  const renderMarkdown = (text: string) => {
    const sections = text.split(/^(## .+)$/m);
    return sections.map((section, i) => {
      if (section.startsWith('## ')) {
        return <h3 key={i} className="text-sm font-semibold text-zinc-100 mt-4 mb-2">{section.replace('## ', '')}</h3>;
      }
      // Handle code blocks
      const parts = section.split(/(```[\s\S]*?```)/);
      return parts.map((part, j) => {
        if (part.startsWith('```') && part.endsWith('```')) {
          const code = part.replace(/^```\w*\n?/, '').replace(/\n?```$/, '');
          return <pre key={`${i}-${j}`} className="bg-zinc-900 rounded p-3 text-xs text-zinc-300 overflow-x-auto my-2">{code}</pre>;
        }
        return part ? <div key={`${i}-${j}`} className="text-sm text-zinc-300 whitespace-pre-wrap">{part}</div> : null;
      });
    });
  };

  return (
    <div className="max-w-2xl space-y-1" data-testid="project-guide">
      {renderMarkdown(guide)}
    </div>
  );
}

// --- Readiness Tab ---
const PLATFORMS = ['linkedin', 'naukri', 'indeed', 'wellfound', 'instahyre', 'portfolio', 'github'];

function ReadinessTab({ readiness, syncs, onUpdateReadiness, onSyncPlatform }: {
  readiness: ProjectReadiness | null;
  syncs: ProfileSync[];
  onUpdateReadiness: (fields: Record<string, unknown>) => Promise<void>;
  onSyncPlatform: (platform: string, synced: boolean) => Promise<void>;
}) {
  const [canExplain, setCanExplain] = useState(readiness?.can_explain ?? false);
  const [canDemo, setCanDemo] = useState(readiness?.can_demo ?? false);
  const [canTradeoffs, setCanTradeoffs] = useState(readiness?.can_tradeoffs ?? false);
  const [selfScore, setSelfScore] = useState(readiness?.self_score ?? 0);

  useEffect(() => {
    setCanExplain(readiness?.can_explain ?? false);
    setCanDemo(readiness?.can_demo ?? false);
    setCanTradeoffs(readiness?.can_tradeoffs ?? false);
    setSelfScore(readiness?.self_score ?? 0);
  }, [readiness]);

  const handleSave = () => {
    onUpdateReadiness({ can_explain: canExplain, can_demo: canDemo, can_tradeoffs: canTradeoffs, self_score: selfScore });
  };

  return (
    <div className="space-y-6" data-testid="project-readiness">
      {/* Toggles */}
      <div className="bg-zinc-800 rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-medium text-zinc-400 mb-2">Self Assessment</h3>
        {([
          { label: 'Can Explain', value: canExplain, set: setCanExplain },
          { label: 'Can Demo', value: canDemo, set: setCanDemo },
          { label: 'Can Discuss Tradeoffs', value: canTradeoffs, set: setCanTradeoffs },
        ] as const).map(({ label, value, set }) => (
          <div key={label} className="flex items-center justify-between">
            <span className="text-sm text-zinc-300">{label}</span>
            <button
              onClick={() => set(!value)}
              className={`w-10 h-5 rounded-full transition-colors relative ${value ? 'bg-emerald-500' : 'bg-zinc-600'}`}
              data-testid={`toggle-${label.toLowerCase().replace(/\s+/g, '-')}`}
            >
              <span className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-all ${value ? 'left-5' : 'left-0.5'}`} />
            </button>
          </div>
        ))}

        {/* Self score */}
        <div className="pt-2">
          <span className="text-sm text-zinc-400 block mb-2">Self Score</span>
          <div className="flex gap-2" data-testid="self-score-buttons">
            {[1, 2, 3, 4, 5].map(n => (
              <button
                key={n}
                onClick={() => setSelfScore(n)}
                className={`w-8 h-8 rounded text-sm font-medium transition-colors ${selfScore === n ? 'bg-emerald-500 text-white' : 'bg-zinc-700 text-zinc-300 hover:bg-zinc-600'}`}
                data-testid={`score-${n}`}
              >
                {n}
              </button>
            ))}
          </div>
        </div>

        <div className="pt-2">
          <button
            onClick={handleSave}
            className="px-4 py-2 text-sm rounded-lg bg-soul text-deep font-medium hover:bg-soul/90 transition-colors"
            data-testid="readiness-save"
          >
            Save
          </button>
        </div>
      </div>

      {/* Platform syncs */}
      <div className="bg-zinc-800 rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-medium text-zinc-400 mb-2">Platform Syncs</h3>
        {PLATFORMS.map(platform => {
          const sync = syncs.find(s => s.platform === platform);
          const isSynced = sync?.synced ?? false;
          return (
            <div key={platform} className="flex items-center justify-between" data-testid={`sync-${platform}`}>
              <span className="text-sm text-zinc-300 capitalize">{platform}</span>
              <button
                onClick={() => onSyncPlatform(platform, !isSynced)}
                className={`w-5 h-5 rounded border transition-colors flex items-center justify-center ${isSynced ? 'bg-emerald-500 border-emerald-500' : 'border-zinc-600 hover:border-zinc-400'}`}
                data-testid={`sync-toggle-${platform}`}
              >
                {isSynced && <span className="text-white text-xs">&#10003;</span>}
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// --- Metrics Tab ---
function MetricsTab({ metrics, onRecord }: { metrics: ProjectMetric[]; onRecord: (name: string, value: string, unit: string) => Promise<void> }) {
  const [name, setName] = useState('');
  const [value, setValue] = useState('');
  const [unit, setUnit] = useState('');

  const handleAdd = async () => {
    if (!name.trim() || !value.trim()) return;
    await onRecord(name.trim(), value.trim(), unit.trim());
    setName('');
    setValue('');
    setUnit('');
  };

  return (
    <div className="space-y-4" data-testid="project-metrics">
      {/* Metrics table */}
      {metrics.length === 0 ? (
        <div className="text-sm text-zinc-500 py-4">No metrics recorded yet.</div>
      ) : (
        <div className="bg-zinc-800 rounded-lg overflow-hidden">
          <table className="w-full text-sm" data-testid="metrics-table">
            <thead>
              <tr className="text-xs text-zinc-400 border-b border-zinc-700">
                <th className="text-left px-3 py-2 font-medium">Name</th>
                <th className="text-right px-3 py-2 font-medium">Value</th>
                <th className="text-left px-3 py-2 font-medium">Unit</th>
                <th className="text-left px-3 py-2 font-medium">Date</th>
              </tr>
            </thead>
            <tbody>
              {metrics.map(m => (
                <tr key={m.id} className="border-b border-zinc-700/50 text-zinc-300" data-testid={`metric-${m.id}`}>
                  <td className="px-3 py-2">{m.name}</td>
                  <td className="px-3 py-2 text-right font-medium text-zinc-100">{m.value}</td>
                  <td className="px-3 py-2 text-zinc-400">{m.unit}</td>
                  <td className="px-3 py-2 text-zinc-400">{new Date(m.captured_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add form */}
      <div className="bg-zinc-800 rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-medium text-zinc-400">Add Metric</h3>
        <div className="flex gap-2 flex-wrap">
          <input
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="Name"
            className="flex-1 min-w-[120px] bg-zinc-900 rounded px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-500 outline-none focus:ring-1 focus:ring-zinc-600"
            data-testid="metric-name-input"
          />
          <input
            value={value}
            onChange={e => setValue(e.target.value)}
            placeholder="Value"
            className="w-24 bg-zinc-900 rounded px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-500 outline-none focus:ring-1 focus:ring-zinc-600"
            data-testid="metric-value-input"
          />
          <input
            value={unit}
            onChange={e => setUnit(e.target.value)}
            placeholder="Unit"
            className="w-20 bg-zinc-900 rounded px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-500 outline-none focus:ring-1 focus:ring-zinc-600"
            data-testid="metric-unit-input"
          />
          <button
            onClick={handleAdd}
            disabled={!name.trim() || !value.trim()}
            className="px-4 py-2 text-sm rounded-lg bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
            data-testid="metric-add-button"
          >
            Add
          </button>
        </div>
      </div>
    </div>
  );
}

// --- Main Page ---
export function ProjectDetailPage() {
  usePerformance('ProjectDetailPage');
  const { id } = useParams();
  const projectId = Number(id);
  const navigate = useNavigate();
  const { project, guide, loading, error, updateMilestone, recordMetric, updateReadiness, syncPlatform } = useProjectDetail(projectId);
  const [activeTab, setActiveTab] = useState<DetailTab>('milestones');

  useEffect(() => { reportUsage('page.view', { page: 'project_detail', projectId }); }, [projectId]);

  const tabs: DetailTab[] = ['milestones', 'guide', 'readiness', 'metrics'];

  return (
    <div data-testid="project-detail-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Back button */}
      <button
        onClick={() => navigate('/projects')}
        className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
        data-testid="project-back"
      >
        &larr; Back to Projects
      </button>

      {error && <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="project-error">{error}</div>}

      {loading && !project && (
        <div className="text-center py-8 text-zinc-400">Loading...</div>
      )}

      {project && (
        <>
          {/* Header */}
          <div className="flex items-center justify-between flex-wrap gap-3">
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-semibold text-zinc-100" data-testid="project-name">{project.name}</h2>
              <span className={`px-2 py-0.5 text-xs rounded-full ${statusColor[project.status] ?? 'bg-zinc-600 text-zinc-300'}`} data-testid="project-status">
                {project.status}
              </span>
            </div>
            <div className="flex items-center gap-3 text-sm text-zinc-400">
              <span data-testid="project-phase">Phase {project.phase}</span>
              <span data-testid="project-hours">{project.hours_actual}/{project.hours_estimated}h</span>
              {project.github_repo && (
                <a
                  href={project.github_repo}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-400 hover:text-blue-300 transition-colors"
                  data-testid="project-github-link"
                >
                  GitHub
                </a>
              )}
            </div>
          </div>

          {/* Tab nav */}
          <nav className="flex gap-1 border-b border-zinc-700 pb-px" data-testid="detail-tabs">
            {tabs.map(tab => (
              <button key={tab} onClick={() => setActiveTab(tab)}
                className={`px-3 py-1.5 text-sm rounded-t transition-colors capitalize ${activeTab === tab ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-200'}`}
                data-testid={`detail-tab-${tab}`}>
                {tab}
              </button>
            ))}
          </nav>

          {/* Tab content */}
          {activeTab === 'milestones' && <MilestonesTab milestones={project.milestones} onUpdate={updateMilestone} />}
          {activeTab === 'guide' && <GuideTab guide={guide} />}
          {activeTab === 'readiness' && <ReadinessTab readiness={project.readiness} syncs={project.syncs} onUpdateReadiness={updateReadiness} onSyncPlatform={syncPlatform} />}
          {activeTab === 'metrics' && <MetricsTab metrics={project.metrics} onRecord={recordMetric} />}
        </>
      )}
    </div>
  );
}
