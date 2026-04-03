// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
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

  it('renders the sidebar', () => {
    renderDetail('happy');
    expect(screen.getByTestId('agent-detail-sidebar')).toBeTruthy();
  });

  // ── Backlog: empty state ────────────────────────────────────────────────────

  it('shows "No active tasks" when no tasks assigned to agent', () => {
    mockTasks = [];
    renderDetail('happy');
    expect(screen.getByText('No active tasks')).toBeTruthy();
  });

  it('shows "No active tasks" when all tasks are assigned to a different agent', () => {
    mockTasks = [makeTask({ assignee: 'shuri' }), makeTask({ assignee: 'fury' })];
    renderDetail('happy');
    expect(screen.getByText('No active tasks')).toBeTruthy();
  });

  it('shows "No active tasks" when all agent tasks are in done stage', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', stage: 'done' }),
      makeTask({ assignee: 'happy', stage: 'done' }),
    ];
    renderDetail('happy');
    expect(screen.getByText('No active tasks')).toBeTruthy();
  });

  // ── Backlog: task list ──────────────────────────────────────────────────────

  it('renders tasks assigned to the agent', () => {
    mockTasks = [makeTask({ assignee: 'happy', title: 'Fix the tests' })];
    renderDetail('happy');
    expect(screen.getByText('Fix the tests')).toBeTruthy();
  });

  it('shows the task count in the section header', () => {
    mockTasks = [
      makeTask({ assignee: 'happy' }),
      makeTask({ assignee: 'happy' }),
    ];
    renderDetail('happy');
    expect(screen.getByText('2')).toBeTruthy();
  });

  it('does not show the count header when no tasks', () => {
    mockTasks = [];
    renderDetail('happy');
    // The number "0" should not be visible as a count badge
    const backlogSection = screen.getByTestId('agent-backlog');
    // Heading should say "Backlog" without a number sibling
    expect(backlogSection.textContent).not.toContain('0');
  });

  it('excludes done tasks from the count', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', stage: 'backlog' }),
      makeTask({ assignee: 'happy', stage: 'done' }),
    ];
    renderDetail('happy');
    // Only 1 active task → count badge should be "1"
    const backlogSection = screen.getByTestId('agent-backlog');
    expect(backlogSection.textContent).toContain('1');
    expect(backlogSection.textContent).not.toContain('2');
  });

  it('renders individual task rows with data-testid', () => {
    const task = makeTask({ assignee: 'happy', id: 'abc-123' });
    mockTasks = [task];
    renderDetail('happy');
    expect(screen.getByTestId('agent-task-abc-123')).toBeTruthy();
  });

  // ── Backlog: sorting ────────────────────────────────────────────────────────

  it('shows in-progress tasks before backlog tasks', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', id: 'backlog-first', stage: 'backlog', title: 'Pending work' }),
      makeTask({ assignee: 'happy', id: 'active-first', stage: 'in-progress', title: 'Active work' }),
    ];
    renderDetail('happy');
    const rows = screen.getAllByRole('listitem');
    // Active work should appear before Pending work
    const firstRow = rows[0]?.textContent ?? '';
    const secondRow = rows[1]?.textContent ?? '';
    expect(firstRow).toContain('Active work');
    expect(secondRow).toContain('Pending work');
  });

  it('shows blocked tasks before backlog tasks but after in-progress', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', id: 'bl', stage: 'backlog',     title: 'Backlog item' }),
      makeTask({ assignee: 'happy', id: 'ip', stage: 'in-progress', title: 'In progress' }),
      makeTask({ assignee: 'happy', id: 'bk', stage: 'blocked',     title: 'Blocked item' }),
    ];
    renderDetail('happy');
    const rows = screen.getAllByRole('listitem');
    expect(rows[0]?.textContent).toContain('In progress');
    expect(rows[1]?.textContent).toContain('Blocked item');
    expect(rows[2]?.textContent).toContain('Backlog item');
  });

  // ── Backlog: capping & overflow ─────────────────────────────────────────────

  it('caps visible tasks at 5', () => {
    mockTasks = Array.from({ length: 8 }, (_, i) =>
      makeTask({ assignee: 'happy', id: `t-${i}`, title: `Task ${i}` }),
    );
    renderDetail('happy');
    const rows = screen.getAllByRole('listitem');
    expect(rows).toHaveLength(5);
  });

  it('shows "+N more" footer when tasks exceed 5', () => {
    mockTasks = Array.from({ length: 7 }, (_, i) =>
      makeTask({ assignee: 'happy', id: `t-${i}`, title: `Task ${i}` }),
    );
    renderDetail('happy');
    expect(screen.getByText('+2 more — see Task Board')).toBeTruthy();
  });

  it('does not show overflow footer when tasks are 5 or fewer', () => {
    mockTasks = Array.from({ length: 5 }, (_, i) =>
      makeTask({ assignee: 'happy', id: `t-${i}`, title: `Task ${i}` }),
    );
    renderDetail('happy');
    expect(screen.queryByText(/\+\d+ more/)).toBeNull();
  });

  // ── Backlog: priority indicators ────────────────────────────────────────────

  it('renders a priority dot for each task', () => {
    mockTasks = [
      makeTask({ assignee: 'happy', priority: 'critical' }),
      makeTask({ assignee: 'happy', priority: 'high' }),
    ];
    renderDetail('happy');
    // Each list item has a priority dot span
    const rows = screen.getAllByRole('listitem');
    expect(rows).toHaveLength(2);
    rows.forEach((row) => {
      expect(row.querySelector('[aria-label^="Priority:"]')).toBeTruthy();
    });
  });

  it('labels priority dot correctly for critical tasks', () => {
    mockTasks = [makeTask({ assignee: 'happy', priority: 'critical' })];
    renderDetail('happy');
    const dot = screen.getByLabelText('Priority: critical');
    expect(dot).toBeTruthy();
  });

  // ── Backlog: stage labels ───────────────────────────────────────────────────

  it('shows "active" label for in-progress tasks', () => {
    mockTasks = [makeTask({ assignee: 'happy', stage: 'in-progress' })];
    renderDetail('happy');
    expect(screen.getByText('active')).toBeTruthy();
  });

  it('shows "blocked" label for blocked tasks', () => {
    mockTasks = [makeTask({ assignee: 'happy', stage: 'blocked' })];
    renderDetail('happy');
    expect(screen.getByText('blocked')).toBeTruthy();
  });

  it('shows "backlog" label for backlog tasks', () => {
    mockTasks = [makeTask({ assignee: 'happy', stage: 'backlog' })];
    renderDetail('happy');
    expect(screen.getByText('backlog')).toBeTruthy();
  });

  // ── Send message flow ───────────────────────────────────────────────────────

  it('renders the send message button', () => {
    renderDetail('happy');
    expect(screen.getByTestId('agent-detail-send-message-btn')).toBeTruthy();
  });
});
