import { useState } from 'react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
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
}

export default function TaskDetail({ task, onClose, onMove, onUpdate, onDelete }: TaskDetailProps) {
  const transitions = ValidTransitions[task.stage];
  const meta = parseMetadata(task.metadata);
  const [autonomous, setAutonomous] = useState(!!meta.autonomous);

  const toggleAutonomous = async () => {
    const next = !autonomous;
    setAutonomous(next);
    const newMeta = { ...meta, autonomous: next };
    await onUpdate(task.id, { metadata: JSON.stringify(newMeta) });
  };

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
                {task.product && (
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-overlay text-fg-secondary">
                    {task.product}
                  </span>
                )}
              </div>

              {/* Autonomous toggle */}
              <div className="flex items-center gap-2 mt-2">
                <button
                  type="button"
                  onClick={toggleAutonomous}
                  className={`relative w-9 h-5 rounded-full transition-colors cursor-pointer ${autonomous ? 'bg-soul' : 'bg-elevated border border-border-default'}`}
                >
                  <span className={`absolute top-0.5 w-4 h-4 rounded-full transition-transform ${autonomous ? 'translate-x-4.5 bg-deep' : 'translate-x-0.5 bg-fg-muted'}`} />
                </button>
                <span className="text-xs text-fg-secondary">Autonomous</span>
                {autonomous && (
                  <span className="text-[10px] text-soul font-medium">Soul will work on this task</span>
                )}
              </div>
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

          {task.output && (
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

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h4 className="font-display text-[10px] font-semibold uppercase tracking-widest text-fg-muted mb-1">{title}</h4>
      {children}
    </div>
  );
}
