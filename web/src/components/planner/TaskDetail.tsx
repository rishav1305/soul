import { useState, useRef, useEffect } from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { PlannerTask, TaskStage, TaskActivity, TaskComment, ProductInfo } from '../../lib/types.ts';

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

const ALL_STAGES: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];

const PRIORITY_OPTIONS = [
  { value: 0, label: 'Low' },
  { value: 1, label: 'Normal' },
  { value: 2, label: 'High' },
  { value: 3, label: 'Critical' },
];

const PRIORITY_COLORS: Record<number, string> = {
  0: 'text-priority-low',
  1: 'text-priority-normal',
  2: 'text-priority-high',
  3: 'text-priority-critical',
};

type DetailTab = 'task' | 'plan' | 'implementation' | 'comments';

interface TaskDetailProps {
  task: PlannerTask;
  onClose: () => void;
  onMove: (id: number, stage: TaskStage, comment: string) => Promise<void>;
  onUpdate: (id: number, updates: Partial<PlannerTask>) => Promise<PlannerTask>;
  onDelete: (id: number) => Promise<void>;
  activities?: TaskActivity[];
  streamContent?: string;
  products?: string[];
  productMetadata?: Map<string, ProductInfo>;
  comments?: TaskComment[];
  onFetchComments?: (id: number) => Promise<any>;
  onAddComment?: (id: number, body: string) => Promise<TaskComment>;
}

