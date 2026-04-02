// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, act } from '@testing-library/react';
import TaskDetail from './TaskDetail';
import type { PlannerTask, TaskStage, PlannerActivity, TaskComment } from '../../lib/types';

// Mock react-markdown to avoid ESM issues
vi.mock('react-markdown', () => ({
  default: ({ children }: { children: string }) => <div data-testid="markdown">{children}</div>,
}));
vi.mock('remark-gfm', () => ({ default: () => {} }));

afterEach(() => cleanup());

function makeTask(overrides: Partial<PlannerTask> = {}): PlannerTask {
  return {
    id: 1,
    title: 'Fix bug',
    description: 'Fix the login bug',
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
  task: makeTask(),
  onClose: vi.fn(),
  onMove: vi.fn().mockResolvedValue(undefined),
  onUpdate: vi.fn().mockResolvedValue(makeTask()),
  onDelete: vi.fn().mockResolvedValue(undefined),
  activities: [] as PlannerActivity[],
  streamContent: '',
  products: ['chat', 'tasks'],
  comments: [] as TaskComment[],
  onFetchComments: vi.fn().mockResolvedValue([]),
  onAddComment: vi.fn().mockResolvedValue({}),
};

describe('TaskDetail', () => {
  it('renders task title and ID', () => {
    render(<TaskDetail {...defaultProps} />);
    expect(screen.getByText('Fix bug')).toBeTruthy();
    expect(screen.getByText('#1')).toBeTruthy();
  });

  it('displays stage badge', () => {
    render(<TaskDetail {...defaultProps} />);
    // Badge and select option both contain 'Backlog' — use getAllByText
    const matches = screen.getAllByText('Backlog');
    expect(matches.length).toBeGreaterThanOrEqual(1);
    // The badge is a span with stage color classes
    const badge = matches.find(el => el.tagName === 'SPAN' && el.className.includes('bg-stage-backlog'));
    expect(badge).toBeTruthy();
  });

  it('calls onClose when close button clicked', () => {
    const onClose = vi.fn();
    render(<TaskDetail {...defaultProps} onClose={onClose} />);
    fireEvent.click(screen.getByTestId('task-detail-close'));
    expect(onClose).toHaveBeenCalled();
  });

  it('calls onDelete when delete button clicked', () => {
    const onDelete = vi.fn().mockResolvedValue(undefined);
    render(<TaskDetail {...defaultProps} onDelete={onDelete} />);
    fireEvent.click(screen.getByTestId('task-detail-delete'));
    expect(onDelete).toHaveBeenCalledWith(1);
  });

  it('displays description in task tab', () => {
    render(<TaskDetail {...defaultProps} />);
    expect(screen.getByText('Fix the login bug')).toBeTruthy();
  });

  it('shows No description for empty description', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ description: '' })} />);
    expect(screen.getByText('No description')).toBeTruthy();
  });

  it('displays error callout when task has error', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ error: 'Build failed' })} />);
    expect(screen.getByText('Build failed')).toBeTruthy();
  });

  it('displays blocker callout when task has blocker', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ blocker: 'Waiting for API' })} />);
    expect(screen.getByText('Waiting for API')).toBeTruthy();
  });

  it('calls onMove when stage is changed', async () => {
    const onMove = vi.fn().mockResolvedValue(undefined);
    render(<TaskDetail {...defaultProps} onMove={onMove} />);

    await act(async () => {
      fireEvent.change(screen.getByTestId('task-detail-stage'), {
        target: { value: 'active' },
      });
    });

    expect(onMove).toHaveBeenCalledWith(1, 'active', '');
  });

  it('calls onUpdate when priority is changed', async () => {
    const onUpdate = vi.fn().mockResolvedValue(makeTask());
    render(<TaskDetail {...defaultProps} onUpdate={onUpdate} />);

    await act(async () => {
      fireEvent.change(screen.getByTestId('task-detail-priority'), {
        target: { value: '2' },
      });
    });

    expect(onUpdate).toHaveBeenCalledWith(1, { priority: 2 });
  });

  it('toggles autonomous mode', async () => {
    const onUpdate = vi.fn().mockResolvedValue(makeTask());
    render(<TaskDetail {...defaultProps} onUpdate={onUpdate} />);

    await act(async () => {
      fireEvent.click(screen.getByTestId('task-detail-autonomous'));
    });

    expect(onUpdate).toHaveBeenCalledWith(1, {
      metadata: JSON.stringify({ autonomous: true }),
    });
  });

  it('shows plan tab with plan content', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ plan: '## Step 1\nDo thing' })} />);

    // Switch to plan tab
    fireEvent.click(screen.getByText('Plan'));
    const md = screen.getByTestId('markdown');
    expect(md.textContent).toContain('## Step 1');
  });

  it('shows No plan message when plan is empty', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ plan: '' })} />);
    fireEvent.click(screen.getByText('Plan'));
    expect(screen.getByText('No plan generated yet')).toBeTruthy();
  });

  it('shows implementation tab with stream content', () => {
    render(<TaskDetail {...defaultProps} streamContent="Working on it..." />);
    fireEvent.click(screen.getByText('Implementation'));
    expect(screen.getByText('Working on it...')).toBeTruthy();
  });

  it('shows Not started yet when no output or stream', () => {
    render(<TaskDetail {...defaultProps} />);
    fireEvent.click(screen.getByText('Implementation'));
    expect(screen.getByText('Not started yet')).toBeTruthy();
  });

  it('shows comments tab and allows posting', async () => {
    const onAddComment = vi.fn().mockResolvedValue({ id: 'c1', body: 'test', author: 'user' });
    render(<TaskDetail {...defaultProps} onAddComment={onAddComment} />);

    // Switch to comments tab
    fireEvent.click(screen.getByText('Comments'));

    const input = screen.getByTestId('task-detail-comment-input');
    fireEvent.change(input, { target: { value: 'Great work' } });

    await act(async () => {
      fireEvent.click(screen.getByTestId('task-detail-submit-comment'));
    });

    expect(onAddComment).toHaveBeenCalledWith(1, 'Great work');
  });

  it('shows No comments yet in empty comments tab', () => {
    render(<TaskDetail {...defaultProps} comments={[]} />);
    fireEvent.click(screen.getByText('Comments'));
    expect(screen.getByText('No comments yet')).toBeTruthy();
  });

  it('fetches comments on mount', () => {
    const onFetchComments = vi.fn().mockResolvedValue([]);
    render(<TaskDetail {...defaultProps} onFetchComments={onFetchComments} />);
    expect(onFetchComments).toHaveBeenCalledWith(1);
  });

  it('displays acceptance criteria', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ acceptance: 'All tests pass' })} />);
    expect(screen.getByText('All tests pass')).toBeTruthy();
  });

  it('Escape key closes the detail', () => {
    const onClose = vi.fn();
    render(<TaskDetail {...defaultProps} onClose={onClose} />);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalled();
  });

  it('shows output content in implementation tab', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ output: 'Build output here' })} />);
    fireEvent.click(screen.getByText('Implementation'));
    expect(screen.getByText('Build output here')).toBeTruthy();
  });

  it('auto-switches to implementation tab when stream content arrives', () => {
    render(<TaskDetail {...defaultProps} streamContent="Streaming..." />);
    // Implementation tab should be auto-selected
    expect(screen.getByText('Streaming...')).toBeTruthy();
  });

  it('displays comment with body text', () => {
    const comment = {
      id: 'c1',
      body: 'Looks good!',
      author: 'user',
      createdAt: '2026-03-30T12:00:00Z',
      type: 'feedback',
    } as any;
    render(<TaskDetail {...defaultProps} comments={[comment]} />);
    // Tab shows "Comments (1)" when comments exist
    fireEvent.click(screen.getByText('Comments (1)'));
    expect(screen.getByText('Looks good!')).toBeTruthy();
  });

  it('does not submit empty comments', async () => {
    const onAddComment = vi.fn().mockResolvedValue({});
    render(<TaskDetail {...defaultProps} onAddComment={onAddComment} />);
    fireEvent.click(screen.getByText('Comments'));

    await act(async () => {
      fireEvent.click(screen.getByTestId('task-detail-submit-comment'));
    });

    expect(onAddComment).not.toHaveBeenCalled();
  });

  it('shows priority selector with current priority', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ priority: 2 })} />);
    const select = screen.getByTestId('task-detail-priority') as HTMLSelectElement;
    expect(select.value).toBe('2');
  });

  it('shows stage selector with current stage', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ stage: 'active' })} />);
    const select = screen.getByTestId('task-detail-stage') as HTMLSelectElement;
    expect(select.value).toBe('active');
  });

  it('shows all tab options', () => {
    render(<TaskDetail {...defaultProps} />);
    expect(screen.getByText('Task')).toBeTruthy();
    expect(screen.getByText('Plan')).toBeTruthy();
    expect(screen.getByText('Implementation')).toBeTruthy();
    expect(screen.getByText('Comments')).toBeTruthy();
  });

  it('shows product label in sidebar', () => {
    render(<TaskDetail {...defaultProps} task={makeTask({ product: 'chat' })} />);
    expect(screen.getByText('Product')).toBeTruthy();
  });
});
