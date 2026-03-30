// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeAll, beforeEach } from 'vitest';
import { render, screen, cleanup, fireEvent, act } from '@testing-library/react';

// ── Mock useChat hook ──
const mockSendMessage = vi.fn();
vi.mock('../../hooks/useChat.ts', () => ({
  useChat: () => ({
    sendMessage: mockSendMessage,
    isStreaming: false,
  }),
}));

// ── Mock authFetch ──
vi.mock('../../lib/api.ts', () => ({
  authFetch: vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve({ models: [{ id: 'claude-3', name: 'Claude 3', description: 'Test' }] }),
  }),
}));

// ── Mock child components ──
vi.mock('../chat/ChatView.tsx', () => ({
  default: ({ activeSessionId }: any) => (
    <div data-testid="chat-view" data-session={activeSessionId}>ChatView</div>
  ),
}));

vi.mock('../chat/SessionDrawer.tsx', () => ({
  default: ({ onClose, onSelect }: any) => (
    <div data-testid="session-drawer">
      <button data-testid="drawer-close" onClick={onClose}>Close</button>
      <button data-testid="drawer-select" onClick={() => onSelect(42)}>Select</button>
    </div>
  ),
}));

vi.mock('../planner/TaskContent.tsx', () => ({
  default: ({ taskView, filteredTasks, onTaskClick, onClearFilters }: any) => (
    <div data-testid="task-content" data-view={taskView} data-count={filteredTasks.length}>
      {filteredTasks[0] && (
        <button data-testid="task-click" onClick={() => onTaskClick(filteredTasks[0])}>Click</button>
      )}
      <button data-testid="clear-filters" onClick={onClearFilters}>Clear</button>
    </div>
  ),
}));

vi.mock('../planner/TaskDetail.tsx', () => ({
  default: ({ task, onClose, onDelete }: any) => (
    <div data-testid="task-detail" data-task-id={task.id}>
      <button data-testid="detail-close" onClick={onClose}>Close</button>
      <button data-testid="detail-delete" onClick={() => onDelete(task.id)}>Delete</button>
    </div>
  ),
}));

vi.mock('../planner/NewTaskForm.tsx', () => ({
  default: ({ onClose, onCreate }: any) => (
    <div data-testid="new-task-form">
      <button data-testid="form-create" onClick={() => onCreate('T', 'D', 1, 'chat')}>Create</button>
      <button data-testid="form-close" onClick={onClose}>Close</button>
    </div>
  ),
}));

// ── ResizeObserver polyfill ──
beforeAll(() => {
  global.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as any;
});

import HorizontalRail from './HorizontalRail';
import type {
  PlannerTask,
  TaskStage,
  TaskView,
  GridSubView,
  TaskFilters,
  HorizontalRailTab,
  DrawerLayout,
  PlannerActivity,
  TaskComment,
} from '../../lib/types';

// ── Helpers ──
function makeTask(overrides: Partial<PlannerTask> = {}): PlannerTask {
  return {
    id: 1,
    title: 'Test Task',
    description: 'Desc',
    stage: 'active' as TaskStage,
    priority: 1,
    product: 'chat',
    workflow: 'micro',
    metadata: '{}',
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    ...overrides,
  } as PlannerTask;
}

const baseProps = {
  expanded: false,
  heightVh: 40,
  tab: 'chat' as HorizontalRailTab,
  chatSplitPct: 50,
  drawerLayout: 'tabbed' as DrawerLayout,
  position: 'bottom' as const,
  onToggleExpand: vi.fn(),
  onSetTab: vi.fn(),
  onHeightChange: vi.fn(),
  onChatSplitChange: vi.fn(),
  activeSessionId: 1,
  sessions: [],
  onSessionCreated: vi.fn(),
  onSessionSelect: vi.fn(),
  onNewSession: vi.fn(),
  runningSessions: [],
  unreadSessions: [],
  lastChatSnippet: undefined,
  tasks: [makeTask(), makeTask({ id: 2, stage: 'backlog' as TaskStage })],
  activeProduct: null as string | null,
  taskActivities: {} as Record<number, PlannerActivity[]>,
  taskStreams: {} as Record<number, string>,
  taskComments: {} as Record<number, TaskComment[]>,
  updateTask: vi.fn().mockResolvedValue(makeTask()),
  moveTask: vi.fn().mockResolvedValue(makeTask()),
  deleteTask: vi.fn().mockResolvedValue(undefined),
  fetchComments: vi.fn().mockResolvedValue([]),
  addComment: vi.fn().mockResolvedValue({} as TaskComment),
  products: ['chat', 'tasks'],
  createTask: vi.fn().mockResolvedValue(makeTask({ id: 3 })),
  taskView: 'list' as TaskView,
  gridSubView: 'priority' as GridSubView,
  filters: { stage: 'all', priority: 'all', product: 'all' } as TaskFilters,
  setTaskView: vi.fn(),
  setGridSubView: vi.fn(),
  setFilters: vi.fn(),
  syncProductFilter: false,
  onSyncProductFilterToggle: vi.fn(),
  buildContextString: vi.fn().mockReturnValue(''),
  autoInjectContext: true,
  showContextChip: true,
  inlineBadgesEnabled: true,
};