export default function TaskDetail({ task, onClose, onMove, onUpdate, onDelete, activities = [], streamContent = '', products = [], productMetadata, comments = [], onFetchComments, onAddComment }: TaskDetailProps) {
  const meta = parseMetadata(task.metadata);
  const [autonomous, setAutonomous] = useState(!!meta.autonomous);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState(task.title);
  const titleInputRef = useRef<HTMLInputElement>(null);
  const streamEndRef = useRef<HTMLDivElement>(null);
  const [commentText, setCommentText] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const commentsEndRef = useRef<HTMLDivElement>(null);
  const [activeTab, setActiveTab] = useState<DetailTab>('task');

  // Sync title draft when task prop changes (e.g., from WebSocket update).
  useEffect(() => {
    if (!editingTitle) setTitleDraft(task.title);
  }, [task.title, editingTitle]);

  // Focus title input when entering edit mode.
  useEffect(() => {
    if (editingTitle && titleInputRef.current) {
      titleInputRef.current.focus();
      titleInputRef.current.select();
    }
  }, [editingTitle]);

  const commitTitle = async () => {
    setEditingTitle(false);
    const trimmed = titleDraft.trim();
    if (trimmed && trimmed !== task.title) {
      await onUpdate(task.id, { title: trimmed });
    } else {
      setTitleDraft(task.title);
    }
  };

  // Sync autonomous state when task prop changes (e.g., from WebSocket update).
  useEffect(() => {
    const m = parseMetadata(task.metadata);
    setAutonomous(!!m.autonomous);
  }, [task.metadata]);

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

  // Auto-switch to implementation tab when streamContent becomes non-empty
  useEffect(() => {
    if (streamContent) {
      setActiveTab('implementation');
    }
  }, [streamContent]);

  // Escape key closes the modal
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !editingTitle) onClose();
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onClose, editingTitle]);

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

  const toggleAutonomous = async () => {
    const next = !autonomous;
    setAutonomous(next);
    const newMeta = { ...meta, autonomous: next };
    await onUpdate(task.id, { metadata: JSON.stringify(newMeta) });
  };

  const handleStageChange = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newStage = e.target.value as TaskStage;
    if (newStage !== task.stage) {
      await onMove(task.id, newStage, '');
    }
  };

  const handlePriorityChange = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newPriority = Number(e.target.value);
    if (newPriority !== task.priority) {
      await onUpdate(task.id, { priority: newPriority });
    }
  };

  const handleProductChange = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const val = e.target.value;
    if (val !== task.product) {
      await onUpdate(task.id, { product: val });
    }
  };

  const isProcessing = autonomous && streamContent.length > 0;
  const hasActivities = activities.length > 0;

  // Build product options: existing products list + current task product (if not in list)
  const productOptions = products.includes(task.product ?? '')
    ? products
    : task.product
    ? [task.product, ...products]
    : products;

  const tabs: { key: DetailTab; label: string }[] = [
    { key: 'task', label: 'Task' },
    { key: 'plan', label: 'Plan' },
    { key: 'implementation', label: 'Implementation' },
    { key: 'comments', label: comments.length > 0 ? `Comments (${comments.length})` : 'Comments' },
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-surface border border-border-default rounded-2xl shadow-2xl animate-fade-in-scale w-full max-w-2xl mx-4 max-h-[85vh] flex flex-col">
        {/* Header */}
        <div className="px-6 py-4 border-b border-border-subtle shrink-0">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-3 flex-1 min-w-0">
              <span className="text-fg-muted font-mono text-xs shrink-0">#{task.id}</span>
              {editingTitle ? (
                <input
                  ref={titleInputRef}
                  value={titleDraft}
                  onChange={(e) => setTitleDraft(e.target.value)}
                  onBlur={commitTitle}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') commitTitle();
                    if (e.key === 'Escape') { setEditingTitle(false); setTitleDraft(task.title); }
                  }}
                  className="flex-1 min-w-0 font-display text-lg font-semibold text-fg bg-transparent border-b border-soul/60 outline-none pb-0.5"
                />
              ) : (
                <h3
                  className="font-display text-lg font-semibold text-fg cursor-text hover:text-fg/80 transition-colors truncate"
                  onClick={() => setEditingTitle(true)}
                  title="Click to edit title"
                >
                  {task.title}
                </h3>
              )}
              <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium shrink-0 ${STAGE_COLORS[task.stage]}`}>
                {STAGE_LABELS[task.stage]}
              </span>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <button
                type="button"
                onClick={() => onDelete(task.id)}
                className="flex items-center justify-center w-8 h-8 rounded bg-red-600 hover:bg-red-700 text-white transition-colors cursor-pointer text-sm leading-none"
                aria-label="Delete task"
                title="Delete task"
              >
                &#x2421;
              </button>
              <button
                type="button"
                onClick={onClose}
                className="flex items-center justify-center w-8 h-8 rounded bg-elevated hover:bg-overlay text-fg-muted hover:text-fg transition-colors cursor-pointer text-lg leading-none"
                aria-label="Close"
              >
                &times;
              </button>
            </div>
          </div>

          {/* Processing banner */}
          {isProcessing && (
            <div className="flex items-center gap-2 mt-2 px-3 py-1.5 rounded-lg bg-soul/10 border border-soul/20">
              <span className="inline-block w-2 h-2 rounded-full bg-soul animate-pulse" />
              <span className="text-xs text-soul font-medium">Soul is working...</span>
            </div>
          )}
        </div>

        {/* Tab Bar */}
        <div className="flex border-b border-border-subtle shrink-0 px-6">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              type="button"
              onClick={() => setActiveTab(tab.key)}
              className={`px-4 py-2.5 text-sm font-medium transition-colors cursor-pointer ${
                activeTab === tab.key
                  ? 'text-soul border-b-2 border-soul -mb-px'
                  : 'text-fg-muted hover:text-fg-secondary'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Tab Content */}
        <div className="flex-1 overflow-y-auto px-6 py-4">

          {/* Task Tab */}
          {activeTab === 'task' && (
            <div className="space-y-5">
              {/* Description */}
              <div>
                <h4 className="font-display text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-1.5">Description</h4>
                {task.description ? (
                  <p className="text-fg-secondary text-sm whitespace-pre-wrap">{task.description}</p>
                ) : (
                  <p className="text-fg-muted text-sm italic">No description</p>
                )}
              </div>

              {/* Acceptance Criteria */}
              {task.acceptance && (
                <div>
                  <h4 className="font-display text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-1.5">Acceptance Criteria</h4>
                  <p className="text-fg-secondary text-sm whitespace-pre-wrap">{task.acceptance}</p>
                </div>
              )}

              {/* Properties */}
              <div>
                <h4 className="font-display text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-2">Properties</h4>
                <div className="grid grid-cols-2 gap-3">
                  {/* Stage */}
                  <div>
                    <label className="text-xs text-fg-muted mb-1 block">Stage</label>
                    <select
                      value={task.stage}
                      onChange={handleStageChange}
                      className="soul-select w-full bg-elevated border border-border-default rounded-lg px-3 py-1.5 text-sm text-fg cursor-pointer outline-none"
                    >
                      {ALL_STAGES.map((s) => (
                        <option key={s} value={s} className="bg-surface text-fg">
                          {STAGE_LABELS[s]}
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* Priority */}
                  <div>
                    <label className="text-xs text-fg-muted mb-1 block">Priority</label>
                    <select
                      value={task.priority}
                      onChange={handlePriorityChange}
                      className="soul-select w-full bg-elevated border border-border-default rounded-lg px-3 py-1.5 text-sm text-fg cursor-pointer outline-none"
                    >
                      {PRIORITY_OPTIONS.map((opt) => (
                        <option key={opt.value} value={opt.value} className="bg-surface text-fg">
                          {opt.label}
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* Product */}
                  <div>
                    <label className="text-xs text-fg-muted mb-1 block">Product</label>
                    <select
                      value={task.product ?? ''}
                      onChange={handleProductChange}
                      className="soul-select w-full bg-elevated border border-border-default rounded-lg px-3 py-1.5 text-sm text-fg cursor-pointer outline-none"
                    >
                      <option value="" className="bg-surface text-fg">No product</option>
                      {productOptions.map((p) => (
                        <option key={p} value={p} className="bg-surface text-fg">
                          {productMetadata?.get(p)?.label ?? (p.charAt(0).toUpperCase() + p.slice(1))}
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* Autonomous */}
                  <div>
                    <label className="text-xs text-fg-muted mb-1 block">Autonomous</label>
                    <div className="flex items-center gap-2 h-[34px]">
                      <button
                        type="button"
                        onClick={toggleAutonomous}
                        className={`relative w-10 h-5 rounded-full transition-colors cursor-pointer shrink-0 ${autonomous ? 'bg-soul' : 'bg-elevated border border-border-default'}`}
                      >
                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full transition-all duration-200 ${autonomous ? 'translate-x-5 bg-deep' : 'translate-x-0 bg-fg-muted'}`} />
                      </button>
                      {autonomous && (
                        <div className="flex items-center gap-1.5">
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
                  </div>
                </div>
              </div>

              {/* Error callout */}
              {task.error && (
                <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3">
                  <h4 className="font-display text-[10px] font-semibold uppercase tracking-widest text-red-400 mb-1">Error</h4>
                  <p className="text-sm text-red-300 font-mono whitespace-pre-wrap">{task.error}</p>
                </div>
              )}

              {/* Blocker callout */}
              {task.blocker && (
                <div className="bg-amber-500/10 border border-amber-500/20 rounded-lg p-3">
                  <h4 className="font-display text-[10px] font-semibold uppercase tracking-widest text-amber-400 mb-1">Blocker</h4>
                  <p className="text-sm text-amber-300 whitespace-pre-wrap">{task.blocker}</p>
                </div>
              )}
            </div>
          )}

          {/* Plan Tab */}
          {activeTab === 'plan' && (
            <div>
              {task.plan ? (
                <div className="prose prose-invert prose-sm prose-soul max-w-none">
                  <Markdown remarkPlugins={[remarkGfm]}>{task.plan}</Markdown>
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center py-16 text-fg-muted">
                  <span className="text-3xl mb-3">{'\u25C7'}</span>
                  <span className="text-sm">No plan generated yet</span>
                </div>
              )}
            </div>
          )}

          {/* Implementation Tab */}
          {activeTab === 'implementation' && (
            <div className="space-y-4">
              {/* Validation review link */}
              {task.stage === 'validation' && task.agent_id?.startsWith('auto-') && (
                <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-soul/10 border border-soul/20 text-sm">
                  <span className="text-fg-secondary">Changes are live on the dev server:</span>
                  <a
                    href="http://localhost:3001"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-soul hover:underline font-mono text-xs"
                  >
                    localhost:3001
                  </a>
                  <span className="text-[10px] text-fg-muted ml-auto">Move to Done to merge to production</span>
                </div>
              )}

              {/* Live stream */}
              {streamContent ? (
                <div className="bg-deep/60 rounded-lg p-4 border border-soul/20">
                  <div className="prose prose-invert prose-sm prose-soul max-w-none">
                    <Markdown remarkPlugins={[remarkGfm]}>{streamContent}</Markdown>
                  </div>
                  <div ref={streamEndRef} />
                </div>
              ) : task.output ? (
                /* Final output */
                <div className="prose prose-invert prose-sm prose-soul max-w-none">
                  <Markdown remarkPlugins={[remarkGfm]}>{task.output}</Markdown>
                </div>
              ) : (
                /* Empty state */
                <div className="flex flex-col items-center justify-center py-16 text-fg-muted">
                  <span className="text-3xl mb-3">{'\u2699'}</span>
                  <span className="text-sm">Not started yet</span>
                </div>
              )}

              {/* Activity log */}
              {hasActivities && (
                <div>
                  <h4 className="font-display text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-2">Activity</h4>
                  <div className="space-y-1">
                    {activities.map((a, i) => (
                      <ActivityEntry key={i} activity={a} />
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Comments Tab */}
          {activeTab === 'comments' && (
            <div className="flex flex-col h-full">
              {/* Scrollable thread */}
              <div className="flex-1 overflow-y-auto space-y-3 mb-3">
                {(comments || []).length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-16 text-fg-muted">
                    <span className="text-3xl mb-3">{'\u{1F4AC}'}</span>
                    <span className="text-sm">No comments yet</span>
                  </div>
                ) : (
                  <>
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
                                : 'bg-overlay border border-border-subtle'
                        }`}
                      >
                        <div className="flex items-center gap-2 mb-1">
                          <span className={`text-xs font-medium ${
                            c.author === 'user' ? 'text-blue-400' : 'text-fg-muted'
                          }`}>
                            {c.author === 'user' ? 'You' : 'Soul'}
                          </span>
                          <span className="text-xs text-fg-muted">
                            {new Date(c.created_at).toLocaleTimeString()}
                          </span>
                          {c.type !== 'feedback' && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-elevated text-fg-muted">
                              {c.type}
                            </span>
                          )}
                        </div>
                        <p className="text-fg-secondary whitespace-pre-wrap">{c.body}</p>
                        {c.attachments && c.attachments.length > 0 && (
                          <div className="mt-2 flex flex-col gap-2">
                            {c.attachments.map((filename, idx) => (
                              <a
                                key={idx}
                                href={`/api/screenshots/${filename}`}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="block"
                              >
                                <img
                                  src={`/api/screenshots/${filename}`}
                                  alt={`Screenshot: ${filename}`}
                                  className="rounded-lg border border-border-default max-w-full max-h-64 object-contain hover:border-soul/50 transition-colors cursor-pointer"
                                  loading="lazy"
                                />
                                <span className="text-[10px] text-fg-muted mt-0.5 block">{filename}</span>
                              </a>
                            ))}
                          </div>
                        )}
                      </div>
                    ))}
                    <div ref={commentsEndRef} />
                  </>
                )}
              </div>

              {/* Input bar */}
              <div className="pt-3 border-t border-border-subtle">
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
                    className="flex-1 bg-elevated border border-border-default rounded-lg px-3 py-2 text-sm text-fg placeholder-fg-muted focus:outline-none focus:border-soul/50"
                    disabled={submitting}
                  />
                  <button
                    onClick={handleSubmitComment}
                    disabled={submitting || !commentText.trim()}
                    className="px-3 py-2 rounded-lg bg-soul hover:bg-soul/80 text-deep text-sm font-medium disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
                  >
                    {submitting ? '...' : 'Send'}
                  </button>
                </div>
              </div>
            </div>
          )}

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
