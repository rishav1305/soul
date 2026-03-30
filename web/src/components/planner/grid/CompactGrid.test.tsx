// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import CompactGrid from './CompactGrid';
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

describe('CompactGrid', () => {
  afterEach(() => cleanup());

  it('renders task cards with testids', () => {
    const tasks = [makeTask({ id: 10 }), makeTask({ id: 20 })];
    render(<CompactGrid tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('grid-task-10')).toBeTruthy();
    expect(screen.getByTestId('grid-task-20')).toBeTruthy();
  });

  it('shows task title', () => {
    render(<CompactGrid tasks={[makeTask({ title: 'Build dashboard' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Build dashboard')).toBeTruthy();
  });

  it('shows task id', () => {
    render(<CompactGrid tasks={[makeTask({ id: 42 })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('#42')).toBeTruthy();
  });

  it('shows priority label', () => {
    render(<CompactGrid tasks={[makeTask({ priority: 3 })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Crit')).toBeTruthy();
  });

  it('shows stage label', () => {
    render(<CompactGrid tasks={[makeTask({ stage: 'validation' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('validation')).toBeTruthy();
  });

  it('shows product label', () => {
    render(<CompactGrid tasks={[makeTask({ product: 'tutor' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('tutor')).toBeTruthy();
  });

  it('truncates long descriptions', () => {
    const longDesc = 'x'.repeat(100);
    render(<CompactGrid tasks={[makeTask({ description: longDesc })]} onTaskClick={vi.fn()} />);
    const card = screen.getByTestId('grid-task-1');
    // Should contain ellipsis character
    expect(card.textContent).toContain('\u2026');
  });

  it('shows autonomous badge when metadata has autonomous flag', () => {
    render(<CompactGrid tasks={[makeTask({ metadata: '{"autonomous":true}' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Auto')).toBeTruthy();
  });

  it('calls onTaskClick when card clicked', () => {
    const onClick = vi.fn();
    const task = makeTask({ id: 5 });
    render(<CompactGrid tasks={[task]} onTaskClick={onClick} />);
    fireEvent.click(screen.getByTestId('grid-task-5'));
    expect(onClick).toHaveBeenCalledWith(task);
  });

  it('sorts by priority descending', () => {
    const tasks = [
      makeTask({ id: 1, priority: 0, title: 'Low' }),
      makeTask({ id: 2, priority: 3, title: 'Critical' }),
      makeTask({ id: 3, priority: 1, title: 'Normal' }),
    ];
    const { container } = render(<CompactGrid tasks={tasks} onTaskClick={vi.fn()} />);
    const buttons = container.querySelectorAll('[data-testid^="grid-task-"]');
    // First should be highest priority (id=2)
    expect(buttons[0].getAttribute('data-testid')).toBe('grid-task-2');
  });

  it('shows relative time', () => {
    render(<CompactGrid tasks={[makeTask()]} onTaskClick={vi.fn()} />);
    const card = screen.getByTestId('grid-task-1');
    expect(card.textContent).toContain('just now');
  });

  it('renders as button elements', () => {
    render(<CompactGrid tasks={[makeTask()]} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('grid-task-1').tagName).toBe('BUTTON');
  });
});
