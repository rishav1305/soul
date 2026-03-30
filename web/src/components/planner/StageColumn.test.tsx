// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/react';
import StageColumn from './StageColumn';
import type { PlannerTask, TaskStage } from '../../lib/types';

// Mock TaskCard to isolate StageColumn
vi.mock('./TaskCard.tsx', () => ({
  default: ({ task, onClick, selected, selectable }: any) => (
    <div data-testid={`mock-task-${task.id}`} data-selected={selected} data-selectable={selectable}>
      {task.title}
      {onClick && <button data-testid={`click-task-${task.id}`} onClick={onClick}>Click</button>}
    </div>
  ),
}));

function makeTask(overrides: Partial<PlannerTask> = {}): PlannerTask {
  return {
    id: 1,
    title: 'Test Task',
    description: 'A task',
    stage: 'active',
    product: 'chat',
    substep: '',
    metadata: '',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

describe('StageColumn', () => {
  afterEach(() => cleanup());

  it('renders stage column with testid', () => {
    render(<StageColumn stage="active" tasks={[]} />);
    expect(screen.getByTestId('stage-column-active')).toBeTruthy();
  });

  it('shows stage label', () => {
    render(<StageColumn stage="backlog" tasks={[]} />);
    expect(screen.getByText('Backlog')).toBeTruthy();
  });

  it('shows task count', () => {
    const tasks = [makeTask({ id: 1 }), makeTask({ id: 2 }), makeTask({ id: 3 })];
    render(<StageColumn stage="active" tasks={tasks} />);
    expect(screen.getByText('3')).toBeTruthy();
  });

  it('renders task cards', () => {
    const tasks = [makeTask({ id: 10, title: 'Task A' }), makeTask({ id: 20, title: 'Task B' })];
    render(<StageColumn stage="active" tasks={tasks} />);
    expect(screen.getByTestId('mock-task-10')).toBeTruthy();
    expect(screen.getByTestId('mock-task-20')).toBeTruthy();
  });

  it('shows "No tasks" when empty', () => {
    render(<StageColumn stage="done" tasks={[]} />);
    expect(screen.getByText('No tasks')).toBeTruthy();
  });

  it('renders all stage types correctly', () => {
    const stages: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];
    for (const stage of stages) {
      const { unmount } = render(<StageColumn stage={stage} tasks={[]} />);
      expect(screen.getByTestId(`stage-column-${stage}`)).toBeTruthy();
      unmount();
    }
  });

  it('shows zero count when no tasks', () => {
    render(<StageColumn stage="backlog" tasks={[]} />);
    expect(screen.getByText('0')).toBeTruthy();
  });

  it('passes onTaskClick callback to TaskCard', () => {
    const onTaskClick = vi.fn();
    const tasks = [makeTask({ id: 5, title: 'Clickable' })];
    render(<StageColumn stage="active" tasks={tasks} onTaskClick={onTaskClick} />);
    fireEvent.click(screen.getByTestId('click-task-5'));
    expect(onTaskClick).toHaveBeenCalledWith(tasks[0]);
  });

  it('passes onTaskSelect as onClick to TaskCard', () => {
    const onTaskSelect = vi.fn();
    const tasks = [makeTask({ id: 6 })];
    render(<StageColumn stage="active" tasks={tasks} onTaskSelect={onTaskSelect} />);
    fireEvent.click(screen.getByTestId('click-task-6'));
    expect(onTaskSelect).toHaveBeenCalledWith(6);
  });

  it('marks selected tasks via selectedIds', () => {
    const tasks = [makeTask({ id: 1 }), makeTask({ id: 2 })];
    render(
      <StageColumn
        stage="active"
        tasks={tasks}
        onTaskSelect={vi.fn()}
        selectedIds={new Set([1])}
      />,
    );
    expect(screen.getByTestId('mock-task-1').getAttribute('data-selected')).toBe('true');
    expect(screen.getByTestId('mock-task-2').getAttribute('data-selected')).toBe('false');
  });

  it('does not render click button when no onTaskClick or onTaskSelect', () => {
    render(<StageColumn stage="active" tasks={[makeTask({ id: 7 })]} />);
    expect(screen.queryByTestId('click-task-7')).toBeNull();
  });
});
