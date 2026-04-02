// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import TaskContent from './TaskContent';
import type { PlannerTask, TaskStage, GridSubView } from '../../lib/types';

// Mock child components to isolate TaskContent logic
vi.mock('./ListView', () => ({
  default: ({ tasks }: { tasks: PlannerTask[] }) => (
    <div data-testid="mock-list-view">ListView ({tasks.length} tasks)</div>
  ),
}));

vi.mock('./KanbanBoard', () => ({
  default: () => <div data-testid="mock-kanban-board">KanbanBoard</div>,
}));

vi.mock('./grid/CompactGrid', () => ({
  default: ({ tasks }: { tasks: PlannerTask[] }) => (
    <div data-testid="mock-compact-grid">CompactGrid ({tasks.length} tasks)</div>
  ),
}));

vi.mock('./grid/TableView', () => ({
  default: ({ tasks }: { tasks: PlannerTask[] }) => (
    <div data-testid="mock-table-view">TableView ({tasks.length} tasks)</div>
  ),
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

const defaultProps = (overrides: Record<string, unknown> = {}) => ({
  taskView: 'list' as const,
  filteredTasks: [makeTask()],
  tasksByStage: makeTasksByStage([makeTask()]),
  gridSubView: 'compact' as GridSubView,
  onGridSubViewChange: vi.fn(),
  onTaskClick: vi.fn(),
  onClearFilters: vi.fn(),
  ...overrides,
});

describe('TaskContent', () => {
  afterEach(() => cleanup());

  it('shows empty state when no tasks match filters', () => {
    render(<TaskContent {...defaultProps({ filteredTasks: [], tasksByStage: makeTasksByStage() })} />);
    expect(screen.getByText('No tasks match filters')).toBeTruthy();
  });

  it('shows clear filters button in empty state', () => {
    render(<TaskContent {...defaultProps({ filteredTasks: [], tasksByStage: makeTasksByStage() })} />);
    expect(screen.getByTestId('clear-filters')).toBeTruthy();
  });

  it('calls onClearFilters when clear button clicked', () => {
    const onClearFilters = vi.fn();
    render(<TaskContent {...defaultProps({ filteredTasks: [], tasksByStage: makeTasksByStage(), onClearFilters })} />);
    fireEvent.click(screen.getByTestId('clear-filters'));
    expect(onClearFilters).toHaveBeenCalled();
  });

  it('renders ListView for list view', () => {
    render(<TaskContent {...defaultProps({ taskView: 'list' })} />);
    expect(screen.getByTestId('mock-list-view')).toBeTruthy();
  });

  it('renders KanbanBoard for kanban view', () => {
    render(<TaskContent {...defaultProps({ taskView: 'kanban' })} />);
    expect(screen.getByTestId('mock-kanban-board')).toBeTruthy();
  });

  it('renders CompactGrid for grid view', () => {
    render(<TaskContent {...defaultProps({ taskView: 'grid' })} />);
    expect(screen.getByTestId('mock-compact-grid')).toBeTruthy();
  });

  it('renders TableView for table view', () => {
    render(<TaskContent {...defaultProps({ taskView: 'table' })} />);
    expect(screen.getByTestId('mock-table-view')).toBeTruthy();
  });

  it('passes task count to list view', () => {
    const tasks = [makeTask({ id: 1 }), makeTask({ id: 2 }), makeTask({ id: 3 })];
    render(<TaskContent {...defaultProps({ filteredTasks: tasks })} />);
    expect(screen.getByText('ListView (3 tasks)')).toBeTruthy();
  });

  it('passes task count to grid view', () => {
    const tasks = [makeTask({ id: 1 }), makeTask({ id: 2 })];
    render(<TaskContent {...defaultProps({ taskView: 'grid', filteredTasks: tasks })} />);
    expect(screen.getByText('CompactGrid (2 tasks)')).toBeTruthy();
  });

  it('passes task count to table view', () => {
    const tasks = [makeTask({ id: 1 })];
    render(<TaskContent {...defaultProps({ taskView: 'table', filteredTasks: tasks })} />);
    expect(screen.getByText('TableView (1 tasks)')).toBeTruthy();
  });
});