describe('HorizontalRail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  afterEach(() => cleanup());

  // ─── Collapsed rail bar (tabbed) ───
  describe('collapsed — tabbed layout', () => {
    it('renders horizontal-rail testid', () => {
      render(<HorizontalRail {...baseProps} />);
      expect(screen.getByTestId('horizontal-rail')).toBeTruthy();
    });

    it('renders chat type selector', () => {
      render(<HorizontalRail {...baseProps} />);
      expect(screen.getByLabelText('Select chat mode')).toBeTruthy();
    });

    it('renders expand button with correct aria', () => {
      render(<HorizontalRail {...baseProps} />);
      const btn = screen.getByLabelText('Expand drawer');
      expect(btn).toBeTruthy();
      expect(btn.getAttribute('aria-expanded')).toBe('false');
    });

    it('calls onToggleExpand when expand button clicked', () => {
      const onToggleExpand = vi.fn();
      render(<HorizontalRail {...baseProps} onToggleExpand={onToggleExpand} />);
      fireEvent.click(screen.getByLabelText('Expand drawer'));
      expect(onToggleExpand).toHaveBeenCalled();
    });

    it('shows stage counts for tasks', () => {
      render(<HorizontalRail {...baseProps} />);
      // 1 active, 1 backlog
      expect(screen.getByText(/1 active/)).toBeTruthy();
      expect(screen.getByText(/1 backlog/)).toBeTruthy();
    });

    it('shows "No tasks" when tasks empty', () => {
      render(<HorizontalRail {...baseProps} tasks={[]} />);
      expect(screen.getByText('No tasks')).toBeTruthy();
    });

    it('renders new task button in rail', () => {
      render(<HorizontalRail {...baseProps} />);
      expect(screen.getByLabelText('New task')).toBeTruthy();
    });

    it('renders sync filter button', () => {
      render(<HorizontalRail {...baseProps} />);
      expect(screen.getByTitle('Sync OFF — showing all tasks')).toBeTruthy();
    });

    it('shows sync ON title when syncProductFilter is true', () => {
      render(<HorizontalRail {...baseProps} syncProductFilter={true} />);
      const btns = screen.getAllByTitle(/Sync ON/);
      expect(btns.length).toBeGreaterThanOrEqual(1);
    });
  });

  // ─── Collapsed rail bar (independent) ───
  describe('collapsed — independent layout', () => {
    it('renders two separate expand buttons for chat and tasks', () => {
      render(
        <HorizontalRail {...baseProps} drawerLayout="independent" />,
      );
      expect(screen.getByLabelText('Expand chat')).toBeTruthy();
      expect(screen.getByLabelText('Expand tasks')).toBeTruthy();
    });
  });

  // ─── Expanded panel (tabbed / split) ───
  describe('expanded — tabbed layout', () => {
    const expandedProps = { ...baseProps, expanded: true };

    it('renders ChatView in expanded panel', () => {
      render(<HorizontalRail {...expandedProps} />);
      expect(screen.getByTestId('chat-view')).toBeTruthy();
    });

    it('renders TaskContent in expanded panel', () => {
      render(<HorizontalRail {...expandedProps} />);
      expect(screen.getByTestId('task-content')).toBeTruthy();
    });

    it('passes activeSessionId to ChatView', () => {
      render(<HorizontalRail {...expandedProps} activeSessionId={99} />);
      expect(screen.getByTestId('chat-view').getAttribute('data-session')).toBe('99');
    });

    it('renders Chat and Tasks tab buttons', () => {
      render(<HorizontalRail {...expandedProps} />);
      // "Chat" appears in both tab button and rail content — use getAllByText
      const chatMatches = screen.getAllByText('Chat');
      expect(chatMatches.length).toBeGreaterThanOrEqual(1);
      const tasksMatches = screen.getAllByText('Tasks');
      expect(tasksMatches.length).toBeGreaterThanOrEqual(1);
    });

    it('renders collapse button', () => {
      render(<HorizontalRail {...expandedProps} />);
      expect(screen.getByLabelText('Collapse panel')).toBeTruthy();
    });

    it('renders new chat button', () => {
      render(<HorizontalRail {...expandedProps} />);
      expect(screen.getByLabelText('New chat')).toBeTruthy();
    });

    it('calls onNewSession when new chat button clicked', () => {
      const onNewSession = vi.fn();
      render(<HorizontalRail {...expandedProps} onNewSession={onNewSession} />);
      fireEvent.click(screen.getByLabelText('New chat'));
      expect(onNewSession).toHaveBeenCalled();
    });

    it('renders view mode buttons (list/kanban/grid/table)', () => {
      render(<HorizontalRail {...expandedProps} />);
      expect(screen.getByTitle('List view')).toBeTruthy();
      expect(screen.getByTitle('Kanban view')).toBeTruthy();
      expect(screen.getByTitle('Grid view')).toBeTruthy();
      expect(screen.getByTitle('Table view')).toBeTruthy();
    });

    it('switches task view on button click', () => {
      const setTaskView = vi.fn();
      render(<HorizontalRail {...expandedProps} setTaskView={setTaskView} />);
      fireEvent.click(screen.getByTitle('Kanban view'));
      expect(setTaskView).toHaveBeenCalledWith('kanban');
    });

    it('renders stage and priority filter selects', () => {
      render(<HorizontalRail {...expandedProps} />);
      expect(screen.getByTitle('Filter by stage')).toBeTruthy();
      expect(screen.getByTitle('Filter by priority')).toBeTruthy();
    });

    it('shows drag handle at top for bottom position', () => {
      render(<HorizontalRail {...expandedProps} position="bottom" />);
      const handles = screen.getByTestId('horizontal-rail').querySelectorAll('[title="Drag to resize"]');
      expect(handles.length).toBeGreaterThanOrEqual(1);
    });

    it('renders + New button in tasks header', () => {
      render(<HorizontalRail {...expandedProps} />);
      const newBtns = screen.getAllByText('+ New');
      expect(newBtns.length).toBeGreaterThanOrEqual(1);
    });

    it('renders history button', () => {
      render(<HorizontalRail {...expandedProps} />);
      expect(screen.getByLabelText('Chat history')).toBeTruthy();
    });

    it('renders refresh credentials button', () => {
      render(<HorizontalRail {...expandedProps} />);
      expect(screen.getByLabelText('Refresh AI credentials')).toBeTruthy();
    });

    it('shows running sessions badge when running', () => {
      render(
        <HorizontalRail
          {...expandedProps}
          runningSessions={[{ id: 1, title: 'Run', created_at: '', updated_at: '' } as any]}
        />,
      );
      expect(screen.getByTitle('1 running session')).toBeTruthy();
    });

    it('shows unread sessions badge when unread', () => {
      render(
        <HorizontalRail
          {...expandedProps}
          unreadSessions={[{ id: 2, title: 'Unread', created_at: '', updated_at: '' } as any]}
        />,
      );
      expect(screen.getByTitle('1 unread session')).toBeTruthy();
    });
  });

  // ─── Session drawer ───
  describe('session drawer', () => {
    const expandedProps = { ...baseProps, expanded: true };

    it('shows session drawer when history button clicked', () => {
      render(<HorizontalRail {...expandedProps} />);
      expect(screen.queryByTestId('session-drawer')).toBeNull();
      fireEvent.click(screen.getByLabelText('Chat history'));
      expect(screen.getByTestId('session-drawer')).toBeTruthy();
    });

    it('closes session drawer and calls onSessionSelect', () => {
      const onSessionSelect = vi.fn();
      render(<HorizontalRail {...expandedProps} onSessionSelect={onSessionSelect} />);
      // Open drawer
      fireEvent.click(screen.getByLabelText('Chat history'));
      // Select session
      fireEvent.click(screen.getByTestId('drawer-select'));
      expect(onSessionSelect).toHaveBeenCalledWith(42);
      // Drawer closes after select
      expect(screen.queryByTestId('session-drawer')).toBeNull();
    });
  });

  // ─── Task detail modal ───
  describe('task detail modal', () => {
    const expandedProps = { ...baseProps, expanded: true };

    it('opens task detail when task clicked in TaskContent', () => {
      render(<HorizontalRail {...expandedProps} />);
      fireEvent.click(screen.getByTestId('task-click'));
      expect(screen.getByTestId('task-detail')).toBeTruthy();
    });

    it('closes task detail on close button', () => {
      render(<HorizontalRail {...expandedProps} />);
      fireEvent.click(screen.getByTestId('task-click'));
      expect(screen.getByTestId('task-detail')).toBeTruthy();
      fireEvent.click(screen.getByTestId('detail-close'));
      expect(screen.queryByTestId('task-detail')).toBeNull();
    });

    it('deletes task and closes detail', async () => {
      const deleteTask = vi.fn().mockResolvedValue(undefined);
      render(<HorizontalRail {...expandedProps} deleteTask={deleteTask} />);
      fireEvent.click(screen.getByTestId('task-click'));
      await act(async () => {
        fireEvent.click(screen.getByTestId('detail-delete'));
      });
      expect(deleteTask).toHaveBeenCalledWith(1);
      expect(screen.queryByTestId('task-detail')).toBeNull();
    });
  });

  // ─── New task form ───
  describe('new task form', () => {
    const expandedProps = { ...baseProps, expanded: true };

    it('shows new task form on + New click', () => {
      render(<HorizontalRail {...expandedProps} />);
      const newBtns = screen.getAllByText('+ New');
      fireEvent.click(newBtns[0]!);
      expect(screen.getByTestId('new-task-form')).toBeTruthy();
    });

    it('creates task and closes form', async () => {
      const createTask = vi.fn().mockResolvedValue(makeTask({ id: 5 }));
      render(<HorizontalRail {...expandedProps} createTask={createTask} />);
      // Open form via expanded panel + New button
      const newBtns = screen.getAllByText('+ New');
      fireEvent.click(newBtns[0]!);
      await act(async () => {
        fireEvent.click(screen.getByTestId('form-create'));
      });
      expect(createTask).toHaveBeenCalledWith('T', 'D', 1, 'chat');
      expect(screen.queryByTestId('new-task-form')).toBeNull();
    });
  });

  // ─── Task filtering ───
  describe('task filtering', () => {
    it('filters tasks by syncProductFilter', () => {
      render(
        <HorizontalRail
          {...baseProps}
          expanded={true}
          syncProductFilter={true}
          activeProduct="chat"
          tasks={[
            makeTask({ id: 1, product: 'chat' }),
            makeTask({ id: 2, product: 'other' }),
          ]}
        />,
      );
      // Only the chat task should pass through
      expect(screen.getByTestId('task-content').getAttribute('data-count')).toBe('1');
    });

    it('shows all tasks when syncProductFilter is off', () => {
      render(
        <HorizontalRail
          {...baseProps}
          expanded={true}
          syncProductFilter={false}
          tasks={[
            makeTask({ id: 1, product: 'chat' }),
            makeTask({ id: 2, product: 'other' }),
          ]}
        />,
      );
      expect(screen.getByTestId('task-content').getAttribute('data-count')).toBe('2');
    });

    it('applies stage filter', () => {
      render(
        <HorizontalRail
          {...baseProps}
          expanded={true}
          filters={{ stage: 'active', priority: 'all', product: 'all' }}
          tasks={[
            makeTask({ id: 1, stage: 'active' }),
            makeTask({ id: 2, stage: 'backlog' }),
          ]}
        />,
      );
      expect(screen.getByTestId('task-content').getAttribute('data-count')).toBe('1');
    });
  });

  // ─── Expanded — independent layout ───
  describe('expanded — independent layout', () => {
    it('renders chat and tasks as separate expandable sections', () => {
      render(
        <HorizontalRail {...baseProps} expanded={true} drawerLayout="independent" />,
      );
      // In independent mode, collapsed bars are shown for each panel
      // Since expanded=true but chatOpen/tasksOpen start false, we see collapsed bars
      const chatExpand = screen.getByLabelText('Expand chat');
      const tasksExpand = screen.getByLabelText('Expand tasks');
      expect(chatExpand).toBeTruthy();
      expect(tasksExpand).toBeTruthy();
    });
  });

  // ─── visiblePanels prop ───
  describe('visiblePanels', () => {
    it('hides tasks rail when visiblePanels=chat', () => {
      render(<HorizontalRail {...baseProps} drawerLayout="independent" visiblePanels="chat" />);
      // Only chat expand should be present
      expect(screen.getByLabelText('Expand chat')).toBeTruthy();
      expect(screen.queryByLabelText('Expand tasks')).toBeNull();
    });

    it('hides chat rail when visiblePanels=tasks', () => {
      render(<HorizontalRail {...baseProps} drawerLayout="independent" visiblePanels="tasks" />);
      expect(screen.queryByLabelText('Expand chat')).toBeNull();
      expect(screen.getByLabelText('Expand tasks')).toBeTruthy();
    });
  });
});
