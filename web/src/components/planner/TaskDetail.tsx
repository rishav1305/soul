import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { PlannerTask, TaskStage } from '../../lib/types.ts';
import { ValidTransitions } from './transitions.ts';

const STAGE_LABELS: Record<TaskStage, string> = {
  backlog: 'Backlog',
  brainstorm: 'Brainstorm',
  active: 'Active',
  blocked: 'Blocked',
  validation: 'Validation',
  done: 'Done',
};

const STAGE_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-700 text-zinc-300',
  brainstorm: 'bg-purple-900/50 text-purple-300',
  active: 'bg-sky-900/50 text-sky-300',
  blocked: 'bg-red-900/50 text-red-300',
  validation: 'bg-amber-900/50 text-amber-300',
  done: 'bg-green-900/50 text-green-300',
};

const TRANSITION_BUTTON_COLORS: Record<TaskStage, string> = {
  backlog: 'bg-zinc-700 hover:bg-zinc-600 text-zinc-200',
  brainstorm: 'bg-purple-700 hover:bg-purple-600 text-purple-100',
  active: 'bg-sky-700 hover:bg-sky-600 text-sky-100',
  blocked: 'bg-red-700 hover:bg-red-600 text-red-100',
  validation: 'bg-amber-700 hover:bg-amber-600 text-amber-100',
  done: 'bg-green-700 hover:bg-green-600 text-green-100',
};

const PRIORITY_LABELS: Record<number, string> = {
  0: 'Low',
  1: 'Normal',
  2: 'High',
  3: 'Critical',
};

const PRIORITY_COLORS: Record<number, string> = {
  0: 'text-zinc-400',
  1: 'text-sky-400',
  2: 'text-amber-400',
  3: 'text-red-400',
};

interface TaskDetailProps {
  task: PlannerTask;
  onClose: () => void;
  onMove: (id: number, stage: TaskStage, comment: string) => Promise<void>;
  onDelete: (id: number) => Promise<void>;
}

export default function TaskDetail({ task, onClose, onMove, onDelete }: TaskDetailProps) {
  const transitions = ValidTransitions[task.stage];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-zinc-900 border border-zinc-700 rounded-xl shadow-2xl w-full max-w-2xl mx-4 max-h-[85vh] flex flex-col">
        {/* Header */}
        <div className="px-6 py-4 border-b border-zinc-800 shrink-0">
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1 min-w-0">
              <h3 className="text-lg font-semibold text-zinc-100">{task.title}</h3>
              <div className="flex items-center gap-2 mt-1">
                <span className="text-xs text-zinc-500">#{task.id}</span>
                <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${STAGE_COLORS[task.stage]}`}>
                  {STAGE_LABELS[task.stage]}
                </span>
                <span className={`text-xs font-medium ${PRIORITY_COLORS[task.priority] ?? 'text-zinc-400'}`}>
                  {PRIORITY_LABELS[task.priority] ?? 'Unknown'} priority
                </span>
              </div>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="text-zinc-500 hover:text-zinc-300 transition-colors cursor-pointer text-xl leading-none"
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
              <p className="text-sm text-zinc-300 whitespace-pre-wrap">{task.description}</p>
            </Section>
          )}

          {task.acceptance && (
            <Section title="Acceptance Criteria">
              <p className="text-sm text-zinc-300 whitespace-pre-wrap">{task.acceptance}</p>
            </Section>
          )}

          {task.plan && (
            <Section title="Plan">
              <div className="prose prose-invert prose-sm max-w-none">
                <Markdown remarkPlugins={[remarkGfm]}>{task.plan}</Markdown>
              </div>
            </Section>
          )}

          {task.output && (
            <Section title="Output">
              <div className="prose prose-invert prose-sm max-w-none">
                <Markdown remarkPlugins={[remarkGfm]}>{task.output}</Markdown>
              </div>
            </Section>
          )}

          {task.error && (
            <Section title="Error">
              <p className="text-sm text-red-400 font-mono whitespace-pre-wrap">{task.error}</p>
            </Section>
          )}

          {task.blocker && (
            <Section title="Blocker">
              <p className="text-sm text-red-400 whitespace-pre-wrap">{task.blocker}</p>
            </Section>
          )}
        </div>

        {/* Actions */}
        <div className="px-6 py-4 border-t border-zinc-800 shrink-0">
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
              className="px-3 py-1.5 text-sm font-medium rounded-md bg-red-900/50 hover:bg-red-800/70 text-red-300 transition-colors cursor-pointer"
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
      <h4 className="text-xs font-semibold text-zinc-500 uppercase tracking-wide mb-1">{title}</h4>
      {children}
    </div>
  );
}
