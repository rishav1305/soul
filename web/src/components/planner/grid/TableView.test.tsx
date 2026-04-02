// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import TableView from './TableView';
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

describe('TableView', () => {
  afterEach(() => cleanup());

  it('renders column headers', () => {
    render(<TableView tasks={[]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('ID')).toBeTruthy();
    expect(screen.getByText('Title')).toBeTruthy();
    expect(screen.getByText('Stage')).toBeTruthy();
    expect(screen.getByText('Priority')).toBeTruthy();
    expect(screen.getByText('Product')).toBeTruthy();
    expect(screen.getByText('Substep')).toBeTruthy();
    expect(screen.getByText('Created')).toBeTruthy();
  });

  it('renders task rows with testids', () => {
    const tasks = [makeTask({ id: 10 }), makeTask({ id: 20 })];
    render(<TableView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByTestId('table-row-10')).toBeTruthy();
    expect(screen.getByTestId('table-row-20')).toBeTruthy();
  });

  it('shows task id', () => {
    render(<TableView tasks={[makeTask({ id: 42 })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('#42')).toBeTruthy();
  });

  it('shows task title', () => {
    render(<TableView tasks={[makeTask({ title: 'Build API' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Build API')).toBeTruthy();
  });

  it('shows task description', () => {
    render(<TableView tasks={[makeTask({ description: 'Detailed info' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Detailed info')).toBeTruthy();
  });

  it('shows stage name', () => {
    render(<TableView tasks={[makeTask({ stage: 'validation' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('validation')).toBeTruthy();
  });

  it('shows priority labels', () => {
    const tasks = [
      makeTask({ id: 1, priority: 0 }),
      makeTask({ id: 2, priority: 1 }),
      makeTask({ id: 3, priority: 2 }),
      makeTask({ id: 4, priority: 3 }),
    ];
    render(<TableView tasks={tasks} onTaskClick={vi.fn()} />);
    expect(screen.getByText('Low')).toBeTruthy();
    expect(screen.getByText('Norm')).toBeTruthy();
    expect(screen.getByText('High')).toBeTruthy();
    expect(screen.getByText('Crit')).toBeTruthy();
  });

  it('shows product name', () => {
    render(<TableView tasks={[makeTask({ product: 'tutor' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('tutor')).toBeTruthy();
  });

  it('shows em-dash for empty product', () => {
    render(<TableView tasks={[makeTask({ product: '' })]} onTaskClick={vi.fn()} />);
    const row = screen.getByTestId('table-row-1');
    expect(row.textContent).toContain('\u2014');
  });

  it('shows substep info when set', () => {
    render(<TableView tasks={[makeTask({ substep: 'qa_test' })]} onTaskClick={vi.fn()} />);
    expect(screen.getByText('QA 4/6')).toBeTruthy();
  });

  it('shows em-dash for no substep', () => {
    render(<TableView tasks={[makeTask({ substep: '' })]} onTaskClick={vi.fn()} />);
    const row = screen.getByTestId('table-row-1');
    // Substep column shows em-dash when empty
    expect(row.textContent).toContain('\u2014');
  });

  it('shows relative time', () => {
    render(<TableView tasks={[makeTask()]} onTaskClick={vi.fn()} />);
    const row = screen.getByTestId('table-row-1');
    expect(row.textContent).toContain('just now');
  });

  it('calls onTaskClick when row clicked', () => {
    const onClick = vi.fn();
    const task = makeTask({ id: 5 });
    render(<TableView tasks={[task]} onTaskClick={onClick} />);
    fireEvent.click(screen.getByTestId('table-row-5'));
    expect(onClick).toHaveBeenCalledWith(task);
  });

  it('default sort is priority descending', () => {
    const tasks = [
      makeTask({ id: 1, priority: 0, title: 'Low' }),
      makeTask({ id: 2, priority: 3, title: 'Critical' }),
      makeTask({ id: 3, priority: 1, title: 'Normal' }),
    ];
    const { container } = render(<TableView tasks={tasks} onTaskClick={vi.fn()} />);
    const rows = container.querySelectorAll('[data-testid^="table-row-"]');
    // Highest priority first (desc)
    expect(rows[0].getAttribute('data-testid')).toBe('table-row-2');
    expect(rows[1].getAttribute('data-testid')).toBe('table-row-3');
    expect(rows[2].getAttribute('data-testid')).toBe('table-row-1');
  });

  it('shows sort indicator on active column', () => {
    render(<TableView tasks={[]} onTaskClick={vi.fn()} />);
    // Default sort is priority desc → should show down arrow
    const priorityHeader = screen.getByTestId('table-sort-priority');
    expect(priorityHeader.textContent).toContain('\u2193');
  });

  it('clicking column header changes sort column', () => {
    const tasks = [
      makeTask({ id: 1, title: 'Zebra' }),
      makeTask({ id: 2, title: 'Alpha' }),
    ];
    const { container } = render(<TableView tasks={tasks} onTaskClick={vi.fn()} />);
    // Click Title column
    fireEvent.click(screen.getByTestId('table-sort-title'));
    const rows = container.querySelectorAll('[data-testid^="table-row-"]');
    // Title sort defaults to asc
    expect(rows[0].getAttribute('data-testid')).toBe('table-row-2'); // Alpha
    expect(rows[1].getAttribute('data-testid')).toBe('table-row-1'); // Zebra
  });

  it('clicking same column toggles sort direction', () => {
    const tasks = [
      makeTask({ id: 1, title: 'Alpha' }),
      makeTask({ id: 2, title: 'Zebra' }),
    ];
    const { container } = render(<TableView tasks={tasks} onTaskClick={vi.fn()} />);
    // Click Title (asc)
    fireEvent.click(screen.getByTestId('table-sort-title'));
    // Click Title again (desc)
    fireEvent.click(screen.getByTestId('table-sort-title'));
    const rows = container.querySelectorAll('[data-testid^="table-row-"]');
    expect(rows[0].getAttribute('data-testid')).toBe('table-row-2'); // Zebra first (desc)
  });

  it('sort indicator shows up arrow for ascending', () => {
    render(<TableView tasks={[]} onTaskClick={vi.fn()} />);
    // Click title (defaults to asc)
    fireEvent.click(screen.getByTestId('table-sort-title'));
    const titleHeader = screen.getByTestId('table-sort-title');
    expect(titleHeader.textContent).toContain('\u2191');
  });

  it('renders as table rows', () => {
    render(<TableView tasks={[makeTask()]} onTaskClick={vi.fn()} />);
    const row = screen.getByTestId('table-row-1');
    expect(row.tagName).toBe('TR');
  });

  it('sort by id works', () => {
    const tasks = [
      makeTask({ id: 30 }),
      makeTask({ id: 10 }),
      makeTask({ id: 20 }),
    ];
    const { container } = render(<TableView tasks={tasks} onTaskClick={vi.fn()} />);
    fireEvent.click(screen.getByTestId('table-sort-id'));
    const rows = container.querySelectorAll('[data-testid^="table-row-"]');
    expect(rows[0].getAttribute('data-testid')).toBe('table-row-10');
    expect(rows[1].getAttribute('data-testid')).toBe('table-row-20');
    expect(rows[2].getAttribute('data-testid')).toBe('table-row-30');
  });
});
