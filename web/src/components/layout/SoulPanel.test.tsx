// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import SoulPanel from './SoulPanel';
import type { ChatSession } from '../../lib/types';

function makeSession(overrides: Partial<ChatSession> = {}): ChatSession {
  return {
    id: 1,
    title: 'Test Session',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  } as ChatSession;
}

const defaultProps = () => ({
  onCollapse: vi.fn(),
  sessions: [] as ChatSession[],
  activeSessionId: null as number | null,
  onSessionSelect: vi.fn(),
  onNewChat: vi.fn(),
  connected: true,
});

describe('SoulPanel', () => {
  afterEach(() => cleanup());

  it('renders soul panel', () => {
    render(<SoulPanel {...defaultProps()} />);
    expect(screen.getByTestId('soul-panel')).toBeTruthy();
  });

  it('shows Soul title', () => {
    render(<SoulPanel {...defaultProps()} />);
    expect(screen.getByText('Soul')).toBeTruthy();
  });

  it('renders collapse button', () => {
    render(<SoulPanel {...defaultProps()} />);
    expect(screen.getByTestId('soul-panel-collapse-btn')).toBeTruthy();
  });

  it('calls onCollapse when collapse button clicked', () => {
    const props = defaultProps();
    render(<SoulPanel {...props} />);
    fireEvent.click(screen.getByTestId('soul-panel-collapse-btn'));
    expect(props.onCollapse).toHaveBeenCalled();
  });

  it('renders new chat button', () => {
    render(<SoulPanel {...defaultProps()} />);
    expect(screen.getByTestId('soul-panel-new-chat-btn')).toBeTruthy();
    expect(screen.getByText('New Chat')).toBeTruthy();
  });

  it('calls onNewChat when new chat clicked', () => {
    const props = defaultProps();
    render(<SoulPanel {...props} />);
    fireEvent.click(screen.getByTestId('soul-panel-new-chat-btn'));
    expect(props.onNewChat).toHaveBeenCalled();
  });

  it('renders session items', () => {
    const sessions = [
      makeSession({ id: 1, title: 'Chat about Go' }),
      makeSession({ id: 2, title: 'React help' }),
    ];
    render(<SoulPanel {...defaultProps()} sessions={sessions} />);
    expect(screen.getByTestId('soul-panel-session-1')).toBeTruthy();
    expect(screen.getByTestId('soul-panel-session-2')).toBeTruthy();
    expect(screen.getByText('Chat about Go')).toBeTruthy();
    expect(screen.getByText('React help')).toBeTruthy();
  });

  it('calls onSessionSelect when session clicked', () => {
    const props = defaultProps();
    props.sessions = [makeSession({ id: 5, title: 'My Session' })];
    render(<SoulPanel {...props} />);
    fireEvent.click(screen.getByTestId('soul-panel-session-5'));
    expect(props.onSessionSelect).toHaveBeenCalledWith(5);
  });

  it('shows fallback title when session has no title', () => {
    const sessions = [makeSession({ id: 3, title: '' })];
    render(<SoulPanel {...defaultProps()} sessions={sessions} />);
    expect(screen.getByText('Session 3')).toBeTruthy();
  });

  it('shows connected indicator when connected', () => {
    render(<SoulPanel {...defaultProps()} />);
    expect(screen.getByText('Connected')).toBeTruthy();
  });

  it('shows disconnected indicator when not connected', () => {
    render(<SoulPanel {...defaultProps()} connected={false} />);
    expect(screen.getByText('Disconnected')).toBeTruthy();
  });

  it('shows Sessions label', () => {
    render(<SoulPanel {...defaultProps()} />);
    expect(screen.getByText('Sessions')).toBeTruthy();
  });
});
