import { useState, useRef, useEffect } from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { PlannerTask, TaskStage, TaskActivity } from '../../lib/types.ts';

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

interface TaskDetailProps {
  task: PlannerTask;
  onClose: () => void;
  onMove: (id: number, stage: TaskStage, comment: string) => Promise<void>;
  onUpdate: (id: number, updates: Partial<PlannerTask>) => Promise<PlannerTask>;
  onDelete: (id: number) => Promise<void>;
  activities?: TaskActivity[];
  streamContent?: string;
  products?: string[];
}

export default function TaskDetail({ task, onClose, onMove, onUpdate, onDelete, activities = [], streamContent = '', products = [] }: TaskDetailProps) {
  const meta = parseMetadata(task.metadata);
  const [autonomous, setAutonomous] = useState(!!meta.autonomous);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState(task.title);
  const titleInputRef = useRef<HTMLInputElement>(null);
  const streamEndRef = useRef<HTMLDivElement>(null);

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

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-surface border border-border-default rounded-2xl shadow-2xl animate-fade-in-scale w-full max-w-2xl mx-4 max-h-[85vh] flex flex-col">
        {/* Header */}
        <div className="px-6 py-4 border-b border-border-subtle shrink-0">
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1 min-w-0">
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
                  className="w-full font-display text-lg font-semibold text-fg bg-transparent border-b border-soul/60 outline-none pb-0.5"
                />
              ) : (
                <h3
                  className="font-display text-lg font-semibold text-fg cursor-text hover:text-fg/80 transition-colors"
                  onClick={() => setEditingTitle(true)}
                  title="Click to edit title"
                >
                  {task.title}
                </h3>
              )}
              <div className="flex items-center gap-2 mt-1 flex-wrap">
                <span className="text-fg-muted font-mono text-xs">#{task.id}</span>
                {/* Stage dropdown */}
                <select
                  value={task.stage}
                  onChange={handleStageChange}
                  className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium cursor-pointer border-0 outline-none appearance-none ${STAGE_COLORS[task.stage]} bg-transparent`}
                  style={{ backgroundImage: 'none' }}
                >
                  {ALL_STAGES.map((s) => (
                    <option key={s} value={s} className="bg-surface text-fg">
                      {STAGE_LABELS[s]}
                    </option>
                  ))}
                </select>
                {/* Priority dropdown */}
                <select
                  value={task.priority}
                  onChange={handlePriorityChange}
                  className={`text-xs font-medium cursor-pointer border-0 outline-none appearance-none bg-transparent ${PRIORITY_COLORS[task.priority] ?? 'text-priority-low'}`}
                  style={{ backgroundImage: 'none' }}
                >
                  {PRIORITY_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value} className="bg-surface text-fg">
                      {opt.label} priority
                    </option>
                  ))}
                </select>
                {/* Product dropdown */}
                <select
                  value={task.product ?? ''}
                  onChange={handleProductChange}
                  className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium cursor-pointer border-0 outline-none appearance-none ${task.product ? 'bg-overlay text-fg-secondary' : 'text-fg-muted'} bg-transparent`}
                  style={{ backgroundImage: 'none' }}
                >
                  <option value="" className="bg-surface text-fg">No product</option>
                  {productOptions.map((p) => (
                    <option key={p} value={p} className="bg-surface text-fg">
                      {p}
                    </option>
                  ))}
                </select>
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
            <div className="flex items-center gap-3 shrink-0">
              <button
                type="button"
                onClick={() => onDelete(task.id)}
                className="flex items-center justify-center w-9 h-9 rounded bg-red-600 hover:bg-red-700 text-white transition-colors cursor-pointer text-base leading-none"
                aria-label="Delete task"
                title="Delete task"
              >
                ␡
              </button>
              <button
                type="button"
                onClick={onClose}
                className="flex items-center justify-center w-9 h-9 rounded bg-elevated hover:bg-overlay text-fg-muted hover:text-fg transition-colors cursor-pointer text-lg leading-none"
                aria-label="Close"
              >
                &times;
              </button>
            </div>
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
