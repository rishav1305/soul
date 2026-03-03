import { useState, useRef, useEffect } from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { PlannerTask, TaskStage, TaskActivity, TaskComment } from '../../lib/types.ts';
import { ValidTransitions } from './transitions.ts';

function parseMetadata(meta: string): Record<string, unknown> {
  try { return meta ? JSON.parse(meta) : {}; } catch { return {}; }
}

const STAGE_LABELS: Record<TaskStage, string> = {
  backlog: 'Backlog',
  brainstorm: 'Brainstorm',
  active: 'Active',
  blocked: 'Blocked',
  validation: 'Validation',
  done: 'Done',
};

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-stage-backlog/15 text-stage-backlog',
  brainstorm: 'bg-stage-brainstorm/15 text-stage-brainstorm',
  active: 'bg-stage-active/15 text-stage-active',
  blocked: 'bg-stage-blocked/15 text-stage-blocked',
  validation: 'bg-stage-validation/15 text-stage-validation',
  done: 'bg-stage-done/15 text-stage-done',
};

const TRANSITION_BUTTON_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-stage-backlog/20 hover:bg-stage-backlog/30 text-stage-backlog',
  brainstorm: 'bg-stage-brainstorm/20 hover:bg-stage-brainstorm/30 text-stage-brainstorm',
  active: 'bg-stage-active/20 hover:bg-stage-active/30 text-stage-active',
  blocked: 'bg-stage-blocked/20 hover:bg-stage-blocked/30 text-stage-blocked',
  validation: 'bg-stage-validation/20 hover:bg-stage-validation/30 text-stage-validation',
  done: 'bg-stage-done/20 hover:bg-stage-done/30 text-stage-done',
};

const PRIORITY_LABELS: Record<number, string> = {
  0: 'Low',
  1: 'Normal',
  2: 'High',
  3: 'Critical',
};

const PRIORITY_COLORS: Record<number, string> = {
  0: 'text-priority-low',
  1: 'text-priority-normal',
  2: 'text-priority-high',
  3: 'text-priority-critical',
};

interface TaskDetailProps {
  task: PlannerTask;
  onClose: () => void;
  onMove: (id: number, stage: TaskStage, comment: string) => Promise<void>;
  onUpdate: (id: number, updates: Partial<PlannerTask>) => Promise<PlannerTask>;
  onDelete: (id: number) => Promise<void>;
  activities?: TaskActivity[];
  streamContent?: string;
  comments?: TaskComment[];
  onFetchComments?: (id: number) => Promise<any>;
  onAddComment?: (id: number, body: string) => Promise<TaskComment>;
}

