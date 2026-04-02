// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';

// Mock all heavy dependencies to test AppShell orchestration in isolation

vi.mock('../../hooks/useLayoutStore.ts', () => ({
  useLayoutStore: () => ({
    activeProduct: 'soul',
    setActiveProduct: vi.fn(),
    panelExpanded: false,
    setPanelExpanded: vi.fn(),
    settingsOpen: false,
    setSettingsOpen: vi.fn(),
    chatPosition: 'bottom',
    setChatPosition: vi.fn(),
    tasksPosition: 'bottom',
    setTasksPosition: vi.fn(),
    drawerLayout: 'tabbed',
    setDrawerLayout: vi.fn(),
    railTab: 'chat',
    setRailTab: vi.fn(),
    railExpanded: true,
    setRailExpanded: vi.fn(),
    railHeightVh: 40,
    setRailHeightVh: vi.fn(),
    chatRailExpanded: true,
    setChatRailExpanded: vi.fn(),
    chatRailHeightVh: 40,
    setChatRailHeightVh: vi.fn(),
    tasksRailExpanded: true,
    setTasksRailExpanded: vi.fn(),
    tasksRailHeightVh: 40,
    setTasksRailHeightVh: vi.fn(),
    chatSplitPct: 50,
    setChatSplitPct: vi.fn(),
    taskView: 'list',
    setTaskView: vi.fn(),
    gridSubView: 'priority',
    setGridSubView: vi.fn(),
    panelWidth: null,
    setPanelWidth: vi.fn(),
    filters: { stage: 'all', priority: 'all', product: 'all' },
    setFilters: vi.fn(),
    sessionsOpen: false,
    setSessionsOpen: vi.fn(),
    toastsEnabled: true,
    setToastsEnabled: vi.fn(),
    autoInjectContext: true,
    setAutoInjectContext: vi.fn(),
    showContextChip: true,
    setShowContextChip: vi.fn(),
    inlineBadgesEnabled: true,
    setInlineBadgesEnabled: vi.fn(),
    rightChatExpanded: true,
    setRightChatExpanded: vi.fn(),
    rightTasksExpanded: true,
    setRightTasksExpanded: vi.fn(),
    rightPanelWidth: 400,
    setRightPanelWidth: vi.fn(),
    rightChatWidth: 400,
    setRightChatWidth: vi.fn(),
    rightTasksWidth: 400,
    setRightTasksWidth: vi.fn(),
    syncProductFilter: false,
    setSyncProductFilter: vi.fn(),
  }),
}));

vi.mock('../../hooks/usePlanner.ts', () => ({
  usePlanner: () => ({
    tasks: [],
    tasksByStage: { backlog: [], brainstorm: [], active: [], blocked: [], validation: [], done: [] },
    loading: false,
    taskActivities: {},
    taskStreams: {},
    taskComments: {},
    createTask: vi.fn(),
    updateTask: vi.fn(),
    moveTask: vi.fn(),
    deleteTask: vi.fn(),
    fetchComments: vi.fn(),
    addComment: vi.fn(),
  }),
}));

vi.mock('../../hooks/useNotifications.ts', () => ({
  useNotifications: () => ({ toasts: [], dismiss: vi.fn() }),
}));

vi.mock('../../hooks/useProductContext.ts', () => ({
  useProductContext: () => ({ buildContextString: vi.fn().mockReturnValue('') }),
}));

vi.mock('../../hooks/useMediaQuery.ts', () => ({
  useMediaQuery: () => false,
}));

vi.mock('../../hooks/useChatSessions.tsx', () => ({
  ChatSessionsProvider: ({ children }: any) => <div>{children}</div>,
  useChatSessions: () => ({
    sessions: [],
    activeSessionId: null,
    setActiveSessionId: vi.fn(),
    createSession: vi.fn(),
    messages: [],
    runningSessions: new Set(),
    unreadSessions: new Set(),
    connected: true,
  }),
}));

vi.mock('../../hooks/useWebSocketContext.ts', () => ({
  WebSocketContext: { Provider: ({ children }: any) => <div>{children}</div> },
  useWebSocketProvider: () => ({
    send: vi.fn(),
    onMessage: vi.fn().mockReturnValue(() => {}),
    connected: true,
  }),
}));

vi.mock('../../lib/api.ts', () => ({
  authFetch: vi.fn().mockResolvedValue({
    json: () => Promise.resolve([]),
  }),
}));

