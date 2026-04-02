// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest';
import { render, screen, cleanup, fireEvent, act } from '@testing-library/react';
import TaskPanel from './TaskPanel';
import type { PlannerTask, TaskStage, TaskView, GridSubView, TaskFilters, PlannerActivity, TaskComment } from '../../lib/types';

// Mock child components to isolate TaskPanel logic
vi.mock('./FilterBar.tsx', () => ({
  default: ({ filters, onChange }: any) => (
    <div data-testid="filter-bar" data-stage={filters.stage}>
      <button data-testid="filter-change" onClick={() => onChange({ stage: 'active' })}>Filter</button>
    </div>
  ),
}));

vi.mock('./TaskContent.tsx', () => ({
  default: ({ taskView, filteredTasks, onTaskClick, onClearFilters }: any) => (
    <div data-testid="task-content" data-view={taskView} data-count={filteredTasks.length}>
      <button data-testid="task-click" onClick={() => onTaskClick(filteredTasks[0])}>Click Task</button>
      <button data-testid="clear-filters" onClick={onClearFilters}>Clear</button>
    </div>
  ),
}));

vi.mock('./TaskDetail.tsx', () => ({
  default: ({ task, onClose, onDelete }: any) => (
    <div data-testid="task-detail-mock" data-task-id={task.id}>
      <button data-testid="mock-close" onClick={onClose}>Close</button>
      <button data-testid="mock-delete" onClick={() => onDelete(task.id)}>Delete</button>
    </div>
  ),
}));

vi.mock('./NewTaskForm.tsx', () => ({
  default: ({ onClose, onCreate }: any) => (
    <div data-testid="new-task-form">
      <button data-testid="form-create" onClick={() => onCreate('Test', 'Desc', 1, 'chat')}>Create</button>
      <button data-testid="form-close" onClick={onClose}>Close</button>
    </div>
  ),
}));

// Mock ResizeObserver for jsdom
beforeAll(() => {
  global.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as any;
});

afterEach(() => cleanup());

