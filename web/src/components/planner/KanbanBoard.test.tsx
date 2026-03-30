// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import KanbanBoard from './KanbanBoard';
import type { PlannerTask, TaskStage } from '../../lib/types';

// Mock StageColumn to isolate KanbanBoard logic
vi.mock('./StageColumn', () => ({
  default: ({ stage, tasks, onTaskClick, onTaskSelect, selectedIds }: {
    stage: string;
    tasks: PlannerTask[];
    onTaskClick?: (t: PlannerTask) => void;
    onTaskSelect?: (id: number) => void;
    selectedIds?: Set<number>;
  }) => (
    <div data-testid={`mock-stage-${stage}`}>
      <span>{stage} ({tasks.length})</span>
      {tasks.map((t) => (
        <button
          key={t.id}
          data-testid={`mock-task-${t.id}`}
          onClick={() => onTaskClick?.(t) || onTaskSelect?.(t.id)}
          data-selected={selectedIds?.has(t.id) ?? false}
        >
          {t.title}
        </button>
      ))}
    </div>
  ),
}));

// Mock authFetch for batch operations
vi.mock('../../lib/api.ts', () => ({
  authFetch: vi.fn().mockResolvedValue({ ok: true }),
}));

function makeTask(overrides: Partial<PlannerTask> = {}): PlannerTask {
  return {
    id: 1,
    title: 'Test Task',
    description: 'Task description',
    stage: 'active',
    product: 'chat',
    substep: '',
    metadata: '',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

function makeTasksByStage(tasks: PlannerTask[] = []): Record<TaskStage, PlannerTask[]> {
  const stages: Record<TaskStage, PlannerTask[]> = {
    active: [], backlog: [], brainstorm: [], blocked: [], validation: [], done: [],
  };
  for (const t of tasks) {
    stages[t.stage].push(t);
  }
  return stages;
}

describe('KanbanBoard', () => {
  afterEach(() => cleanup());

  it('renders stage columns for populated stages', () => {
    const tasks = [makeTask({ id: 1, stage: 'active' }), makeTask({ id: 2, stage: 'backlog' })];
    render(<KanbanBoard tasksByStage={makeTasksByStage(tasks)} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('mock-stage-active')).toBeTruthy();
    expect(screen.getByTestId('mock-stage-backlog')).toBeTruthy();
  });

  it('shows all stages when no tasks exist', () => {
    render(<KanbanBoard tasksByStage={makeTasksByStage()} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('mock-stage-active')).toBeTruthy();
    expect(screen.getByTestId('mock-stage-backlog')).toBeTruthy();
    expect(screen.getByTestId('mock-stage-brainstorm')).toBeTruthy();
    expect(screen.getByTestId('mock-stage-blocked')).toBeTruthy();
    expect(screen.getByTestId('mock-stage-validation')).toBeTruthy();
    expect(screen.getByTestId('mock-stage-done')).toBeTruthy();
  });

  it('hides empty stages when some have tasks', () => {
    const tasks = [makeTask({ id: 1, stage: 'active' })];
    render(<KanbanBoard tasksByStage={makeTasksByStage(tasks)} onTaskClick={vi.fn()} />);
    // Only active should be visible since it is the only populated stage
    expect(screen.getByTestId('mock-stage-active')).toBeTruthy();
    expect(screen.queryByTestId('mock-stage-done')).toBeNull();
  });

  it('shows Select button to enter batch mode', () => {
    render(<KanbanBoard tasksByStage={makeTasksByStage()} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('kanban-batch-mode')).toBeTruthy();
    expect(screen.getByText('Select')).toBeTruthy();
  });

  it('enters batch mode when Select clicked', () => {
    render(<KanbanBoard tasksByStage={makeTasksByStage()} onTaskClick={vi.fn()} />);
    fireEvent.click(screen.getByTestId('kanban-batch-mode'));
    // Should show "0 selected" and batch controls
    expect(screen.getByText('0 selected')).toBeTruthy();
    expect(screen.getByTestId('kanban-exit-batch')).toBeTruthy();
    expect(screen.getByTestId('kanban-select-all')).toBeTruthy();
  });

  it('exits batch mode when Cancel clicked', () => {
    render(<KanbanBoard tasksByStage={makeTasksByStage()} onTaskClick={vi.fn()} />);
    fireEvent.click(screen.getByTestId('kanban-batch-mode'));
    expect(screen.getByText('0 selected')).toBeTruthy();
    fireEvent.click(screen.getByTestId('kanban-exit-batch'));
    // Should return to normal mode
    expect(screen.getByTestId('kanban-batch-mode')).toBeTruthy();
    expect(screen.queryByText('0 selected')).toBeNull();
  });

  it('shows correct task count in stage columns', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active' }),
      makeTask({ id: 2, stage: 'active' }),
    ];
    render(<KanbanBoard tasksByStage={makeTasksByStage(tasks)} onTaskClick={vi.fn()} />);
    expect(screen.getByText('active (2)')).toBeTruthy();
  });

  it('shows Move to, Priority, and Delete options in batch mode', () => {
    render(<KanbanBoard tasksByStage={makeTasksByStage()} onTaskClick={vi.fn()} />);
    fireEvent.click(screen.getByTestId('kanban-batch-mode'));
    expect(screen.getByText('Move to...')).toBeTruthy();
    expect(screen.getByText('Priority...')).toBeTruthy();
    expect(screen.getByText('Delete')).toBeTruthy();
  });

  it('shows product dropdown in batch mode when products provided', () => {
    render(<KanbanBoard tasksByStage={makeTasksByStage()} onTaskClick={vi.fn()} products={['chat', 'tasks']} />);
    fireEvent.click(screen.getByTestId('kanban-batch-mode'));
    expect(screen.getByText('Product...')).toBeTruthy();
  });

  it('does not show product dropdown when no products provided', () => {
    render(<KanbanBoard tasksByStage={makeTasksByStage()} onTaskClick={vi.fn()} />);
    fireEvent.click(screen.getByTestId('kanban-batch-mode'));
    expect(screen.queryByText('Product...')).toBeNull();
  });

  it('select all button shows total task count', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active' }),
      makeTask({ id: 2, stage: 'backlog' }),
      makeTask({ id: 3, stage: 'done' }),
    ];
    render(<KanbanBoard tasksByStage={makeTasksByStage(tasks)} onTaskClick={vi.fn()} />);
    fireEvent.click(screen.getByTestId('kanban-batch-mode'));
    expect(screen.getByText('Select all (3)')).toBeTruthy();
  });
});
