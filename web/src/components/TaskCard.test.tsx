// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { TaskCard } from './TaskCard';

vi.mock('react-router', () => ({
  Link: ({ to, children, ...props }: { to: string; children: React.ReactNode }) => (
    <a href={to} {...props}>{children}</a>
  ),
}));

vi.mock('../hooks/usePerformance', () => ({
  usePerformance: vi.fn(),
}));

vi.mock('../lib/utils', () => ({
  formatRelativeTime: vi.fn(() => '2m ago'),
}));

function makeTask(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Fix bug',
    description: 'Fix the login bug',
    stage: 'backlog',
    product: 'chat',
    workflow: 'micro',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  } as any;
}

describe('TaskCard', () => {
  afterEach(() => cleanup());

  it('renders task card container', () => {
    render(<TaskCard task={makeTask()} />);
    expect(screen.getByTestId('task-card-1')).toBeTruthy();
  });

  it('shows task title', () => {
    render(<TaskCard task={makeTask({ title: 'Deploy API' })} />);
    expect(screen.getByText('Deploy API')).toBeTruthy();
  });

  it('shows description', () => {
    render(<TaskCard task={makeTask({ description: 'Fix login flow' })} />);
    expect(screen.getByText('Fix login flow')).toBeTruthy();
  });

  it('shows workflow badge', () => {
    render(<TaskCard task={makeTask({ workflow: 'full' })} />);
    expect(screen.getByText('full')).toBeTruthy();
  });

  it('shows relative time', () => {
    render(<TaskCard task={makeTask()} />);
    expect(screen.getByText('2m ago')).toBeTruthy();
  });

  it('shows Start button for backlog tasks', () => {
    render(<TaskCard task={makeTask({ id: 5, stage: 'backlog' })} onStart={vi.fn()} />);
    expect(screen.getByTestId('start-task-5')).toBeTruthy();
  });

  it('calls onStart when Start clicked', () => {
    const onStart = vi.fn();
    render(<TaskCard task={makeTask({ id: 5, stage: 'backlog' })} onStart={onStart} />);
    fireEvent.click(screen.getByTestId('start-task-5'));
    expect(onStart).toHaveBeenCalledWith(5);
  });

  it('shows Stop button for active tasks', () => {
    render(<TaskCard task={makeTask({ id: 3, stage: 'active' })} onStop={vi.fn()} />);
    expect(screen.getByTestId('stop-task-3')).toBeTruthy();
  });

  it('calls onStop when Stop clicked', () => {
    const onStop = vi.fn();
    render(<TaskCard task={makeTask({ id: 3, stage: 'active' })} onStop={onStop} />);
    fireEvent.click(screen.getByTestId('stop-task-3'));
    expect(onStop).toHaveBeenCalledWith(3);
  });

  it('hides description when empty', () => {
    const { container } = render(<TaskCard task={makeTask({ description: '' })} />);
    const desc = container.querySelector('.line-clamp-2');
    expect(desc).toBeNull();
  });

  it('links to task detail page', () => {
    render(<TaskCard task={makeTask({ id: 7 })} />);
    const link = screen.getByText('Fix bug') as HTMLAnchorElement;
    expect(link.getAttribute('href')).toBe('/tasks/7');
  });
});
