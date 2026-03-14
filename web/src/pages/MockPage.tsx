import { useEffect } from 'react';
import { useParams, useNavigate } from 'react-router';
import { useMockSession } from '../hooks/useMockSession';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';

function scoreColor(score: number): string {
  if (score >= 80) return 'text-emerald-400';
  if (score >= 50) return 'text-amber-400';
  return 'text-red-400';
}

function scoreBarColor(score: number): string {
  if (score >= 80) return 'bg-emerald-500';
  if (score >= 50) return 'bg-amber-500';
  return 'bg-red-500';
}

export function MockPage() {
  usePerformance('MockPage');
  const { id } = useParams();
  const navigate = useNavigate();
  const { session, loading, error } = useMockSession(id!);

  useEffect(() => { reportUsage('page.view', { page: 'mock', sessionId: id }); }, [id]);

  if (loading) {
    return (
      <div data-testid="mock-page" className="h-full overflow-y-auto p-4 sm:p-6">
        <div className="text-center py-8 text-zinc-400">Loading session...</div>
      </div>
    );
  }

  if (error || !session) {
    return (
      <div data-testid="mock-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
        <button
          onClick={() => navigate('/tutor')}
          className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
          data-testid="mock-back"
        >
          &larr; Back to Tutor
        </button>
        <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="mock-error">
          {error || 'Session not found'}
        </div>
      </div>
    );
  }

  const overallScore = session.overall_score != null ? Math.round(session.overall_score) : null;
  let feedback: string | null = null;
  try {
    const parsed = JSON.parse(session.feedback_json);
    feedback = typeof parsed === 'string' ? parsed : JSON.stringify(parsed, null, 2);
  } catch {
    feedback = session.feedback_json || null;
  }

  return (
    <div data-testid="mock-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Back button */}
      <button
        onClick={() => navigate('/tutor')}
        className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
        data-testid="mock-back"
      >
        &larr; Back to Tutor
      </button>

      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-zinc-100" data-testid="mock-header">
            Mock Interview #{session.id}
          </h2>
          <div className="flex items-center gap-2 mt-1">
            <span className="px-2 py-0.5 text-xs rounded-full bg-zinc-600 text-zinc-300 capitalize" data-testid="mock-type">
              {session.type.replace('_', ' ')}
            </span>
            <span className="text-xs text-zinc-500">
              {new Date(session.started_at).toLocaleDateString()}
            </span>
          </div>
        </div>
        {overallScore != null && (
          <span className={`text-2xl font-bold ${scoreColor(overallScore)}`} data-testid="mock-overall-score">
            {overallScore}%
          </span>
        )}
      </div>

      {/* JD summary */}
      {session.job_description && (
        <div className="bg-zinc-800 rounded-lg p-4" data-testid="mock-jd">
          <h3 className="text-xs font-medium text-zinc-400 mb-2">Job Description</h3>
          <p className="text-sm text-zinc-300 leading-relaxed whitespace-pre-wrap">{session.job_description}</p>
        </div>
      )}

      {/* Completed session details */}
      {session.completed_at ? (
        <div className="space-y-4">
          {/* Dimension scores */}
          {session.scores && session.scores.length > 0 && (
            <div className="bg-zinc-800 rounded-lg p-4" data-testid="mock-dimension-scores">
              <h3 className="text-xs font-medium text-zinc-400 mb-3">Dimension Scores</h3>
              <div className="space-y-3">
                {session.scores.map(dim => {
                  const pct = Math.round(dim.score);
                  return (
                    <div key={dim.dimension} data-testid={`dimension-${dim.dimension}`}>
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-sm text-zinc-300">{dim.dimension}</span>
                        <span className={`text-sm font-medium ${scoreColor(pct)}`}>{pct}%</span>
                      </div>
                      <div className="w-full h-1.5 bg-zinc-700 rounded-full overflow-hidden">
                        <div className={`h-full rounded-full transition-all ${scoreBarColor(pct)}`} style={{ width: `${pct}%` }} />
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* Feedback */}
          {feedback && (
            <div className="bg-zinc-800 rounded-lg p-4" data-testid="mock-feedback">
              <h3 className="text-xs font-medium text-zinc-400 mb-2">Feedback</h3>
              <pre className="text-sm text-zinc-300 leading-relaxed whitespace-pre-wrap font-sans">{feedback}</pre>
            </div>
          )}

          {/* Completed timestamp */}
          <div className="text-xs text-zinc-500">
            Completed: {new Date(session.completed_at).toLocaleString()}
          </div>
        </div>
      ) : (
        <div className="bg-zinc-800 rounded-lg p-6 text-center" data-testid="mock-in-progress">
          <div className="text-amber-400 text-sm font-medium mb-1">Session In Progress</div>
          <p className="text-xs text-zinc-400">This mock interview has not been completed yet.</p>
        </div>
      )}
    </div>
  );
}
