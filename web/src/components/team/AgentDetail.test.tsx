// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/react';
import AgentDetail from './AgentDetail';
import type { TeamTask } from '../../hooks/useTeamTasks.ts';

// ── Mock leaf components ──────────────────────────────────────────────────────
vi.mock('./StatusDot.tsx', () => ({
  StatusDot: ({ status }: { status: string }) => (
    <span data-testid="mock-status-dot">{status}</span>
  ),
}));
vi.mock('./ModelBadge.tsx', () => ({
  ModelBadge: ({ model }: { model: string }) => (
    <span data-testid="mock-model-badge">{model}</span>
  ),
}));
vi.mock('./PaneViewer.tsx', () => ({
  PaneViewer: () => <div data-testid="mock-pane-viewer" />,
}));

// ── Mock hooks ────────────────────────────────────────────────────────────────
const mockAgents = [
  {
    name: 'happy',
    role: 'QA Lead',
    model: 'claude-sonnet',
    machine: 'titan-pc',
    status: 'working' as const,
    task: 'Running tests',
    contextPct: 42,
    inboxCount: 1,
    lastSeen: Date.now(),
  },
];

vi.mock('../../hooks/useTeamAgents.ts', () => ({
  useTeamAgents: () => ({ agents: mockAgents }),
}));

vi.mock('../../hooks/useAgentPane.ts', () => ({
  useAgentPane: () => ({ lines: [], loading: false }),
}));

vi.mock('../../lib/api.ts', () => ({
  authFetch: vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) }),
}));

// Mutable tasks list for per-test control
let mockTasks: TeamTask[] = [];
vi.mock('../../hooks/useTeamTasks.ts', () => ({
  useTeamTasks: () => ({ tasks: mockTasks }),
}));

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeTask(overrides: Partial<TeamTask> = {}): TeamTask {
  return {
    id: `task-${Math.random().toString(36).slice(2, 8)}`,
    title: 'Default task title',
    priority: 'medium',
    stage: 'backlog',
    assignee: 'happy',
    createdAt: Date.now(),
    ...overrides,
  };
}

function renderDetail(agentName = 'happy') {
  return render(
    <AgentDetail agentName={agentName} onNavigate={vi.fn()} />,
  );
}

