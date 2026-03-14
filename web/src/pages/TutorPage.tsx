import { useEffect } from 'react';
import { useNavigate } from 'react-router';
import { useTutor } from '../hooks/useTutor';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import type { TutorDashboard, TutorTopic, TutorAnalytics, TutorMockSession } from '../lib/types';

// Color helpers
const statusColor: Record<string, string> = {
  not_started: 'bg-zinc-600 text-zinc-300',
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
      <div className="bg-zinc-800 rounded-lg p-4 space-y-3">
        <div className="flex items-center justify-between">
          <span className="text-sm text-zinc-400">Interview Readiness</span>
          <span className={`text-lg font-bold ${scoreColor(readinessPercent)}`} data-testid="readiness-score">{readinessPercent}%</span>
        </div>
        <div className="w-full h-2 bg-zinc-700 rounded-full overflow-hidden">
          <div className="h-full bg-emerald-500 rounded-full transition-all" style={{ width: `${readinessPercent}%` }} data-testid="readiness-bar" />
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
        <h3 className="text-sm font-medium text-zinc-400 mb-3">Modules</h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3" data-testid="module-grid">
          {(dashboard.moduleStats ?? []).map(mod => {
            const completionPercent = Math.round(mod.completionPct);
            return (
              <div key={mod.module} className="bg-zinc-800 rounded-lg p-4 space-y-2" data-testid={`module-card-${mod.module}`}>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium text-zinc-100 capitalize">{mod.module}</span>
                  <span className={`text-xs font-medium ${scoreColor(completionPercent)}`}>{completionPercent}%</span>
                </div>
                <div className="w-full h-1.5 bg-zinc-700 rounded-full overflow-hidden">
                  <div className="h-full bg-emerald-500 rounded-full transition-all" style={{ width: `${completionPercent}%` }} />
                </div>
                <div className="flex items-center justify-between text-xs text-zinc-400">
                  <span>{mod.completed}/{mod.topicCount} completed</span>
                  <span>{mod.inProgress} in progress</span>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Today's activity */}
      <div className="bg-zinc-800 rounded-lg p-4" data-testid="today-activity">
        <h3 className="text-sm font-medium text-zinc-400 mb-3">Today</h3>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div className="text-center">
            <div className="text-lg font-bold text-zinc-100">{formatTime(todaySummary.time)}</div>
            <div className="text-xs text-zinc-400">Time</div>
          </div>
          <div className="text-center">
            <div className="text-lg font-bold text-zinc-100">{todaySummary.sessions}</div>
            <div className="text-xs text-zinc-400">Sessions</div>
          </div>
          <div className="text-center">
            <div className="text-lg font-bold text-zinc-100">{todaySummary.questions}</div>
            <div className="text-xs text-zinc-400">Questions</div>
          </div>
          <div className="text-center">
            <div className={`text-lg font-bold ${scoreColor(Math.round(todayAvgScore))}`}>
              {todayAvgScore > 0 ? `${Math.round(todayAvgScore)}%` : '--'}
            </div>
            <div className="text-xs text-zinc-400">Avg Score</div>
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
        <h3 className="text-sm font-medium text-zinc-400 mb-3">Daily Activity</h3>
        {activityRows.length === 0 ? (
          <div className="text-sm text-zinc-500">No activity recorded yet.</div>
        ) : (
          <div className="bg-zinc-800 rounded-lg overflow-hidden">
            <table className="w-full text-sm" data-testid="activity-table">
              <thead>
                <tr className="text-xs text-zinc-400 border-b border-zinc-700">
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
                  <tr key={`${row.date}-${row.module}-${i}`} className="border-b border-zinc-700/50 text-zinc-300">
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
        <h3 className="text-sm font-medium text-zinc-400 mb-3">Confidence Gaps</h3>
        {gaps.length === 0 ? (
          <div className="text-sm text-zinc-500">No confidence gaps detected.</div>
        ) : (
          <div className="bg-zinc-800 rounded-lg overflow-hidden">
            <table className="w-full text-sm" data-testid="confidence-gaps-table">
              <thead>
                <tr className="text-xs text-zinc-400 border-b border-zinc-700">
                  <th className="text-left px-3 py-2 font-medium">Topic</th>
                  <th className="text-right px-3 py-2 font-medium">Self-Rated</th>
                  <th className="text-right px-3 py-2 font-medium">Actual</th>
                  <th className="text-right px-3 py-2 font-medium">Gap</th>
                </tr>
              </thead>
              <tbody>
                {gaps.map(gap => (
                  <tr key={gap.topicId} className="border-b border-zinc-700/50 text-zinc-300">
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
            className={`px-3 py-1 text-xs rounded-full transition-colors ${moduleFilter === mod ? 'bg-zinc-600 text-zinc-100' : 'bg-zinc-800 text-zinc-400 hover:text-zinc-200'}`}
            data-testid={`filter-${mod || 'all'}`}
          >
            {mod || 'All'}
          </button>
        ))}
      </div>

      {/* Topic rows */}
      {topics.length === 0 ? (
        <div className="text-sm text-zinc-500 py-4">No topics found.</div>
      ) : (
        <div className="space-y-1" data-testid="topics-list">
          {topics.map(topic => (
            <button
              key={topic.id}
              onClick={() => navigate(`/tutor/drill/${topic.id}`)}
              className="w-full flex items-center justify-between p-3 rounded-lg bg-zinc-800 hover:bg-zinc-700 transition-colors text-left"
              data-testid={`topic-${topic.id}`}
            >
              <div className="min-w-0">
                <div className="text-sm text-zinc-100 truncate">{topic.name}</div>
                <div className="text-xs text-zinc-500">{topic.category}</div>
              </div>
              <div className="flex items-center gap-2 shrink-0 ml-3">
                <span className={`text-xs font-medium ${difficultyColor[topic.difficulty] ?? 'text-zinc-400'}`}>
                  {topic.difficulty}
                </span>
                <span className={`px-2 py-0.5 text-xs rounded-full ${statusColor[topic.status] ?? 'bg-zinc-600 text-zinc-300'}`}>
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
        <div className="text-sm text-zinc-500 py-4">No mock sessions yet. Start one via chat.</div>
      ) : (
        mocks.map(session => {
          const scorePercent = session.overall_score != null ? Math.round(session.overall_score) : null;
          return (
            <button
              key={session.id}
              onClick={() => navigate(`/tutor/mock/${session.id}`)}
              className="w-full bg-zinc-800 rounded-lg p-4 hover:bg-zinc-700 transition-colors text-left space-y-2"
              data-testid={`mock-${session.id}`}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="px-2 py-0.5 text-xs rounded-full bg-zinc-600 text-zinc-300 capitalize">{session.type.replace('_', ' ')}</span>
                  <span className="text-xs text-zinc-500">{new Date(session.started_at).toLocaleDateString()}</span>
                </div>
                {scorePercent != null ? (
                  <span className={`text-sm font-bold ${scoreColor(scorePercent)}`} data-testid={`mock-score-${session.id}`}>{scorePercent}%</span>
                ) : (
                  <span className="text-xs text-zinc-500">In progress</span>
                )}
              </div>
              {scorePercent != null && (
                <div className="w-full h-1.5 bg-zinc-700 rounded-full overflow-hidden">
                  <div className="h-full bg-emerald-500 rounded-full transition-all" style={{ width: `${scorePercent}%` }} />
                </div>
              )}
              {session.job_description && (
                <div className="text-xs text-zinc-400 truncate">{session.job_description.slice(0, 120)}</div>
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
      <div className="bg-zinc-800 rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-semibold text-zinc-100">Tabs Overview</h3>
        <ul className="text-sm text-zinc-300 space-y-1.5 list-disc list-inside">
          <li><strong className="text-zinc-100">Dashboard</strong> -- Readiness score, module progress, and today's activity.</li>
          <li><strong className="text-zinc-100">Analytics</strong> -- Daily activity history and confidence gap analysis.</li>
          <li><strong className="text-zinc-100">Topics</strong> -- Browse topics by module and start drill sessions.</li>
          <li><strong className="text-zinc-100">Mocks</strong> -- View mock interview sessions and scores.</li>
          <li><strong className="text-zinc-100">Guide</strong> -- This page. Learning flow and mastery criteria.</li>
        </ul>
      </div>

      <div className="bg-zinc-800 rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-semibold text-zinc-100">Chat Commands</h3>
        <div className="text-sm text-zinc-300 space-y-1">
          <div><code className="text-amber-400">tutor study &lt;module&gt;</code> -- Learn a topic with explanations.</div>
          <div><code className="text-amber-400">tutor drill &lt;module&gt;</code> -- Practice with quiz questions.</div>
          <div><code className="text-amber-400">tutor mock &lt;type&gt;</code> -- Start a mock interview session.</div>
          <div><code className="text-amber-400">tutor plan</code> -- Generate or view your study plan.</div>
          <div><code className="text-amber-400">tutor status</code> -- Quick readiness check.</div>
        </div>
      </div>

      <div className="bg-zinc-800 rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-semibold text-zinc-100">Recommended 8-Step Learning Flow</h3>
        <ol className="text-sm text-zinc-300 space-y-1.5 list-decimal list-inside">
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

      <div className="bg-zinc-800 rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-semibold text-zinc-100">Mastery Criteria</h3>
        <ul className="text-sm text-zinc-300 space-y-1 list-disc list-inside">
          <li><strong className="text-zinc-100">Not Started</strong> -- Topic has not been studied yet.</li>
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
        <h2 className="text-lg font-semibold text-zinc-100">Tutor</h2>
        <button onClick={refresh} className="px-3 py-1 text-xs rounded bg-zinc-700 hover:bg-zinc-600 text-zinc-300 transition-colors" data-testid="tutor-refresh">Refresh</button>
      </div>

      {/* Tab nav */}
      <nav className="flex gap-1 border-b border-zinc-700 pb-px" data-testid="tutor-tabs">
        {tabs.map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className={`px-3 py-1.5 text-sm rounded-t transition-colors capitalize ${activeTab === tab ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-200'}`}
            data-testid={`tab-${tab}`}>
            {tab}
          </button>
        ))}
      </nav>

      {error && <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="tutor-error">{error}</div>}

      {/* Tab content */}
      {activeTab === 'dashboard' && dashboard && <DashboardTab dashboard={dashboard} />}
      {activeTab === 'analytics' && analytics && <AnalyticsTab analytics={analytics} />}
      {activeTab === 'topics' && <TopicsTab topics={topics} moduleFilter={moduleFilter} setModuleFilter={setModuleFilter} navigate={(p: string) => navigate(p)} />}
      {activeTab === 'mocks' && <MocksTab mocks={mocks} navigate={(p: string) => navigate(p)} />}
      {activeTab === 'guide' && <GuideTab />}
      {loading && !dashboard && !analytics && topics.length === 0 && mocks.length === 0 && (
        <div className="text-center py-8 text-zinc-400">Loading...</div>
      )}
    </div>
  );
}
