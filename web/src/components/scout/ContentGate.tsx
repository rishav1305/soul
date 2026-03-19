import { useState } from 'react';
import { api } from '../../lib/api';

interface ContentGateProps {
  onAction?: (action: string) => void;
}

interface TopicResult {
  topic: string;
  angle: string;
  audience: string;
}

interface SeriesResult {
  linkedin_posts: string[];
  x_posts: string[];
  carousel_outline: string;
}

interface HookResult {
  hooks: string[];
}

type ExpandedSection = 'topics' | 'series' | 'hooks';

export function ContentGate({ onAction }: ContentGateProps) {
  const [weekSummary, setWeekSummary] = useState('');
  const [topicInput, setTopicInput] = useState('');
  const [topics, setTopics] = useState<TopicResult[]>([]);
  const [series, setSeries] = useState<SeriesResult | null>(null);
  const [hooks, setHooks] = useState<HookResult | null>(null);
  const [selectedTopic, setSelectedTopic] = useState<TopicResult | null>(null);
  const [selectedDraft, setSelectedDraft] = useState('');
  const [loading, setLoading] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<ExpandedSection>>(new Set(['topics']));

  const toggle = (section: ExpandedSection) => {
    setExpanded(prev => {
      const next = new Set(prev);
      if (next.has(section)) {
        next.delete(section);
      } else {
        next.add(section);
      }
      return next;
    });
  };

  const handleGenerateTopics = async () => {
    if (!weekSummary.trim()) return;
    setLoading('topics');
    setError(null);
    try {
      const result = await api.post<{ topics: TopicResult[] }>('/api/ai/content-topic', {
        week_summary: weekSummary,
      });
      setTopics(result?.topics ?? []);
      setExpanded(prev => new Set(prev).add('topics'));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(null);
    }
  };

  const handleCreateSeries = async (topic: TopicResult) => {
    setLoading('series');
    setError(null);
    setSelectedTopic(topic);
    try {
      const result = await api.post<SeriesResult>('/api/ai/content-series', {
        topic: topic.topic,
        insights: topic.angle,
      });
      setSeries(result);
      setExpanded(prev => new Set(prev).add('series'));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(null);
    }
  };

  const handleGenerateHooks = async (draft: string) => {
    setLoading('hooks');
    setError(null);
    setSelectedDraft(draft);
    try {
      const result = await api.post<HookResult>('/api/ai/hook-writer', { draft });
      setHooks(result);
      setExpanded(prev => new Set(prev).add('hooks'));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(null);
    }
  };

  return (
    <div className="space-y-4" data-testid="content-gate">
      {error && (
        <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="content-gate-error">
          {error}
        </div>
      )}

      {/* Topic Generation */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('topics')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="content-section-topics"
        >
          <span>Generate Topics</span>
          <span className="text-fg-muted text-xs">{expanded.has('topics') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('topics') && (
          <div className="px-4 pb-4 space-y-3">
            <div>
              <label className="text-xs text-fg-muted block mb-1">Week Summary</label>
              <textarea
                value={weekSummary}
                onChange={e => setWeekSummary(e.target.value)}
                placeholder="What happened this week? Key learnings, wins, challenges..."
                className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-soul/50 resize-none"
                rows={3}
                data-testid="content-week-summary"
              />
            </div>
            <div>
              <label className="text-xs text-fg-muted block mb-1">Topic (optional override)</label>
              <input
                type="text"
                value={topicInput}
                onChange={e => setTopicInput(e.target.value)}
                placeholder="Specific topic to explore..."
                className="w-full bg-elevated rounded px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-soul/50"
                data-testid="content-topic-input"
              />
            </div>
            <button
              onClick={handleGenerateTopics}
              disabled={loading === 'topics' || !weekSummary.trim()}
              className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
              data-testid="content-generate-topics-btn"
            >
              {loading === 'topics' ? 'Generating...' : 'Generate Topics'}
            </button>

            {topics.length > 0 && (
              <div className="space-y-2 mt-3">
                <h4 className="text-xs text-fg-muted font-medium">Topic Ideas</h4>
                {topics.map((topic, i) => (
                  <div key={i} className="bg-elevated rounded-lg p-3" data-testid={`content-topic-card-${i}`}>
                    <div className="text-sm font-medium text-fg">{topic.topic}</div>
                    <div className="text-xs text-fg-muted mt-1">{topic.angle}</div>
                    <div className="text-xs text-fg-secondary mt-0.5">Audience: {topic.audience}</div>
                    <button
                      onClick={() => handleCreateSeries(topic)}
                      disabled={loading === 'series'}
                      className="mt-2 px-3 py-1 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 disabled:opacity-50 transition-colors"
                      data-testid={`content-create-series-btn-${i}`}
                    >
                      {loading === 'series' && selectedTopic === topic ? 'Generating...' : 'Create Series'}
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Series Output */}
      {series && selectedTopic && (
        <div className="bg-surface rounded-lg overflow-hidden">
          <button
            onClick={() => toggle('series')}
            className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
            data-testid="content-section-series"
          >
            <span>Content Series: {selectedTopic.topic}</span>
            <span className="text-fg-muted text-xs">{expanded.has('series') ? 'Collapse' : 'Expand'}</span>
          </button>
          {expanded.has('series') && (
            <div className="px-4 pb-4 space-y-4">
              {/* LinkedIn Posts */}
              <div>
                <h4 className="text-xs text-fg-muted font-medium mb-2">LinkedIn Posts</h4>
                <div className="space-y-2">
                  {series.linkedin_posts.map((post, i) => (
                    <div key={i} className="bg-elevated rounded-lg p-3" data-testid={`content-linkedin-post-${i}`}>
                      <pre className="text-xs text-fg whitespace-pre-wrap font-sans">{post}</pre>
                      <button
                        onClick={() => handleGenerateHooks(post)}
                        disabled={loading === 'hooks'}
                        className="mt-2 px-3 py-1 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 disabled:opacity-50 transition-colors"
                        data-testid={`content-hooks-linkedin-btn-${i}`}
                      >
                        {loading === 'hooks' && selectedDraft === post ? 'Generating...' : 'Generate Hooks'}
                      </button>
                    </div>
                  ))}
                </div>
              </div>

              {/* X Posts */}
              <div>
                <h4 className="text-xs text-fg-muted font-medium mb-2">X Posts</h4>
                <div className="space-y-2">
                  {series.x_posts.map((post, i) => (
                    <div key={i} className="bg-elevated rounded-lg p-3" data-testid={`content-x-post-${i}`}>
                      <pre className="text-xs text-fg whitespace-pre-wrap font-sans">{post}</pre>
                      <button
                        onClick={() => handleGenerateHooks(post)}
                        disabled={loading === 'hooks'}
                        className="mt-2 px-3 py-1 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 disabled:opacity-50 transition-colors"
                        data-testid={`content-hooks-x-btn-${i}`}
                      >
                        {loading === 'hooks' && selectedDraft === post ? 'Generating...' : 'Generate Hooks'}
                      </button>
                    </div>
                  ))}
                </div>
              </div>

              {/* Carousel Outline */}
              <div>
                <h4 className="text-xs text-fg-muted font-medium mb-2">Carousel Outline</h4>
                <div className="bg-elevated rounded-lg p-3" data-testid="content-carousel-outline">
                  <pre className="text-xs text-fg whitespace-pre-wrap font-sans">{series.carousel_outline}</pre>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Hooks Output */}
      {hooks && (
        <div className="bg-surface rounded-lg overflow-hidden">
          <button
            onClick={() => toggle('hooks')}
            className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
            data-testid="content-section-hooks"
          >
            <span>Hook Alternatives ({hooks.hooks.length})</span>
            <span className="text-fg-muted text-xs">{expanded.has('hooks') ? 'Collapse' : 'Expand'}</span>
          </button>
          {expanded.has('hooks') && (
            <div className="px-4 pb-4 space-y-2">
              {hooks.hooks.map((hook, i) => (
                <div key={i} className="bg-elevated rounded-lg p-3" data-testid={`content-hook-${i}`}>
                  <pre className="text-xs text-fg whitespace-pre-wrap font-sans">{hook}</pre>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Footer Actions */}
      <div className="flex items-center gap-2 pt-2 border-t border-border-subtle">
        <button
          onClick={() => onAction?.('publish')}
          className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 transition-colors"
          data-testid="content-publish-btn"
        >
          Publish
        </button>
        <button
          onClick={() => onAction?.('schedule')}
          className="px-3 py-1.5 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
          data-testid="content-schedule-btn"
        >
          Schedule
        </button>
        <button
          onClick={() => onAction?.('save-draft')}
          className="px-3 py-1.5 text-xs rounded bg-elevated text-fg-secondary hover:bg-overlay transition-colors"
          data-testid="content-save-draft-btn"
        >
          Save Draft
        </button>
      </div>
    </div>
  );
}