/** Click the backlog toggle to reveal task list. */
function openBacklog() {
  const toggle = screen.getByTestId('agent-backlog-toggle');
  fireEvent.click(toggle);
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('AgentDetail', () => {
  afterEach(() => {
    mockTasks = [];
    cleanup();
  });

  // ── Rendering ──────────────────────────────────────────────────────────────

  it('renders the detail panel for the given agent', () => {
    renderDetail('happy');
    expect(screen.getByTestId('agent-detail-happy')).toBeTruthy();
  });

  it('renders the back button', () => {
    renderDetail('happy');
    expect(screen.getByTestId('agent-detail-back')).toBeTruthy();
  });

  it('renders the pane viewer', () => {
    renderDetail('happy');
    expect(screen.getByTestId('mock-pane-viewer')).toBeTruthy();
  });

  it('renders the compact info bar with context usage', () => {
    renderDetail('happy');
    expect(screen.getByTestId('agent-context-bar')).toBeTruthy();
  });

  it('shows current task in info bar', () => {
    renderDetail('happy');
    expect(screen.getByTestId('agent-info-task')).toBeTruthy();
    expect(screen.getByText('Running tests')).toBeTruthy();
  });

  // ── Backlog: empty state ────────────────────────────────────────────────────

  it('does not show backlog toggle when no tasks assigned', () => {
    mockTasks = [];
    renderDetail('happy');
    expect(screen.queryByTestId('agent-backlog-toggle')).toBeNull();
  });

  it('does not show backlog toggle when all tasks assigned to a different agent', () => {
    mockTasks = [makeTask({ assignee: 'shuri' }), makeTask({ assignee: 'fury' })];
    renderDetail('happy');
    expect(screen.queryByTestId('agent-backlog-toggle')).toBeNull();
  });

  it('does not show backlog toggle when all agent tasks are done', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', stage: 'done' }),
      makeTask({ assignee: 'happy', stage: 'done' }),
    ];
    renderDetail('happy');
    expect(screen.queryByTestId('agent-backlog-toggle')).toBeNull();
  });

  // ── Backlog: toggle and task count ─────────────────────────────────────────

  it('shows backlog toggle with task count when agent has active tasks', () => {
    mockTasks = [
      makeTask({ assignee: 'happy' }),
      makeTask({ assignee: 'happy' }),
    ];
    renderDetail('happy');
    const toggle = screen.getByTestId('agent-backlog-toggle');
    expect(toggle).toBeTruthy();
    expect(toggle.textContent).toContain('2');
  });

  it('excludes done tasks from the count', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', stage: 'backlog' }),
      makeTask({ assignee: 'happy', stage: 'done' }),
    ];
    renderDetail('happy');
    const toggle = screen.getByTestId('agent-backlog-toggle');
    expect(toggle.textContent).toContain('1');
    expect(toggle.textContent).not.toContain('2');
  });

  // ── Backlog: task list (after toggle open) ─────────────────────────────────

  it('renders task list when backlog is opened', () => {
    mockTasks = [makeTask({ assignee: 'happy', title: 'Fix the tests' })];
    renderDetail('happy');
    openBacklog();
    expect(screen.getByTestId('agent-backlog')).toBeTruthy();
    expect(screen.getByText('Fix the tests')).toBeTruthy();
  });

  it('renders individual task rows with data-testid', () => {
    const task = makeTask({ assignee: 'happy', id: 'abc-123' });
    mockTasks = [task];
    renderDetail('happy');
    openBacklog();
    expect(screen.getByTestId('agent-task-abc-123')).toBeTruthy();
  });

  // ── Backlog: sorting ────────────────────────────────────────────────────────

  it('shows in-progress tasks before backlog tasks', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', id: 'backlog-first', stage: 'backlog', title: 'Pending work' }),
      makeTask({ assignee: 'happy', id: 'active-first', stage: 'in-progress', title: 'Active work' }),
    ];
    renderDetail('happy');
    openBacklog();
    const rows = screen.getAllByRole('listitem');
    expect(rows[0]?.textContent).toContain('Active work');
    expect(rows[1]?.textContent).toContain('Pending work');
  });

  it('shows blocked tasks before backlog tasks but after in-progress', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', id: 'bl', stage: 'backlog',     title: 'Backlog item' }),
      makeTask({ assignee: 'happy', id: 'ip', stage: 'in-progress', title: 'In progress' }),
      makeTask({ assignee: 'happy', id: 'bk', stage: 'blocked',     title: 'Blocked item' }),
    ];
    renderDetail('happy');
    openBacklog();
    const rows = screen.getAllByRole('listitem');
    expect(rows[0]?.textContent).toContain('In progress');
    expect(rows[1]?.textContent).toContain('Blocked item');
    expect(rows[2]?.textContent).toContain('Backlog item');
  });

  // ── Backlog: all tasks shown (no cap in new layout) ────────────────────────

  it('shows all tasks without capping', () => {
    mockTasks = Array.from({ length: 8 }, (_, i) =>
      makeTask({ assignee: 'happy', id: `t-${i}`, title: `Task ${i}` }),
    );
    renderDetail('happy');
    openBacklog();
    const rows = screen.getAllByRole('listitem');
    expect(rows).toHaveLength(8);
  });

  it('does not show overflow footer when tasks are 5 or fewer', () => {
    mockTasks = Array.from({ length: 5 }, (_, i) =>
      makeTask({ assignee: 'happy', id: `t-${i}`, title: `Task ${i}` }),
    );
    renderDetail('happy');
    openBacklog();
    expect(screen.queryByText(/\+\d+ more/)).toBeNull();
  });

  // ── Backlog: priority indicators ────────────────────────────────────────────

  it('renders a priority dot for each task', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', priority: 'critical' }),
      makeTask({ assignee: 'happy', priority: 'high' }),
    ];
    renderDetail('happy');
    openBacklog();
    const rows = screen.getAllByRole('listitem');
    expect(rows).toHaveLength(2);
    rows.forEach((row) => {
      expect(row.querySelector('[aria-label^="Priority:"]')).toBeTruthy();
    });
  });

  it('labels priority dot correctly for critical tasks', () => {
    mockTasks = [makeTask({ assignee: 'happy', priority: 'critical' })];
    renderDetail('happy');
    openBacklog();
    const dot = screen.getByLabelText('Priority: critical');
    expect(dot).toBeTruthy();
  });

  // ── Backlog: stage labels ───────────────────────────────────────────────────

  it('shows "active" label for in-progress tasks', () => {
    mockTasks = [makeTask({ assignee: 'happy', stage: 'in-progress' })];
    renderDetail('happy');
    openBacklog();
    expect(screen.getByText('active')).toBeTruthy();
  });

  it('shows "blocked" label for blocked tasks', () => {
    mockTasks = [makeTask({ assignee: 'happy', stage: 'blocked' })];
    renderDetail('happy');
    openBacklog();
    expect(screen.getByText('blocked')).toBeTruthy();
  });

  it('shows "backlog" label for backlog tasks', () => {
    mockTasks = [makeTask({ assignee: 'happy', stage: 'backlog' })];
    renderDetail('happy');
    openBacklog();
    expect(screen.getByText('backlog')).toBeTruthy();
  });

  // ── Send message flow ───────────────────────────────────────────────────────

  it('renders the send message button', () => {
    renderDetail('happy');
    expect(screen.getByTestId('agent-detail-send-message-btn')).toBeTruthy();
  });
});
