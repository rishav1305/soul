// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/react';
import SessionDrawer from './SessionDrawer';
import type { ChatSession } from '../../lib/types';

afterEach(() => cleanup());

function makeSession(overrides: Partial<ChatSession> = {}): ChatSession {
  return {
    id: 1,
    title: 'Test Session',
    summary: 'A test summary',
    model: 'claude-3',
    status: 'idle',
    message_count: 5,
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T12:00:00Z',
    ...overrides,
  } as ChatSession;
}

const defaultProps = {
  sessions: [makeSession()],
  activeSessionId: 1,
  onSelect: vi.fn(),
  onClose: vi.fn(),
};

describe('SessionDrawer', () => {
  it('renders drawer container', () => {
    render(<SessionDrawer {...defaultProps} />);
    expect(screen.getByTestId('session-drawer')).toBeTruthy();
  });

  it('displays session title', () => {
    render(<SessionDrawer {...defaultProps} />);
    expect(screen.getByTestId('session-item-1')).toBeTruthy();
    expect(screen.getByTestId('session-item-1').textContent).toContain('Test Session');
  });

  it('shows Untitled for sessions without title', () => {
    render(<SessionDrawer {...defaultProps} sessions={[makeSession({ title: '' })]} />);
    expect(screen.getByTestId('session-item-1').textContent).toContain('Untitled');
  });

  it('calls onSelect and onClose when session clicked', () => {
    const onSelect = vi.fn();
    const onClose = vi.fn();
    render(<SessionDrawer {...defaultProps} onSelect={onSelect} onClose={onClose} />);

    fireEvent.click(screen.getByTestId('session-item-1'));

    expect(onSelect).toHaveBeenCalledWith(1);
    expect(onClose).toHaveBeenCalled();
  });

  it('calls onClose when backdrop clicked', () => {
    const onClose = vi.fn();
    render(<SessionDrawer {...defaultProps} onClose={onClose} />);
    fireEvent.click(screen.getByTestId('session-drawer-backdrop'));
    expect(onClose).toHaveBeenCalled();
  });

  it('calls onClose when close button clicked', () => {
    const onClose = vi.fn();
    render(<SessionDrawer {...defaultProps} onClose={onClose} />);
    fireEvent.click(screen.getByTestId('session-drawer-close'));
    expect(onClose).toHaveBeenCalled();
  });

  it('filters sessions by search', () => {
    const sessions = [
      makeSession({ id: 1, title: 'React Components' }),
      makeSession({ id: 2, title: 'Go Backend' }),
    ];
    render(<SessionDrawer {...defaultProps} sessions={sessions} />);

    fireEvent.change(screen.getByTestId('session-drawer-search'), {
      target: { value: 'React' },
    });

    expect(screen.queryByTestId('session-item-1')).toBeTruthy();
    expect(screen.queryByTestId('session-item-2')).toBeNull();
  });

  it('filters by summary too', () => {
    const sessions = [
      makeSession({ id: 1, title: 'Chat 1', summary: 'about databases' }),
      makeSession({ id: 2, title: 'Chat 2', summary: 'about frontend' }),
    ];
    render(<SessionDrawer {...defaultProps} sessions={sessions} />);

    fireEvent.change(screen.getByTestId('session-drawer-search'), {
      target: { value: 'frontend' },
    });

    expect(screen.queryByTestId('session-item-1')).toBeNull();
    expect(screen.queryByTestId('session-item-2')).toBeTruthy();
  });

  it('shows no matches message when search has no results', () => {
    render(<SessionDrawer {...defaultProps} />);
    fireEvent.change(screen.getByTestId('session-drawer-search'), {
      target: { value: 'nonexistent' },
    });
    expect(screen.getByText(/No matches for/)).toBeTruthy();
  });

  it('shows empty state when no sessions', () => {
    render(<SessionDrawer {...defaultProps} sessions={[]} />);
    expect(screen.getByText('No conversations yet')).toBeTruthy();
  });

  it('highlights active session', () => {
    const sessions = [
      makeSession({ id: 1, title: 'Active' }),
      makeSession({ id: 2, title: 'Inactive' }),
    ];
    render(<SessionDrawer {...defaultProps} sessions={sessions} activeSessionId={1} />);

    const active = screen.getByTestId('session-item-1');
    expect(active.className).toContain('border-soul');
  });

  it('displays session count', () => {
    const sessions = [
      makeSession({ id: 1 }),
      makeSession({ id: 2 }),
      makeSession({ id: 3 }),
    ];
    render(<SessionDrawer {...defaultProps} sessions={sessions} />);
    // Count is displayed in the header
    expect(screen.getByText('3')).toBeTruthy();
  });

  it('clears search and calls onClose on Escape with empty search', () => {
    const onClose = vi.fn();
    render(<SessionDrawer {...defaultProps} onClose={onClose} />);
    const input = screen.getByTestId('session-drawer-search');
    fireEvent.keyDown(input, { key: 'Escape' });
    expect(onClose).toHaveBeenCalled();
  });

  it('clears search on Escape when search has text', () => {
    const onClose = vi.fn();
    render(<SessionDrawer {...defaultProps} onClose={onClose} />);
    const input = screen.getByTestId('session-drawer-search');

    fireEvent.change(input, { target: { value: 'test' } });
    fireEvent.keyDown(input, { key: 'Escape' });

    // Should clear search, not close
    expect(onClose).not.toHaveBeenCalled();
    expect((input as HTMLInputElement).value).toBe('');
  });
});
