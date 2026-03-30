// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import ProductView from './ProductView';
import type { PlannerTask, TaskStage, TaskView, GridSubView, TaskFilters, PlannerActivity, TaskComment } from '../../lib/types';

// Mock TaskPanel
vi.mock('../planner/TaskPanel.tsx', () => ({
  default: ({ taskView }: any) => (
    <div data-testid="task-panel" data-view={taskView}>TaskPanel</div>
  ),
}));

// Mock PlaceholderPanel
vi.mock('../panels/PlaceholderPanel.tsx', () => ({
  default: () => null,
}));

// Mock telemetry
vi.mock('../../lib/telemetry.ts', () => ({
  reportError: vi.fn(),
}));

afterEach(() => cleanup());

function makeTask(overrides: Partial<PlannerTask> = {}): PlannerTask {
  return {
    id: 1,
    title: 'Test',
    description: '',
    stage: 'backlog' as TaskStage,
    priority: 1,
    product: 'chat',
    workflow: 'micro',
    metadata: '{}',
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    ...overrides,
  } as PlannerTask;
}

const defaultProps = {
  activeProduct: null as string | null,
  taskView: 'list' as TaskView,
  gridSubView: 'priority' as GridSubView,
  panelWidth: null as number | null,
  filters: { stage: 'all', priority: 'all', product: 'all' } as TaskFilters,
  setTaskView: vi.fn(),
  setGridSubView: vi.fn(),
  setPanelWidth: vi.fn(),
  setFilters: vi.fn(),
  tasks: [makeTask()],
  filteredTasks: [makeTask()],
  tasksByStage: {
    backlog: [makeTask()],
    brainstorm: [],
    active: [],
    blocked: [],
    validation: [],
    done: [],
  } as Record<TaskStage, PlannerTask[]>,
  products: ['chat'],
  loading: false,
  createTask: vi.fn().mockResolvedValue(makeTask()),
  updateTask: vi.fn().mockResolvedValue(makeTask()),
  moveTask: vi.fn().mockResolvedValue(makeTask()),
  deleteTask: vi.fn().mockResolvedValue(undefined),
  taskActivities: {} as Record<number, PlannerActivity[]>,
  taskStreams: {} as Record<number, string>,
  taskComments: {} as Record<number, TaskComment[]>,
  fetchComments: vi.fn(),
  addComment: vi.fn(),
};

describe('ProductView', () => {
  it('renders TaskPanel when no active product', () => {
    render(<ProductView {...defaultProps} activeProduct={null} />);
    expect(screen.getByTestId('task-panel')).toBeTruthy();
  });

  it('renders TaskPanel for placeholder products', () => {
    render(<ProductView {...defaultProps} activeProduct="scout" />);
    // Scout is registered as PlaceholderPanel, falls through to TaskPanel
    expect(screen.getByTestId('task-panel')).toBeTruthy();
  });

  it('renders TaskPanel for unknown products', () => {
    render(<ProductView {...defaultProps} activeProduct="unknown-product" />);
    expect(screen.getByTestId('task-panel')).toBeTruthy();
  });

  it('renders TaskPanel for soul product', () => {
    render(<ProductView {...defaultProps} activeProduct="soul" />);
    expect(screen.getByTestId('task-panel')).toBeTruthy();
  });

  it('passes taskView to TaskPanel', () => {
    render(<ProductView {...defaultProps} activeProduct={null} taskView="kanban" />);
    expect(screen.getByTestId('task-panel').getAttribute('data-view')).toBe('kanban');
  });
});
