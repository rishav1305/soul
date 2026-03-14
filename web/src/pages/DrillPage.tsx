import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router';
import { useDrill } from '../hooks/useDrill';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';

function scoreColor(score: number): string {
  if (score >= 80) return 'text-emerald-400';
  if (score >= 50) return 'text-amber-400';
  return 'text-red-400';
}

const difficultyColor: Record<string, string> = {
  easy: 'text-emerald-400',
  medium: 'text-amber-400',
  hard: 'text-red-400',
};

export function DrillPage() {
  usePerformance('DrillPage');
  const { id } = useParams();
  const topicId = Number(id);
  const navigate = useNavigate();
  const { question, evaluation, loading, error, answered, correct, startDrill, submitAnswer, nextQuestion } = useDrill();
  const [answer, setAnswer] = useState('');

  useEffect(() => {
    reportUsage('page.view', { page: 'drill', topicId });
    startDrill(topicId);
  }, [topicId, startDrill]);

  const handleSubmit = async () => {
    if (!question || !answer.trim() || evaluation) return;
    await submitAnswer(question.id, answer.trim());
  };

  const handleNext = () => {
    setAnswer('');
    nextQuestion(topicId);
  };

  const sessionScore = answered > 0 ? Math.round((correct / answered) * 100) : 0;

  return (
    <div data-testid="drill-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Back button */}
      <button
        onClick={() => navigate('/tutor')}
        className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
        data-testid="drill-back"
      >
        &larr; Back to Tutor
      </button>

      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-zinc-100" data-testid="drill-header">
          Drill — Topic #{topicId}
        </h2>
        {/* Session stats */}
        <div className="flex items-center gap-3 text-sm" data-testid="drill-stats">
          <span className="text-zinc-400">
            Answered: <span className="text-zinc-100 font-medium">{answered}</span>
          </span>
          <span className="text-zinc-400">
            Correct: <span className="text-emerald-400 font-medium">{correct}</span>
          </span>
          {answered > 0 && (
            <span className={`font-bold ${scoreColor(sessionScore)}`} data-testid="drill-score">
              {sessionScore}%
            </span>
          )}
        </div>
      </div>

      {error && <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="drill-error">{error}</div>}

      {/* Question */}
      {question && (
        <div className="bg-zinc-800 rounded-lg p-4 space-y-4" data-testid="drill-question-card">
          <div className="flex items-center justify-between">
            <span className="text-xs text-zinc-500">Question #{question.id}</span>
            <span className={`text-xs font-medium ${difficultyColor[question.difficulty] ?? 'text-zinc-400'}`}>
              {question.difficulty}
            </span>
          </div>
          <p className="text-sm text-zinc-100 leading-relaxed" data-testid="drill-question-text">
            {question.question_text}
          </p>

          {/* Answer input */}
          <div>
            <textarea
              value={answer}
              onChange={e => setAnswer(e.target.value)}
              placeholder="Type your answer..."
              rows={4}
              disabled={!!evaluation || loading}
              className="w-full bg-zinc-900 rounded px-3 py-2 text-sm text-zinc-100 placeholder:text-zinc-500 outline-none focus:ring-1 focus:ring-zinc-600 resize-none disabled:opacity-50"
              data-testid="drill-answer-input"
              onKeyDown={e => {
                if (e.key === 'Enter' && e.metaKey) handleSubmit();
              }}
            />
          </div>

          {/* Submit button */}
          {!evaluation && (
            <div className="flex justify-end">
              <button
                onClick={handleSubmit}
                disabled={loading || !answer.trim()}
                className="px-4 py-2 text-sm rounded-lg bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
                data-testid="drill-submit"
              >
                {loading ? 'Evaluating...' : 'Submit Answer'}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Evaluation */}
      {evaluation && (
        <div className="bg-zinc-800 rounded-lg p-4 space-y-4" data-testid="drill-evaluation">
          {/* Correct/Incorrect badge */}
          <div className="flex items-center gap-3">
            <span
              className={`px-3 py-1 text-sm rounded-full font-medium ${evaluation.correct ? 'bg-emerald-500/20 text-emerald-400' : 'bg-red-500/20 text-red-400'}`}
              data-testid="drill-result-badge"
            >
              {evaluation.correct ? 'Correct' : 'Incorrect'}
            </span>
            <span className={`text-sm font-bold ${scoreColor(evaluation.score)}`} data-testid="drill-eval-score">
              {evaluation.score}%
            </span>
          </div>

          {/* Explanation */}
          <div>
            <h4 className="text-xs font-medium text-zinc-400 mb-1">Explanation</h4>
            <p className="text-sm text-zinc-300 leading-relaxed" data-testid="drill-explanation">{evaluation.explanation}</p>
          </div>

          {/* Keywords */}
          {evaluation.expected_keywords.length > 0 && (
            <div>
              <h4 className="text-xs font-medium text-zinc-400 mb-1">Expected Keywords</h4>
              <div className="flex flex-wrap gap-1">
                {evaluation.expected_keywords.map(kw => {
                  const matched = evaluation.matched_keywords.includes(kw);
                  return (
                    <span key={kw} className={`px-2 py-0.5 text-xs rounded-full ${matched ? 'bg-emerald-500/20 text-emerald-400' : 'bg-zinc-700 text-zinc-400'}`}>
                      {kw}
                    </span>
                  );
                })}
              </div>
            </div>
          )}

          {/* Next review */}
          <div className="text-xs text-zinc-500" data-testid="drill-next-review">
            Next review: {new Date(evaluation.next_review).toLocaleDateString()}
          </div>

          {/* Next question */}
          <div className="flex justify-end">
            <button
              onClick={handleNext}
              disabled={loading}
              className="px-4 py-2 text-sm rounded-lg bg-zinc-700 hover:bg-zinc-600 text-zinc-100 transition-colors disabled:opacity-50"
              data-testid="drill-next"
            >
              Next Question
            </button>
          </div>
        </div>
      )}

      {/* Loading state */}
      {loading && !question && (
        <div className="text-center py-8 text-zinc-400">Loading question...</div>
      )}
    </div>
  );
}