function makeTask(overrides: Partial<PlannerTask> = {}): PlannerTask {
  return {
    id: 1,
    title: 'Test Task',
    description: 'Test desc',
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

function defaultTasksByStage(): Record<TaskStage, PlannerTask[]> {
  return {
    backlog: [makeTask()],
    brainstorm: [],
    active: [],
    blocked: [],
    validation: [],
    done: [],
  };
}

const defaultProps = {
  taskView: 'list' as TaskView,
  gridSubView: 'priority' as GridSubView,
  panelWidth: null as number | null,
  filters: { stage: 'all', priority: 'all', product: 'all' } as TaskFilters,
  setTaskView: vi.fn(),
  setGridSubView: vi.fn(),
  setPanelWidth: vi.fn(),
  setFilters: vi.fn(),
  canCollapse: true,
  onCollapse: vi.fn(),
  tasks: [makeTask()],
  filteredTasks: [makeTask()],
  tasksByStage: defaultTasksByStage(),
  products: ['chat', 'tasks'],
  loading: false,
  createTask: vi.fn().mockResolvedValue(makeTask({ id: 2 })),
  updateTask: vi.fn().mockResolvedValue(makeTask()),
  moveTask: vi.fn().mockResolvedValue(makeTask()),
  deleteTask: vi.fn().mockResolvedValue(undefined),
  taskActivities: {} as Record<number, PlannerActivity[]>,
  taskStreams: {} as Record<number, string>,
  taskComments: {} as Record<number, TaskComment[]>,
  fetchComments: vi.fn().mockResolvedValue([]),
  addComment: vi.fn().mockResolvedValue({} as TaskComment),
};

describe('TaskPanel', () => {
  it('renders Tasks header', () => {
    render(<TaskPanel {...defaultProps} />);
    expect(screen.getByText('Tasks')).toBeTruthy();
  });

  it('renders product scope header when productScope set', () => {
    render(<TaskPanel {...defaultProps} productScope="scout" />);
    expect(screen.getByText('scout')).toBeTruthy();
  });

  it('renders view mode buttons', () => {
    render(<TaskPanel {...defaultProps} />);
    expect(screen.getByTestId('view-mode-list')).toBeTruthy();
    expect(screen.getByTestId('view-mode-kanban')).toBeTruthy();
    expect(screen.getByTestId('view-mode-grid')).toBeTruthy();
    expect(screen.getByTestId('view-mode-table')).toBeTruthy();
  });

  it('switches view mode on button click', () => {
    const setTaskView = vi.fn();
    render(<TaskPanel {...defaultProps} setTaskView={setTaskView} />);
    fireEvent.click(screen.getByTestId('view-mode-kanban'));
    expect(setTaskView).toHaveBeenCalledWith('kanban');
  });

  it('renders new task button', () => {
    render(<TaskPanel {...defaultProps} />);
    expect(screen.getByTestId('new-task-button')).toBeTruthy();
  });

  it('shows new task form on button click', () => {
    render(<TaskPanel {...defaultProps} />);
    expect(screen.queryByTestId('new-task-form')).toBeNull();

    fireEvent.click(screen.getByTestId('new-task-button'));
    expect(screen.getByTestId('new-task-form')).toBeTruthy();
  });

  it('shows loading state', () => {
    render(<TaskPanel {...defaultProps} loading={true} />);
    expect(screen.getByText('Loading tasks...')).toBeTruthy();
  });

  it('renders TaskContent when not loading', () => {
    render(<TaskPanel {...defaultProps} />);
    expect(screen.getByTestId('task-content')).toBeTruthy();
    expect(screen.getByTestId('task-content').getAttribute('data-count')).toBe('1');
  });

  it('opens task detail on task click', () => {
    render(<TaskPanel {...defaultProps} />);
    fireEvent.click(screen.getByTestId('task-click'));
    expect(screen.getByTestId('task-detail-mock')).toBeTruthy();
  });

  it('closes task detail', () => {
    render(<TaskPanel {...defaultProps} />);
    // Open
    fireEvent.click(screen.getByTestId('task-click'));
    expect(screen.getByTestId('task-detail-mock')).toBeTruthy();
    // Close
    fireEvent.click(screen.getByTestId('mock-close'));
    expect(screen.queryByTestId('task-detail-mock')).toBeNull();
  });

  it('deletes task and closes detail', async () => {
    const deleteTask = vi.fn().mockResolvedValue(undefined);
    render(<TaskPanel {...defaultProps} deleteTask={deleteTask} />);
    // Open detail
    fireEvent.click(screen.getByTestId('task-click'));
    // Delete
    await act(async () => {
      fireEvent.click(screen.getByTestId('mock-delete'));
    });
    expect(deleteTask).toHaveBeenCalledWith(1);
    expect(screen.queryByTestId('task-detail-mock')).toBeNull();
  });

  it('creates task from new form', async () => {
    const createTask = vi.fn().mockResolvedValue(makeTask({ id: 2 }));
    render(<TaskPanel {...defaultProps} createTask={createTask} />);

    fireEvent.click(screen.getByTestId('new-task-button'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('form-create'));
    });

    expect(createTask).toHaveBeenCalledWith('Test', 'Desc', 1, 'chat');
    // Form should close after creation
    expect(screen.queryByTestId('new-task-form')).toBeNull();
  });

  it('shows reset width button when panelWidth is set', () => {
    render(<TaskPanel {...defaultProps} panelWidth={500} />);
    expect(screen.getByTestId('reset-panel-width')).toBeTruthy();
  });

  it('hides reset width button when panelWidth is null', () => {
    render(<TaskPanel {...defaultProps} panelWidth={null} />);
    expect(screen.queryByTestId('reset-panel-width')).toBeNull();
  });

  it('passes canCollapse to collapse button', () => {
    render(<TaskPanel {...defaultProps} canCollapse={false} />);
    const btn = screen.getByTestId('collapse-tasks');
    expect(btn.hasAttribute('disabled')).toBe(true);
  });

  it('collapse button calls onCollapse', () => {
    const onCollapse = vi.fn();
    render(<TaskPanel {...defaultProps} canCollapse={true} onCollapse={onCollapse} />);
    fireEvent.click(screen.getByTestId('collapse-tasks'));
    expect(onCollapse).toHaveBeenCalled();
  });

  it('active view mode has active styling', () => {
    render(<TaskPanel {...defaultProps} taskView="kanban" />);
    const kanbanBtn = screen.getByTestId('view-mode-kanban');
    // Active button has bg-overlay distinguishing it from non-active
    expect(kanbanBtn.className).toContain('bg-overlay');
  });

  it('non-active view mode lacks active styling', () => {
    render(<TaskPanel {...defaultProps} taskView="list" />);
    const kanbanBtn = screen.getByTestId('view-mode-kanban');
    expect(kanbanBtn.className).not.toContain('text-soul');
  });

  it('passes taskView to TaskContent', () => {
    render(<TaskPanel {...defaultProps} taskView="kanban" />);
    expect(screen.getByTestId('task-content').getAttribute('data-view')).toBe('kanban');
  });

  it('renders filter bar', () => {
    render(<TaskPanel {...defaultProps} />);
    expect(screen.getByTestId('filter-bar')).toBeTruthy();
  });

  it('filter change calls setFilters', () => {
    const setFilters = vi.fn();
    render(<TaskPanel {...defaultProps} setFilters={setFilters} />);
    fireEvent.click(screen.getByTestId('filter-change'));
    expect(setFilters).toHaveBeenCalledWith({ stage: 'active' });
  });

  it('reset width button calls setPanelWidth with null', () => {
    const setPanelWidth = vi.fn();
    render(<TaskPanel {...defaultProps} panelWidth={500} setPanelWidth={setPanelWidth} />);
    fireEvent.click(screen.getByTestId('reset-panel-width'));
    expect(setPanelWidth).toHaveBeenCalledWith(null);
  });

  it('closes new task form on form close button', () => {
    render(<TaskPanel {...defaultProps} />);
    fireEvent.click(screen.getByTestId('new-task-button'));
    expect(screen.getByTestId('new-task-form')).toBeTruthy();
    fireEvent.click(screen.getByTestId('form-close'));
    expect(screen.queryByTestId('new-task-form')).toBeNull();
  });

  it('shows correct task count in TaskContent', () => {
    const filteredTasks = [makeTask({ id: 1 }), makeTask({ id: 2 })];
    render(<TaskPanel {...defaultProps} filteredTasks={filteredTasks} />);
    expect(screen.getByTestId('task-content').getAttribute('data-count')).toBe('2');
  });

  it('clears filters via TaskContent clear button', () => {
    const setFilters = vi.fn();
    render(<TaskPanel {...defaultProps} setFilters={setFilters} />);
    fireEvent.click(screen.getByTestId('clear-filters'));
    expect(setFilters).toHaveBeenCalledWith({ stage: 'all', priority: 'all', product: 'all' });
  });
});
