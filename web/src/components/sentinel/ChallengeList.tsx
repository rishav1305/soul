import { useState } from 'react';
import type { SentinelChallenge } from '../../hooks/useSentinel';

const difficultyColor: Record<string, string> = {
  easy: 'bg-emerald-500/20 text-emerald-400',
  medium: 'bg-amber-500/20 text-amber-400',
  hard: 'bg-red-500/20 text-red-400',
  expert: 'bg-purple-500/20 text-purple-400',
};

const categoryColor: Record<string, string> = {
  injection: 'bg-blue-500/20 text-blue-400',
  jailbreak: 'bg-orange-500/20 text-orange-400',
  exfiltration: 'bg-red-500/20 text-red-400',
  social: 'bg-pink-500/20 text-pink-400',
  evasion: 'bg-cyan-500/20 text-cyan-400',
  default: 'bg-overlay text-fg-secondary',
};

interface ChallengeListProps {
  challenges: SentinelChallenge[];
  onStart: (id: number) => void;
}

export function ChallengeList({ challenges, onStart }: ChallengeListProps) {
  const [categoryFilter, setCategoryFilter] = useState<string>('all');
  const [difficultyFilter, setDifficultyFilter] = useState<string>('all');

  const categories = ['all', ...new Set(challenges.map(c => c.category))];
  const difficulties = ['all', ...new Set(challenges.map(c => c.difficulty))];

  const filtered = challenges.filter(c => {
    if (categoryFilter !== 'all' && c.category !== categoryFilter) return false;
    if (difficultyFilter !== 'all' && c.difficulty !== difficultyFilter) return false;
    return true;
  });

  return (
    <div className="space-y-4" data-testid="challenge-list">
      {/* Filters */}
      <div className="flex gap-3 flex-wrap">
        <select
          value={categoryFilter}
          onChange={e => setCategoryFilter(e.target.value)}
          className="soul-select"
          data-testid="filter-category"
        >
          {categories.map(c => (
            <option key={c} value={c}>{c === 'all' ? 'All Categories' : c}</option>
          ))}
        </select>
        <select
          value={difficultyFilter}
          onChange={e => setDifficultyFilter(e.target.value)}
          className="soul-select"
          data-testid="filter-difficulty"
        >
          {difficulties.map(d => (
            <option key={d} value={d}>{d === 'all' ? 'All Difficulties' : d}</option>
          ))}
        </select>
      </div>

      {/* Challenge grid */}
      {filtered.length === 0 ? (
        <div className="text-sm text-fg-muted py-4">No challenges match the current filters.</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {filtered.map(challenge => (
            <button
              key={challenge.id}
              onClick={() => onStart(challenge.id)}
              className="bg-surface rounded-lg p-4 text-left hover:bg-elevated transition-colors group"
              data-testid={`challenge-card-${challenge.id}`}
            >
              <div className="flex items-start justify-between mb-2">
                <h3 className="text-sm font-medium text-fg group-hover:text-soul transition-colors truncate pr-2">
                  {challenge.title}
                </h3>
                {challenge.completed && (
                  <span className="shrink-0 w-5 h-5 flex items-center justify-center rounded-full bg-emerald-500/20 text-emerald-400 text-xs" data-testid={`challenge-complete-${challenge.id}`}>
                    &#10003;
                  </span>
                )}
              </div>

              <div className="flex flex-wrap gap-1.5 mb-3">
                <span className={`px-2 py-0.5 text-[10px] rounded-full ${categoryColor[challenge.category] ?? categoryColor.default}`}>
                  {challenge.category}
                </span>
                <span className={`px-2 py-0.5 text-[10px] rounded-full ${difficultyColor[challenge.difficulty] ?? difficultyColor.easy}`}>
                  {challenge.difficulty}
                </span>
              </div>

              <div className="flex items-center justify-between">
                <span className="text-xs text-fg-muted">{challenge.points} pts</span>
                <span className={`text-xs ${challenge.completed ? 'text-emerald-400' : 'text-fg-secondary'}`}>
                  {challenge.completed ? 'Completed' : 'Start'}
                </span>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
