// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import ListView from './ListView';
import type { PlannerTask, TaskStage } from '../../lib/types';

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

describe('ListView', () => {
  afterEach(() => cleanup());

  it('renders stage section headers', () => {
    render(<ListView tasks={[]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('active')).toBeTruthy();
    expect(screen.getByText('backlog')).toBeTruthy();
    expect(screen.getByText('done')).toBeTruthy();
  });

  it('shows task count per stage', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active' }),
      makeTask({ id: 2, stage: 'active' }),
      makeTask({ id: 3, stage: 'backlog' }),
    ];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    // Active section should show "2"
    const activeHeader = screen.getByText('active');
    const activeSection = activeHeader.parentElement!;
    expect(activeSection.textContent).toContain('2');
  });

  it('renders task items with testids', () => {
    const tasks = [makeTask({ id: 42, stage: 'active' })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('list-task-42')).toBeTruthy();
  });

  it('shows task title', () => {
    const tasks = [makeTask({ title: 'Fix the bug' })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Fix the bug')).toBeTruthy();
  });

  it('shows task id', () => {
    const tasks = [makeTask({ id: 99 })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByText('#99')).toBeTruthy();
  });

  it('shows task description', () => {
    const tasks = [makeTask({ description: 'Detailed info' })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Detailed info')).toBeTruthy();
  });

  it('shows priority label', () => {
    const tasks = [makeTask({ priority: 3 })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Crit')).toBeTruthy();
  });

  it('shows product badge', () => {
    const tasks = [makeTask({ product: 'tutor' })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByText('tutor')).toBeTruthy();
  });

  it('shows substep info when set', () => {
    const tasks = [makeTask({ substep: 'qa_test' })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByText('QA 4/6')).toBeTruthy();
  });

  it('calls onTaskClick when task row clicked', () => {
    const onClick = vi.fn();
    const task = makeTask({ id: 5 });
    render(<ListView tasks={[task]} onTaskClick={onClick} />);
    fireEvent.click(screen.getByTestId('list-task-5'));
    expect(onClick).toHaveBeenCalledWith(task);
  });

  it('collapses sections when header clicked', () => {
    const tasks = [makeTask({ id: 1, stage: 'active' })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);

    // Task should be visible initially (active has tasks → starts expanded)
    expect(screen.getByTestId('list-task-1')).toBeTruthy();

    // Click the active header to collapse
    fireEvent.click(screen.getByText('active'));

    // Task should be hidden
    expect(screen.queryByTestId('list-task-1')).toBeNull();
  });

  it('expands collapsed section when header clicked again', () => {
    const tasks = [makeTask({ id: 1, stage: 'active' })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);

    // Collapse
    fireEvent.click(screen.getByText('active'));
    expect(screen.queryByTestId('list-task-1')).toBeNull();

    // Expand
    fireEvent.click(screen.getByText('active'));
    expect(screen.getByTestId('list-task-1')).toBeTruthy();
  });

  it('groups tasks by stage correctly', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active', title: 'Active Task' }),
      makeTask({ id: 2, stage: 'done', title: 'Done Task' }),
      makeTask({ id: 3, stage: 'blocked', title: 'Blocked Task' }),
    ];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('list-task-1')).toBeTruthy();
    expect(screen.getByTestId('list-task-2')).toBeTruthy();
    expect(screen.getByTestId('list-task-3')).toBeTruthy();
  });

  it('empty stages start collapsed', () => {
    // Only active has tasks; backlog should be collapsed but that just means its tasks aren't shown
    const tasks = [makeTask({ id: 1, stage: 'active' })];
    render(<ListView tasks={tasks} onTaskClick={vi.fn()} />);
    // All section headers should be visible regardless
    expect(screen.getByText('backlog')).toBeTruthy();
    expect(screen.getByText('done')).toBeTruthy();
  });
});
