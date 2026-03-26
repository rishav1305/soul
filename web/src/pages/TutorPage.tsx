import { useEffect } from 'react';
import { useNavigate } from 'react-router';
import { useTutor } from '../hooks/useTutor';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import type { TutorDashboard, TutorTopic, TutorAnalytics, TutorMockSession } from '../lib/types';

// Color helpers
const statusColor: Record<string, string> = {
  not_started: 'bg-overlay text-fg-secondary',
  learning: 'bg-blue-500/20 text-blue-400',
  drilling: 'bg-amber-500/20 text-amber-400',
  mastered: 'bg-emerald-500/20 text-emerald-400',
};

const difficultyColor: Record<string, string> = {
  easy: 'text-emerald-400',
  medium: 'text-amber-400',
  hard: 'text-red-400',
};

function scoreColor(score: number): string {
  if (score >= 80) return 'text-emerald-400';
  if (score >= 50) return 'text-amber-400';
  return 'text-red-400';
}

function formatTime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const mins = Math.floor(seconds / 60);
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  return `${hrs}h ${mins % 60}m`;
}

// --- Dashboard Tab ---
function DashboardTab({ dashboard }: { dashboard: TutorDashboard }) {
  const readinessPercent = Math.round(dashboard.readinessPct);
  const todaySummary = (dashboard.todayActivity ?? []).reduce(
    (acc, a) => ({
      time: acc.time + a.timeSpentSeconds,
      sessions: acc.sessions + a.sessionsCount,
      questions: acc.questions + a.questionsAnswered,
      scoreSum: acc.scoreSum + a.scoreAvg,
      count: acc.count + (a.scoreAvg > 0 ? 1 : 0),
    }),
    { time: 0, sessions: 0, questions: 0, scoreSum: 0, count: 0 },
  );
  const todayAvgScore = todaySummary.count > 0 ? todaySummary.scoreSum / todaySummary.count : 0;

  return (
    <div className="space-y-6" data-testid="dashboard-tab">
      {/* Readiness + badges */}
      <div className="bg-surface rounded-lg p-4 space-y-3">
        <div className="flex items-center justify-between">
          <span className="text-sm text-fg-muted">Interview Readiness</span>
          <span className={`text-lg font-bold ${scoreColor(readinessPercent)}`} data-testid="readiness-score">{readinessPercent}%</span>
        </div>
        <div className="w-full h-2 bg-elevated rounded-full overflow-hidden">
          <div className="h-full bg-emerald-500 rounded-full transition-all" style={{ width: `${readinessPercent}%` }} data-testid="readiness-bar" role="progressbar" aria-valuenow={readinessPercent} aria-valuemin={0} aria-valuemax={100} aria-label="Interview readiness" />
        </div>
        <div className="flex gap-3">
          <span className="px-2 py-0.5 text-xs rounded-full bg-amber-500/20 text-amber-400" data-testid="streak-badge">
            {dashboard.streak} day streak
          </span>
          {dashboard.dueReviewCount > 0 && (
            <span className="px-2 py-0.5 text-xs rounded-full bg-red-500/20 text-red-400" data-testid="due-reviews-badge">
              {dashboard.dueReviewCount} due reviews
            </span>
          )}
        </div>
      </div>

      {/* Module cards */}
      <div>
        <h3 className="text-sm font-medium text-fg-muted mb-3">Modules</h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3" data-testid="module-grid">
          {(dashboard.moduleStats ?? []).map(mod => {
            const completionPercent = Math.round(mod.completionPct);
            return (
              <div key={mod.module} className="bg-surface rounded-lg p-4 space-y-2" data-testid={`module-card-${mod.module}`}>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium text-fg capitalize">{mod.module}</span>
                  <span className={`text-xs font-medium ${scoreColor(completionPercent)}`}>{completionPercent}%</span>
                </div>
                <div className="w-full h-1.5 bg-elevated rounded-full overflow-hidden">
                  <div className="h-full bg-emerald-500 rounded-full transition-all" style={{ width: `${completionPercent}%` }} />
                </div>
                <div className="flex items-center justify-between text-xs text-fg-muted">
                  <span>{mod.completed}/{mod.topicCount} completed</span>
                  <span>{mod.inProgress} in progress</span>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Today's activity */}
      <div className="bg-surface rounded-lg p-4" data-testid="today-activity">
        <h3 className="text-sm font-medium text-fg-muted mb-3">Today</h3>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div className="text-center">
            <div className="text-lg font-bold text-fg">{formatTime(todaySummary.time)}</div>
            <div className="text-xs text-fg-muted">Time</div>
          </div>
          <div className="text-center">
            <div className="text-lg font-bold text-fg">{todaySummary.sessions}</div>
            <div className="text-xs text-fg-muted">Sessions</div>
          </div>
          <div className="text-center">
            <div className="text-lg font-bold text-fg">{todaySummary.questions}</div>
            <div className="text-xs text-fg-muted">Questions</div>
          </div>
          <div className="text-center">
            <div className={`text-lg font-bold ${scoreColor(Math.round(todayAvgScore))}`}>
              {todayAvgScore > 0 ? `${Math.round(todayAvgScore)}%` : '--'}
            </div>
            <div className="text-xs text-fg-muted">Avg Score</div>
          </div>
        </div>
      </div>
    </div>
  );
}

// --- Analytics Tab ---
function AnalyticsTab({ analytics }: { analytics: TutorAnalytics }) {
  // Flatten last30Days into activity rows (skip null days)
  const activityRows = (analytics.last30Days ?? []).flatMap(day =>
    (day.activity ?? []).map(a => ({ ...a, date: day.date }))
  );
  const gaps = analytics.confidenceGaps ?? [];

  return (
    <div className="space-y-6" data-testid="analytics-tab">
      {/* Activity table */}
      <div>
        <h3 className="text-sm font-medium text-fg-muted mb-3">Daily Activity</h3>
        {activityRows.length === 0 ? (
          <div className="text-sm text-fg-muted">No activity recorded yet.</div>
        ) : (
          <div className="bg-surface rounded-lg overflow-hidden">
            <table className="w-full text-sm" data-testid="activity-table">
              <thead>
                <tr className="text-xs text-fg-muted border-b border-border-subtle">
                  <th className="text-left px-3 py-2 font-medium">Date</th>
                  <th className="text-left px-3 py-2 font-medium">Module</th>
                  <th className="text-right px-3 py-2 font-medium">Time</th>
                  <th className="text-right px-3 py-2 font-medium">Sessions</th>
                  <th className="text-right px-3 py-2 font-medium">Questions</th>
                  <th className="text-right px-3 py-2 font-medium">Score</th>
                </tr>
              </thead>
              <tbody>
                {activityRows.map((row, i) => (
                  <tr key={`${row.date}-${row.module}-${i}`} className="border-b border-border-subtle/50 text-fg-secondary">
                    <td className="px-3 py-2">{row.date}</td>
                    <td className="px-3 py-2">{row.module}</td>
                    <td className="px-3 py-2 text-right">{formatTime(row.timeSpentSeconds)}</td>
                    <td className="px-3 py-2 text-right">{row.sessionsCount}</td>
                    <td className="px-3 py-2 text-right">{row.questionsAnswered}</td>
                    <td className={`px-3 py-2 text-right ${scoreColor(Math.round(row.scoreAvg))}`}>
                      {row.scoreAvg > 0 ? `${Math.round(row.scoreAvg)}%` : '--'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Confidence gaps */}
      <div>
        <h3 className="text-sm font-medium text-fg-muted mb-3">Confidence Gaps</h3>
        {gaps.length === 0 ? (
          <div className="text-sm text-fg-muted">No confidence gaps detected.</div>
        ) : (
          <div className="bg-surface rounded-lg overflow-hidden">
            <table className="w-full text-sm" data-testid="confidence-gaps-table">
              <thead>
                <tr className="text-xs text-fg-muted border-b border-border-subtle">
                  <th className="text-left px-3 py-2 font-medium">Topic</th>
                  <th className="text-right px-3 py-2 font-medium">Self-Rated</th>
                  <th className="text-right px-3 py-2 font-medium">Actual</th>
                  <th className="text-right px-3 py-2 font-medium">Gap</th>
                </tr>
              </thead>
              <tbody>
                {gaps.map(gap => (
                  <tr key={gap.topicId} className="border-b border-border-subtle/50 text-fg-secondary">
                    <td className="px-3 py-2">{gap.topicName || gap.topicId}</td>
                    <td className="px-3 py-2 text-right">{Math.round(gap.avgSelfRated)}%</td>
                    <td className="px-3 py-2 text-right">{Math.round(gap.avgActual)}%</td>
                    <td className={`px-3 py-2 text-right font-medium ${gap.gap > 20 ? 'text-red-400' : 'text-amber-400'}`}>
                      {Math.round(gap.gap)}%
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

// --- Topics Tab ---
const MODULE_FILTERS = ['', 'DSA', 'SystemDesign', 'AI', 'Behavioral', 'GoLang'];

function TopicsTab({ topics, moduleFilter, setModuleFilter, navigate }: {
  topics: TutorTopic[];
  moduleFilter: string;
  setModuleFilter: (m: string) => void;
  navigate: (path: string) => void;
}) {
  return (
    <div className="space-y-4" data-testid="topics-tab">
      {/* Module filters */}
      <div className="flex flex-wrap gap-1" data-testid="module-filters">
        {MODULE_FILTERS.map(mod => (
          <button
            key={mod || 'all'}
            onClick={() => setModuleFilter(mod)}
            className={`px-3 py-1 text-xs rounded-full transition-colors ${moduleFilter === mod ? 'bg-overlay text-fg' : 'bg-surface text-fg-muted hover:text-zinc-200'}`}
            data-testid={`filter-${mod || 'all'}`}
          >
            {mod || 'All'}
          </button>
        ))}
      </div>

      {/* Topic rows */}
      {topics.length === 0 ? (
        <div className="text-sm text-fg-muted py-4">No topics found.</div>
      ) : (
        <div className="space-y-1" data-testid="topics-list">
          {topics.map(topic => (
            <button
              key={topic.id}
              onClick={() => navigate(`/tutor/drill/${topic.id}`)}
              className="w-full flex items-center justify-between p-3 rounded-lg bg-surface hover:bg-elevated transition-colors text-left"
              data-testid={`topic-${topic.id}`}
            >
              <div className="min-w-0">
                <div className="text-sm text-fg truncate">{topic.name}</div>
                <div className="text-xs text-fg-muted">{topic.category}</div>
              </div>
              <div className="flex items-center gap-2 shrink-0 ml-3">
                <span className={`text-xs font-medium ${difficultyColor[topic.difficulty] ?? 'text-fg-muted'}`}>
                  {topic.difficulty}
                </span>
                <span className={`px-2 py-0.5 text-xs rounded-full ${statusColor[topic.status] ?? 'bg-overlay text-fg-secondary'}`}>
                  {topic.status.replace('_', ' ')}
                </span>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// --- Mocks Tab ---
function MocksTab({ mocks, navigate }: { mocks: TutorMockSession[]; navigate: (path: string) => void }) {
  return (
    <div className="space-y-3" data-testid="mocks-tab">
      {mocks.length === 0 ? (
        <div className="text-sm text-fg-muted py-4">No mock sessions yet. Start one via chat.</div>
      ) : (
        mocks.map(session => {
          const scorePercent = session.overall_score != null ? Math.round(session.overall_score) : null;
          return (
            <button
              key={session.id}
              onClick={() => navigate(`/tutor/mock/${session.id}`)}
              className="w-full bg-surface rounded-lg p-4 hover:bg-elevated transition-colors text-left space-y-2"
              data-testid={`mock-${session.id}`}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="px-2 py-0.5 text-xs rounded-full bg-overlay text-fg-secondary capitalize">{session.type.replace('_', ' ')}</span>
                  <span className="text-xs text-fg-muted">{new Date(session.started_at).toLocaleDateString()}</span>
                </div>
                {scorePercent != null ? (
                  <span className={`text-sm font-bold ${scoreColor(scorePercent)}`} data-testid={`mock-score-${session.id}`}>{scorePercent}%</span>
                ) : (
                  <span className="text-xs text-fg-muted">In progress</span>
                )}
              </div>
              {scorePercent != null && (
                <div className="w-full h-1.5 bg-elevated rounded-full overflow-hidden">
                  <div className="h-full bg-emerald-500 rounded-full transition-all" style={{ width: `${scorePercent}%` }} />
                </div>
              )}
              {session.job_description && (
                <div className="text-xs text-fg-muted truncate">{session.job_description.slice(0, 120)}</div>
              )}
            </button>
          );
        })
      )}
    </div>
  );
}

// --- Guide Tab ---
function GuideTab() {
  return (
    <div className="space-y-6 max-w-2xl" data-testid="guide-tab">
      <div className="bg-surface rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-semibold text-fg">Tabs Overview</h3>
        <ul className="text-sm text-fg-secondary space-y-1.5 list-disc list-inside">
          <li><strong className="text-fg">Dashboard</strong> -- Readiness score, module progress, and today's activity.</li>
          <li><strong className="text-fg">Analytics</strong> -- Daily activity history and confidence gap analysis.</li>
          <li><strong className="text-fg">Topics</strong> -- Browse topics by module and start drill sessions.</li>
          <li><strong className="text-fg">Mocks</strong> -- View mock interview sessions and scores.</li>
          <li><strong className="text-fg">Guide</strong> -- This page. Learning flow and mastery criteria.</li>
        </ul>
      </div>

      <div className="bg-surface rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-semibold text-fg">Chat Commands</h3>
        <div className="text-sm text-fg-secondary space-y-1">
          <div><code className="text-amber-400">tutor study &lt;module&gt;</code> -- Learn a topic with explanations.</div>
          <div><code className="text-amber-400">tutor drill &lt;module&gt;</code> -- Practice with quiz questions.</div>
          <div><code className="text-amber-400">tutor mock &lt;type&gt;</code> -- Start a mock interview session.</div>
          <div><code className="text-amber-400">tutor plan</code> -- Generate or view your study plan.</div>
          <div><code className="text-amber-400">tutor status</code> -- Quick readiness check.</div>
        </div>
      </div>

      <div className="bg-surface rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-semibold text-fg">Recommended 8-Step Learning Flow</h3>
        <ol className="text-sm text-fg-secondary space-y-1.5 list-decimal list-inside">
          <li>Generate a study plan targeting your role and timeline.</li>
          <li>Study one topic at a time -- read the explanation carefully.</li>
          <li>Drill on the topic until you hit 3 correct answers in a row.</li>
          <li>Move to the next topic in the same module.</li>
          <li>After completing a module, review confidence gaps in Analytics.</li>
          <li>Re-drill any topics where actual score lags self-rated by &gt;20%.</li>
          <li>Run mock interviews weekly to build endurance.</li>
          <li>Review mock feedback and target weak dimensions.</li>
        </ol>
      </div>

      <div className="bg-surface rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-semibold text-fg">Mastery Criteria</h3>
        <ul className="text-sm text-fg-secondary space-y-1 list-disc list-inside">
          <li><strong className="text-fg">Not Started</strong> -- Topic has not been studied yet.</li>
          <li><strong className="text-blue-400">Learning</strong> -- Topic studied but not drilled.</li>
          <li><strong className="text-amber-400">Drilling</strong> -- Active practice, SM-2 review scheduled.</li>
          <li><strong className="text-emerald-400">Mastered</strong> -- Score &ge;80% with 3+ correct in a row, next review &gt;7 days out.</li>
        </ul>
      </div>
    </div>
  );
}

// --- Main Page ---
export function TutorPage() {
  usePerformance('TutorPage');
  const navigate = useNavigate();
  const { dashboard, topics, analytics, mocks, loading, error, activeTab, moduleFilter, setActiveTab, setModuleFilter, refresh } = useTutor();

  useEffect(() => { reportUsage('page.view', { page: 'tutor' }); }, []);

  const tabs = ['dashboard', 'analytics', 'topics', 'mocks', 'guide'] as const;

  return (
    <div data-testid="tutor-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-fg">Tutor</h2>
        <button onClick={refresh} className="px-3 py-1 text-xs rounded bg-elevated hover:bg-elevated text-fg-secondary transition-colors" data-testid="tutor-refresh">Refresh</button>
      </div>

      {/* Tab nav */}
      <nav className="tab-scroll flex gap-1 border-b border-border-subtle pb-px" data-testid="tutor-tabs" role="tablist" aria-label="Tutor sections">
        {tabs.map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            role="tab"
            aria-selected={activeTab === tab}
            aria-controls={`tutor-panel-${tab}`}
            className={`px-3 py-1.5 text-sm rounded-t transition-colors capitalize whitespace-nowrap ${activeTab === tab ? 'bg-surface text-fg' : 'text-fg-muted hover:text-fg-secondary'}`}
            data-testid={`tab-${tab}`}>
            {tab}
          </button>
        ))}
      </nav>

      {error && <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" role="alert" data-testid="tutor-error">{error}</div>}

      {/* Tab content */}
      {activeTab === 'dashboard' && dashboard && <DashboardTab dashboard={dashboard} />}
      {activeTab === 'analytics' && analytics && <AnalyticsTab analytics={analytics} />}
      {activeTab === 'topics' && <TopicsTab topics={topics} moduleFilter={moduleFilter} setModuleFilter={setModuleFilter} navigate={(p: string) => navigate(p)} />}
      {activeTab === 'mocks' && <MocksTab mocks={mocks} navigate={(p: string) => navigate(p)} />}
      {activeTab === 'guide' && <GuideTab />}
      {loading && !dashboard && !analytics && topics.length === 0 && mocks.length === 0 && (
        <div className="text-center py-8 text-fg-muted" role="status" aria-live="polite">Loading...</div>
      )}
    </div>
  );
}