export default function TaskDetail({ task, onClose, onMove, onUpdate, onDelete, activities = [], streamContent = '', comments = [], onFetchComments, onAddComment }: TaskDetailProps) {
  const transitions = ValidTransitions[task.stage];
  const meta = parseMetadata(task.metadata);
  const [autonomous, setAutonomous] = useState(!!meta.autonomous);
  const [editingProduct, setEditingProduct] = useState(false);
  const [productValue, setProductValue] = useState(task.product || '');
  const productInputRef = useRef<HTMLInputElement>(null);
  const streamEndRef = useRef<HTMLDivElement>(null);
  const [commentText, setCommentText] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const commentsEndRef = useRef<HTMLDivElement>(null);

  // Sync autonomous state when task prop changes (e.g., from WebSocket update).
  useEffect(() => {
    const m = parseMetadata(task.metadata);
    setAutonomous(!!m.autonomous);
  }, [task.metadata]);

  useEffect(() => {
    if (editingProduct && productInputRef.current) {
      productInputRef.current.focus();
    }
  }, [editingProduct]);

  // Auto-scroll the stream output as new content arrives.
  useEffect(() => {
    if (streamContent && streamEndRef.current) {
      streamEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [streamContent]);

  useEffect(() => {
    if (onFetchComments) {
      onFetchComments(task.id);
    }
  }, [task.id, onFetchComments]);

  useEffect(() => {
    commentsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [comments]);

  const toggleAutonomous = async () => {
    const next = !autonomous;
    setAutonomous(next);
    const newMeta = { ...meta, autonomous: next };
    await onUpdate(task.id, { metadata: JSON.stringify(newMeta) });
  };

  const saveProduct = async () => {
    const trimmed = productValue.trim();
    if (trimmed !== task.product) {
      await onUpdate(task.id, { product: trimmed });
    }
    setEditingProduct(false);
  };

  const handleSubmitComment = async () => {
    if (!commentText.trim() || !onAddComment) return;
    setSubmitting(true);
    try {
      await onAddComment(task.id, commentText.trim());
      setCommentText('');
    } catch (err) {
      console.error('Failed to add comment:', err);
    } finally {
      setSubmitting(false);
    }
  };

  const isProcessing = autonomous && streamContent.length > 0;
  const hasActivities = activities.length > 0;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-surface border border-border-default rounded-2xl shadow-2xl animate-fade-in-scale w-full max-w-2xl mx-4 max-h-[85vh] flex flex-col">
        {/* Header */}
        <div className="px-6 py-4 border-b border-border-subtle shrink-0">
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1 min-w-0">
              <h3 className="font-display text-lg font-semibold text-fg">{task.title}</h3>
              <div className="flex items-center gap-2 mt-1">
                <span className="text-fg-muted font-mono text-xs">#{task.id}</span>
                <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${STAGE_COLORS[task.stage]}`}>
                  {STAGE_LABELS[task.stage]}
                </span>
                <span className={`text-xs font-medium ${PRIORITY_COLORS[task.priority] ?? 'text-priority-low'}`}>
                  {PRIORITY_LABELS[task.priority] ?? 'Unknown'} priority
                </span>
                {/* Editable product badge */}
                {editingProduct ? (
                  <input
                    ref={productInputRef}
                    type="text"
                    value={productValue}
                    onChange={(e) => setProductValue(e.target.value)}
                    onBlur={saveProduct}
                    onKeyDown={(e) => { if (e.key === 'Enter') saveProduct(); if (e.key === 'Escape') setEditingProduct(false); }}
                    placeholder="product"
                    className="px-2 py-0.5 rounded text-xs bg-elevated border border-soul/50 text-fg outline-none w-24"
                  />
                ) : (
                  <button
                    type="button"
                    onClick={() => setEditingProduct(true)}
                    className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium cursor-pointer transition-colors ${task.product ? 'bg-overlay text-fg-secondary hover:bg-elevated' : 'border border-dashed border-border-default text-fg-muted hover:border-fg-muted'}`}
                  >
                    {task.product || '+ Product'}
                  </button>
                )}
                {/* Branch info for active/validation tasks */}
                {(task.stage === 'active' || task.stage === 'validation') && task.agent_id?.startsWith('auto-') && (
                  <span className="text-[10px] text-fg-muted font-mono">
                    branch: task/{task.id}-...
                  </span>
                )}
              </div>

              {/* Autonomous toggle */}
              <div className="flex items-center gap-2 mt-2">
                <button
                  type="button"
                  onClick={toggleAutonomous}
                  className={`relative w-10 h-5 rounded-full transition-colors cursor-pointer shrink-0 ${autonomous ? 'bg-soul' : 'bg-elevated border border-border-default'}`}
                >
                  <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full transition-all duration-200 ${autonomous ? 'translate-x-5 bg-deep' : 'translate-x-0 bg-fg-muted'}`} />
                </button>
                <span className="text-xs text-fg-secondary">Autonomous</span>
                {autonomous && (
                  <span className="text-[10px] text-soul font-medium">
                    {isProcessing ? 'Soul is working...' : 'Soul will work on this task'}
                  </span>
                )}
                {isProcessing && (
                  <span className="inline-block w-2 h-2 rounded-full bg-soul animate-pulse" />
                )}
              </div>

              {/* Workflow mode selector */}
              {autonomous && (
                <div className="flex items-center gap-2 mt-1">
                  <span className="text-xs text-fg-muted">Workflow:</span>
                  {(['quick', 'full'] as const).map((mode) => (
                    <button
                      key={mode}
                      type="button"
                      onClick={async () => {
                        const newMeta = { ...meta, workflow: mode };
                        await onUpdate(task.id, { metadata: JSON.stringify(newMeta) });
                      }}
                      className={`px-2 py-0.5 rounded text-[10px] font-medium transition-colors cursor-pointer ${
                        (meta.workflow || 'quick') === mode
                          ? 'bg-soul/20 text-soul'
                          : 'bg-elevated text-fg-muted hover:text-fg-secondary'
                      }`}
                    >
                      {mode}
                    </button>
                  ))}
                </div>
              )}
            </div>
            <button
              type="button"
              onClick={onClose}
              className="text-fg-muted hover:text-fg transition-colors cursor-pointer text-xl leading-none"
              aria-label="Close"
            >
              &times;
            </button>
          </div>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
          {/* Live streaming output — shown when agent is actively processing */}
          {streamContent && (
            <Section title="Live Output">
              <div className="bg-deep/60 rounded-lg p-3 border border-soul/20">
                <div className="prose prose-invert prose-sm prose-soul max-w-none">
                  <Markdown remarkPlugins={[remarkGfm]}>{streamContent}</Markdown>
                </div>
                <div ref={streamEndRef} />
                <div className="flex items-center gap-1.5 mt-2 pt-2 border-t border-border-subtle">
                  <span className="inline-block w-1.5 h-1.5 rounded-full bg-soul animate-pulse" />
                  <span className="text-[10px] text-soul font-medium">Processing...</span>
                </div>
              </div>
            </Section>
          )}

          {/* Activity log — tool calls, stage changes, status updates */}
          {hasActivities && (
            <Section title="Activity">
              <div className="space-y-1">
                {activities.map((a, i) => (
                  <ActivityEntry key={i} activity={a} />
                ))}
              </div>
            </Section>
          )}

          {task.stage === 'validation' && task.agent_id?.startsWith('auto-') && (
            <Section title="Review">
              <div className="flex items-center gap-2 text-sm">
                <span className="text-fg-secondary">Changes are live on the dev server:</span>
                <a
                  href="http://localhost:3001"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-soul hover:underline font-mono text-xs"
                >
                  localhost:3001
                </a>
              </div>
              <p className="text-[10px] text-fg-muted mt-1">
                Move to Done to merge to production (localhost:3000)
              </p>
            </Section>
          )}

          {task.description && (
            <Section title="Description">
              <p className="text-fg-secondary text-sm whitespace-pre-wrap">{task.description}</p>
            </Section>
          )}

          {task.acceptance && (
            <Section title="Acceptance Criteria">
              <p className="text-fg-secondary text-sm whitespace-pre-wrap">{task.acceptance}</p>
            </Section>
          )}

          {task.plan && (
            <Section title="Plan">
              <div className="prose prose-invert prose-sm prose-soul max-w-none">
                <Markdown remarkPlugins={[remarkGfm]}>{task.plan}</Markdown>
              </div>
            </Section>
          )}

          {task.output && !streamContent && (
            <Section title="Output">
              <div className="prose prose-invert prose-sm prose-soul max-w-none">
                <Markdown remarkPlugins={[remarkGfm]}>{task.output}</Markdown>
              </div>
            </Section>
          )}

          {task.error && (
            <Section title="Error">
              <p className="text-sm text-stage-blocked font-mono whitespace-pre-wrap">{task.error}</p>
            </Section>
          )}

          {task.blocker && (
            <Section title="Blocker">
              <p className="text-sm text-stage-blocked whitespace-pre-wrap">{task.blocker}</p>
            </Section>
          )}

          {/* Comment Thread */}
          <div className="border-t border-zinc-700/50 pt-4 mt-4">
            <h3 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-3">
              Comments
            </h3>

            <div className="space-y-3 max-h-64 overflow-y-auto mb-3">
              {(comments || []).map((c) => (
                <div
                  key={c.id}
                  className={`rounded-lg p-3 text-sm ${
                    c.author === 'user'
                      ? 'bg-blue-500/10 border border-blue-500/20'
                      : c.type === 'error'
                        ? 'bg-red-500/10 border border-red-500/20'
                        : c.type === 'verification'
                          ? 'bg-emerald-500/10 border border-emerald-500/20'
                          : 'bg-zinc-700/30 border border-zinc-600/20'
                  }`}
                >
                  <div className="flex items-center gap-2 mb-1">
                    <span className={`text-xs font-medium ${
                      c.author === 'user' ? 'text-blue-400' : 'text-zinc-400'
                    }`}>
                      {c.author === 'user' ? 'You' : 'Soul'}
                    </span>
                    <span className="text-xs text-zinc-500">
                      {new Date(c.created_at).toLocaleTimeString()}
                    </span>
                    {c.type !== 'feedback' && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-zinc-600/50 text-zinc-400">
                        {c.type}
                      </span>
                    )}
                  </div>
                  <p className="text-zinc-300 whitespace-pre-wrap">{c.body}</p>
                </div>
              ))}
              <div ref={commentsEndRef} />
            </div>

            {/* Comment input */}
            <div className="flex gap-2">
              <input
                type="text"
                value={commentText}
                onChange={(e) => setCommentText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    handleSubmitComment();
                  }
                }}
                placeholder="Post feedback..."
                className="flex-1 bg-zinc-800 border border-zinc-600/50 rounded-lg px-3 py-2 text-sm text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-500"
                disabled={submitting}
              />
              <button
                onClick={handleSubmitComment}
                disabled={submitting || !commentText.trim()}
                className="px-3 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-sm disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
              >
                {submitting ? '...' : 'Send'}
              </button>
            </div>
          </div>
        </div>

        {/* Actions */}
        <div className="px-6 py-4 border-t border-border-subtle shrink-0">
          <div className="flex flex-wrap items-center gap-2">
            {transitions.map((targetStage) => (
              <button
                key={targetStage}
                type="button"
                onClick={() => onMove(task.id, targetStage, '')}
                className={`px-3 py-1.5 text-sm font-medium rounded-md transition-colors cursor-pointer ${TRANSITION_BUTTON_COLORS[targetStage]}`}
              >
                Move to {STAGE_LABELS[targetStage]}
              </button>
            ))}
            <div className="flex-1" />
            <button
              type="button"
              onClick={() => onDelete(task.id)}
              className="px-3 py-1.5 text-sm font-medium rounded-md bg-stage-blocked/15 hover:bg-stage-blocked/25 text-stage-blocked transition-colors cursor-pointer"
            >
              Delete
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function ActivityEntry({ activity }: { activity: TaskActivity }) {
  const time = new Date(activity.time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });

  let icon: string;
  let color: string;
  let label: string;

  switch (activity.type) {
    case 'status':
      icon = '\u25C6'; // diamond
      color = 'text-soul';
      label = activity.content;
      break;
    case 'stage':
      icon = '\u2192'; // arrow
      color = 'text-stage-active';
      label = `Stage: ${activity.content}`;
      break;
    case 'tool_call': {
      let toolName = 'tool';
      try {
        const d = JSON.parse(activity.content);
        toolName = d.name || 'tool';
      } catch { /* */ }
      icon = '\u2699'; // gear
      color = 'text-fg-secondary';
      label = `Calling ${toolName}...`;
      break;
    }
    case 'tool_complete': {
      icon = '\u2713'; // check
      color = 'text-stage-done';
      label = 'Tool completed';
      break;
    }
    case 'tool_error': {
      icon = '\u2717'; // x
      color = 'text-stage-blocked';
      label = 'Tool error';
      break;
    }
    case 'done':
      icon = '\u2714'; // heavy check
      color = 'text-stage-done';
      label = 'Processing complete';
      break;
    default:
      icon = '\u2022'; // bullet
      color = 'text-fg-muted';
      label = activity.content;
  }

  return (
    <div className="flex items-start gap-2 text-xs">
      <span className="text-fg-muted font-mono shrink-0">{time}</span>
      <span className={`${color} shrink-0`}>{icon}</span>
      <span className="text-fg-secondary">{label}</span>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h4 className="font-display text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-1">{title}</h4>
      {children}
    </div>
  );
}
