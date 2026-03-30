// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import TaskRail from './TaskRail';

vi.mock('../planner/NewTaskForm.tsx', () => ({
  default: ({ onClose, onCreate }: { onClose: () => void; onCreate: (t: string, d: string, p: number, pr: string) => void }) => (
    <div data-testid="mock-new-task-form">
      <button data-testid="mock-close" onClick={onClose}>Close</button>
      <button data-testid="mock-create" onClick={() => onCreate('title', 'desc', 1, 'chat')}>Create</button>
    </div>
  ),
}));

function makeTasksByStage(overrides: Record<string, unknown[]> = {}) {
  return {
    backlog: [],
    brainstorm: [],
    active: [],
    blocked: [],
    validation: [],
    done: [],
    ...overrides,
  } as any;
}

function makeTask(id: number) {
  return { id, title: `Task ${id}`, stage: 'active' } as any;
}

describe('TaskRail', () => {
  afterEach(() => cleanup());

  it('renders task rail container', () => {
    render(<TaskRail tasksByStage={makeTasksByStage()} onExpand={vi.fn()} onNewTask={vi.fn()} />);
    expect(screen.getByTestId('task-rail')).toBeTruthy();
  });

  it('renders expand button', () => {
    render(<TaskRail tasksByStage={makeTasksByStage()} onExpand={vi.fn()} onNewTask={vi.fn()} />);
    expect(screen.getByTestId('task-rail-expand-btn')).toBeTruthy();
  });

  it('calls onExpand when expand clicked', () => {
    const onExpand = vi.fn();
    render(<TaskRail tasksByStage={makeTasksByStage()} onExpand={onExpand} onNewTask={vi.fn()} />);
    fireEvent.click(screen.getByTestId('task-rail-expand-btn'));
    expect(onExpand).toHaveBeenCalled();
  });

  it('renders new task button', () => {
    render(<TaskRail tasksByStage={makeTasksByStage()} onExpand={vi.fn()} onNewTask={vi.fn()} />);
    expect(screen.getByTestId('task-rail-new-btn')).toBeTruthy();
  });

  it('shows NewTaskForm when new button clicked', () => {
    render(<TaskRail tasksByStage={makeTasksByStage()} onExpand={vi.fn()} onNewTask={vi.fn()} />);
    fireEvent.click(screen.getByTestId('task-rail-new-btn'));
    expect(screen.getByTestId('mock-new-task-form')).toBeTruthy();
  });

  it('renders all 6 stage boxes', () => {
    render(<TaskRail tasksByStage={makeTasksByStage()} onExpand={vi.fn()} onNewTask={vi.fn()} />);
    expect(screen.getByTestId('task-rail-stage-backlog')).toBeTruthy();
    expect(screen.getByTestId('task-rail-stage-brainstorm')).toBeTruthy();
    expect(screen.getByTestId('task-rail-stage-active')).toBeTruthy();
    expect(screen.getByTestId('task-rail-stage-blocked')).toBeTruthy();
    expect(screen.getByTestId('task-rail-stage-validation')).toBeTruthy();
    expect(screen.getByTestId('task-rail-stage-done')).toBeTruthy();
  });

  it('shows 0 for empty stages', () => {
    render(<TaskRail tasksByStage={makeTasksByStage()} onExpand={vi.fn()} onNewTask={vi.fn()} />);
    const box = screen.getByTestId('task-rail-stage-active');
    expect(box.textContent).toBe('0');
  });

  it('shows correct count for populated stages', () => {
    const tasks = makeTasksByStage({
      active: [makeTask(1), makeTask(2), makeTask(3)],
      done: [makeTask(4)],
    });
    render(<TaskRail tasksByStage={tasks} onExpand={vi.fn()} onNewTask={vi.fn()} />);
    expect(screen.getByTestId('task-rail-stage-active').textContent).toBe('3');
    expect(screen.getByTestId('task-rail-stage-done').textContent).toBe('1');
  });

  it('shows stage tooltip with name and count', () => {
    const tasks = makeTasksByStage({ blocked: [makeTask(1), makeTask(2)] });
    render(<TaskRail tasksByStage={tasks} onExpand={vi.fn()} onNewTask={vi.fn()} />);
    expect(screen.getByTestId('task-rail-stage-blocked').title).toBe('blocked: 2');
  });
});