// Mock child layout components
vi.mock('./ProductRail.tsx', () => ({
  default: () => <div data-testid="product-rail">ProductRail</div>,
  RAIL_WIDTH: 48,
  PANEL_WIDTH: 200,
}));

vi.mock('./ProductView.tsx', () => ({
  default: () => <div data-testid="product-view">ProductView</div>,
}));

vi.mock('./HorizontalRail.tsx', () => ({
  default: () => <div data-testid="horizontal-rail">HorizontalRail</div>,
}));

vi.mock('./RightPanel.tsx', () => ({
  default: () => <div data-testid="right-panel">RightPanel</div>,
}));

vi.mock('./SessionsDrawer.tsx', () => ({
  default: () => <div data-testid="sessions-drawer">SessionsDrawer</div>,
}));

vi.mock('./ToastStack.tsx', () => ({
  default: () => <div data-testid="toast-stack">ToastStack</div>,
}));

vi.mock('../planner/TaskDetail.tsx', () => ({
  default: () => <div data-testid="task-detail">TaskDetail</div>,
}));

beforeAll(() => {
  global.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as any;
});

import AppShell from './AppShell';

describe('AppShell', () => {
  afterEach(() => cleanup());

  it('renders the app shell container', () => {
    render(<AppShell />);
    expect(screen.getByTestId('app-shell')).toBeTruthy();
  });

  it('renders ProductRail', () => {
    render(<AppShell />);
    expect(screen.getByTestId('product-rail')).toBeTruthy();
  });

  it('renders ProductView in main content area', () => {
    render(<AppShell />);
    expect(screen.getByTestId('product-view')).toBeTruthy();
  });

  it('renders horizontal rail for bottom panel', () => {
    render(<AppShell />);
    // Both chat and tasks are set to 'bottom' position
    expect(screen.getByTestId('horizontal-rail')).toBeTruthy();
  });

  it('renders toast stack', () => {
    render(<AppShell />);
    expect(screen.getByTestId('toast-stack')).toBeTruthy();
  });

  it('contains skip-to-content link for accessibility', () => {
    render(<AppShell />);
    const link = screen.getByText('Skip to main content');
    expect(link).toBeTruthy();
    expect(link.getAttribute('href')).toBe('#main-content');
  });

  it('renders main content area with correct id', () => {
    render(<AppShell />);
    const main = document.getElementById('main-content');
    expect(main).toBeTruthy();
    expect(main?.tagName).toBe('MAIN');
  });

  it('has dark theme background class', () => {
    render(<AppShell />);
    const shell = screen.getByTestId('app-shell');
    expect(shell.className).toContain('bg-deep');
  });

  it('has noise overlay class', () => {
    render(<AppShell />);
    const shell = screen.getByTestId('app-shell');
    expect(shell.className).toContain('noise');
  });

  it('has overflow-hidden to prevent scroll leaking', () => {
    render(<AppShell />);
    const shell = screen.getByTestId('app-shell');
    expect(shell.className).toContain('overflow-hidden');
  });

  it('has full screen height', () => {
    render(<AppShell />);
    const shell = screen.getByTestId('app-shell');
    expect(shell.className).toContain('h-screen');
  });

  it('renders with font-body class', () => {
    render(<AppShell />);
    const shell = screen.getByTestId('app-shell');
    expect(shell.className).toContain('font-body');
  });

  it('renders text-fg for default text color', () => {
    render(<AppShell />);
    const shell = screen.getByTestId('app-shell');
    expect(shell.className).toContain('text-fg');
  });

  it('does not render right panel when positions are bottom', () => {
    render(<AppShell />);
    // Both chatPosition and tasksPosition are 'bottom' in mock, so no right panel
    expect(screen.queryByTestId('right-panel')).toBeNull();
  });

  it('does not render sessions drawer when closed', () => {
    render(<AppShell />);
    // sessionsOpen is false in mock
    expect(screen.queryByTestId('sessions-drawer')).toBeNull();
  });

  it('skip-to-content link has sr-only class', () => {
    render(<AppShell />);
    const link = screen.getByText('Skip to main content');
    expect(link.className).toContain('sr-only');
  });

  it('shell uses flex column layout', () => {
    render(<AppShell />);
    const shell = screen.getByTestId('app-shell');
    expect(shell.className).toContain('flex');
    expect(shell.className).toContain('flex-col');
  });
});
