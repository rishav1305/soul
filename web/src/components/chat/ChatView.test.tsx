// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, screen, cleanup, fireEvent, act } from '@testing-library/react';

// Mock useChatSessions
const mockSendMessage = vi.fn();
const mockStopStreaming = vi.fn();
const mockRetryFromMessage = vi.fn();
const mockEditMessage = vi.fn();
const mockSetActiveSessionId = vi.fn();

let mockMessages: any[] = [];
let mockIsStreaming = false;
let mockTokenUsage: any = null;
let mockCurrentSessionId: number | null = null;

vi.mock('../../hooks/useChatSessions', () => ({
  useChatSessions: () => ({
    messages: mockMessages,
    isStreaming: mockIsStreaming,
    tokenUsage: mockTokenUsage,
    sendMessage: mockSendMessage,
    stopStreaming: mockStopStreaming,
    retryFromMessage: mockRetryFromMessage,
    editMessage: mockEditMessage,
    activeSessionId: mockCurrentSessionId,
    setActiveSessionId: mockSetActiveSessionId,
  }),
}));

// Mock child components
vi.mock('../MessageList', () => ({
  MessageList: ({ messages, isStreaming, searchQuery }: any) => (
    <div data-testid="message-list" data-count={messages.length} data-streaming={isStreaming} data-search={searchQuery ?? ''}>
      {messages.map((m: any) => (
        <div key={m.id} data-testid={`msg-${m.id}`}>{m.content}</div>
      ))}
    </div>
  ),
}));

vi.mock('../ChatInput', () => ({
  ChatInput: ({ onSend, onStop, isStreaming }: any) => (
    <div data-testid="chat-input" data-streaming={isStreaming}>
      <button data-testid="send-btn" onClick={() => onSend('test message')}>Send</button>
      <button data-testid="stop-btn" onClick={onStop}>Stop</button>
    </div>
  ),
}));

import ChatView from './ChatView';

describe('ChatView', () => {
  beforeEach(() => {
    mockMessages = [];
    mockIsStreaming = false;
    mockTokenUsage = null;
    mockCurrentSessionId = null;
    mockSendMessage.mockClear();
    mockStopStreaming.mockClear();
    mockRetryFromMessage.mockClear();
    mockEditMessage.mockClear();
    mockSetActiveSessionId.mockClear();
  });
  afterEach(() => cleanup());

  it('renders chat-view container', () => {
    render(<ChatView activeSessionId={1} />);
    expect(screen.getByTestId('chat-view')).toBeTruthy();
  });

  it('renders MessageList with adapted messages', () => {
    mockMessages = [
      { id: 'msg-1', role: 'user', content: 'Hello', timestamp: new Date('2026-03-30'), model: undefined, thinking: undefined, toolCalls: undefined },
    ];
    render(<ChatView activeSessionId={1} />);
    const list = screen.getByTestId('message-list');
    expect(list.getAttribute('data-count')).toBe('1');
  });

  it('renders ChatInput', () => {
    render(<ChatView activeSessionId={1} />);
    expect(screen.getByTestId('chat-input')).toBeTruthy();
  });

  it('sends message through ChatInput', () => {
    render(<ChatView activeSessionId={1} />);
    fireEvent.click(screen.getByTestId('send-btn'));
    expect(mockSendMessage).toHaveBeenCalledWith('test message', undefined);
  });

  it('calls stopStreaming when stop clicked', () => {
    mockIsStreaming = true;
    render(<ChatView activeSessionId={1} />);
    fireEvent.click(screen.getByTestId('stop-btn'));
    expect(mockStopStreaming).toHaveBeenCalled();
  });

  it('passes isStreaming to ChatInput', () => {
    mockIsStreaming = true;
    render(<ChatView activeSessionId={1} />);
    expect(screen.getByTestId('chat-input').getAttribute('data-streaming')).toBe('true');
  });

  it('syncs activeSessionId to useChatSessions', () => {
    render(<ChatView activeSessionId={42} />);
    expect(mockSetActiveSessionId).toHaveBeenCalledWith(42);
  });

  it('does not sync null activeSessionId', () => {
    render(<ChatView activeSessionId={null} />);
    expect(mockSetActiveSessionId).not.toHaveBeenCalled();
  });

  it('notifies parent on new session creation', () => {
    const onSessionCreated = vi.fn();
    mockCurrentSessionId = 99;
    render(<ChatView activeSessionId={null} onSessionCreated={onSessionCreated} />);
    expect(onSessionCreated).toHaveBeenCalledWith(99);
  });

  it('displays token usage when not streaming', () => {
    mockTokenUsage = { inputTokens: 1500, outputTokens: 500, contextPct: 0 };
    render(<ChatView activeSessionId={1} />);
    expect(screen.getByText(/1.5k in/)).toBeTruthy();
    expect(screen.getByText(/500 out/)).toBeTruthy();
  });

  it('hides token usage during streaming', () => {
    mockTokenUsage = { inputTokens: 1000, outputTokens: 200, contextPct: 0 };
    mockIsStreaming = true;
    render(<ChatView activeSessionId={1} />);
    expect(screen.queryByText(/1.0k in/)).toBeNull();
  });

  it('shows drag overlay when dragging', () => {
    render(<ChatView activeSessionId={1} />);
    const view = screen.getByTestId('chat-view');

    fireEvent.dragOver(view, { preventDefault: vi.fn() });
    expect(screen.getByText('Drop files to attach')).toBeTruthy();
  });

  it('hides search bar initially', () => {
    render(<ChatView activeSessionId={1} />);
    expect(screen.queryByTestId('chat-search-input')).toBeNull();
  });
});
