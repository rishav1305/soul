// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import GroupedList from './GroupedList';
import type { PlannerTask } from '../../../lib/types';

function makeTask(overrides: Partial<PlannerTask> = {}): PlannerTask {
  return {
    id: 1,
    title: 'Test Task',
    description: 'Task description here',
    stage: 'active',
    product: 'chat',
    substep: '',
    metadata: '',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

describe('GroupedList', () => {
  afterEach(() => cleanup());

  it('renders stage section headers', () => {
    render(<GroupedList tasks={[]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('active')).toBeTruthy();
    expect(screen.getByText('backlog')).toBeTruthy();
    expect(screen.getByText('brainstorm')).toBeTruthy();
    expect(screen.getByText('blocked')).toBeTruthy();
    expect(screen.getByText('validation')).toBeTruthy();
    expect(screen.getByText('done')).toBeTruthy();
  });

  it('shows task count per stage', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active' }),
      makeTask({ id: 2, stage: 'active' }),
      makeTask({ id: 3, stage: 'backlog' }),
    ];
    render(<GroupedList tasks={tasks} onTaskClick={vi.fn()} />);
    const toggle = screen.getByTestId('grouped-stage-toggle-active');
    expect(toggle.textContent).toContain('2');
  });

  it('renders task items with testids', () => {
    const tasks = [makeTask({ id: 42, stage: 'active' })];
    render(<GroupedList tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('grouped-task-42')).toBeTruthy();
  });

  it('shows task id', () => {
    render(<GroupedList tasks={[makeTask({ id: 99 })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('#99')).toBeTruthy();
  });

  it('shows task title', () => {
    render(<GroupedList tasks={[makeTask({ title: 'Fix the bug' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Fix the bug')).toBeTruthy();
  });

  it('shows priority label', () => {
    render(<GroupedList tasks={[makeTask({ priority: 3 })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Crit')).toBeTruthy();
  });

  it('shows product badge', () => {
    render(<GroupedList tasks={[makeTask({ product: 'tutor' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('tutor')).toBeTruthy();
  });

  it('shows substep info when set', () => {
    render(<GroupedList tasks={[makeTask({ substep: 'e2e_test' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('E2E 5/6')).toBeTruthy();
  });

  it('truncates long descriptions', () => {
    const longDesc = 'x'.repeat(100);
    render(<GroupedList tasks={[makeTask({ description: longDesc })]} onTaskClick={vi.fn()} />);
    const task = screen.getByTestId('grouped-task-1');
    expect(task.textContent).toContain('\u2026');
  });

  it('shows relative time', () => {
    render(<GroupedList tasks={[makeTask()]} onTaskClick={vi.fn()} />);
    const task = screen.getByTestId('grouped-task-1');
    expect(task.textContent).toContain('just now');
  });

  it('calls onTaskClick when task clicked', () => {
    const onClick = vi.fn();
    const task = makeTask({ id: 5 });
    render(<GroupedList tasks={[task]} onTaskClick={onClick} />);
    fireEvent.click(screen.getByTestId('grouped-task-5'));
    expect(onClick).toHaveBeenCalledWith(task);
  });

  it('collapses section when header clicked', () => {
    const tasks = [makeTask({ id: 1, stage: 'active' })];
    render(<GroupedList tasks={tasks} onTaskClick={vi.fn()} />);
    // Task visible initially
    expect(screen.getByTestId('grouped-task-1')).toBeTruthy();
    // Collapse
    fireEvent.click(screen.getByTestId('grouped-stage-toggle-active'));
    expect(screen.queryByTestId('grouped-task-1')).toBeNull();
  });

  it('expands collapsed section when header clicked again', () => {
    const tasks = [makeTask({ id: 1, stage: 'active' })];
    render(<GroupedList tasks={tasks} onTaskClick={vi.fn()} />);
    // Collapse
    fireEvent.click(screen.getByTestId('grouped-stage-toggle-active'));
    expect(screen.queryByTestId('grouped-task-1')).toBeNull();
    // Expand
    fireEvent.click(screen.getByTestId('grouped-stage-toggle-active'));
    expect(screen.getByTestId('grouped-task-1')).toBeTruthy();
  });

  it('empty stages start collapsed', () => {
    // Only active has tasks → backlog, brainstorm, etc. start collapsed
    const tasks = [makeTask({ id: 1, stage: 'active' })];
    render(<GroupedList tasks={tasks} onTaskClick={vi.fn()} />);
    // All headers still visible
    expect(screen.getByText('backlog')).toBeTruthy();
    // Collapse indicator should show right-arrow for collapsed
    const backlogToggle = screen.getByTestId('grouped-stage-toggle-backlog');
    expect(backlogToggle.textContent).toContain('\u25B6');
  });

  it('non-empty stages start expanded', () => {
    const tasks = [makeTask({ id: 1, stage: 'active' })];
    render(<GroupedList tasks={tasks} onTaskClick={vi.fn()} />);
    // Active has tasks → should show down-arrow (expanded)
    const activeToggle = screen.getByTestId('grouped-stage-toggle-active');
    expect(activeToggle.textContent).toContain('\u25BC');
  });

  it('shows blocked badge when blocker set', () => {
    render(<GroupedList tasks={[makeTask({ blocker: 'Waiting on API' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Blocked')).toBeTruthy();
  });

  it('groups tasks by stage correctly', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active' }),
      makeTask({ id: 2, stage: 'done' }),
      makeTask({ id: 3, stage: 'backlog' }),
    ];
    render(<GroupedList tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('grouped-task-1')).toBeTruthy();
    expect(screen.getByTestId('grouped-task-2')).toBeTruthy();
    expect(screen.getByTestId('grouped-task-3')).toBeTruthy();
  });

  it('renders task items as button elements', () => {
    render(<GroupedList tasks={[makeTask()]} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('grouped-task-1').tagName).toBe('BUTTON');
  });
});
