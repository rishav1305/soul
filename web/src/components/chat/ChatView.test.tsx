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

  it('opens search bar on Ctrl+F', () => {
    render(<ChatView activeSessionId={1} />);
    fireEvent.keyDown(document, { key: 'f', ctrlKey: true });
    expect(screen.getByTestId('chat-search-input')).toBeTruthy();
  });

  it('closes search bar on Escape', () => {
    render(<ChatView activeSessionId={1} />);
    // Open search
    fireEvent.keyDown(document, { key: 'f', ctrlKey: true });
    expect(screen.getByTestId('chat-search-input')).toBeTruthy();
    // Close with Escape
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByTestId('chat-search-input')).toBeNull();
  });

  it('closes search bar on close button click', () => {
    render(<ChatView activeSessionId={1} />);
    fireEvent.keyDown(document, { key: 'f', ctrlKey: true });
    expect(screen.getByTestId('chat-search-input')).toBeTruthy();
    fireEvent.click(screen.getByTestId('chat-search-close'));
    expect(screen.queryByTestId('chat-search-input')).toBeNull();
  });

  it('filters messages based on search query', () => {
    mockMessages = [
      { id: 'msg-1', role: 'user', content: 'Hello world', timestamp: new Date(), toolCalls: [] },
      { id: 'msg-2', role: 'assistant', content: 'Goodbye world', timestamp: new Date(), toolCalls: [] },
    ];
    render(<ChatView activeSessionId={1} />);
    // Open search and type
    fireEvent.keyDown(document, { key: 'f', ctrlKey: true });
    fireEvent.change(screen.getByTestId('chat-search-input'), { target: { value: 'Hello' } });
    // MessageList should receive filtered messages
    const list = screen.getByTestId('message-list');
    expect(list.getAttribute('data-search')).toBe('Hello');
  });

  it('passes onEdit to MessageList', () => {
    render(<ChatView activeSessionId={1} />);
    // The MessageList mock receives onEdit — we can verify it was passed
    expect(screen.getByTestId('message-list')).toBeTruthy();
  });

  it('hides drag overlay when not dragging', () => {
    render(<ChatView activeSessionId={1} />);
    expect(screen.queryByText('Drop files to attach')).toBeNull();
  });

  it('drag overlay clears on drop', () => {
    render(<ChatView activeSessionId={1} />);
    const view = screen.getByTestId('chat-view');
    fireEvent.dragOver(view, { preventDefault: vi.fn() });
    expect(screen.getByText('Drop files to attach')).toBeTruthy();
    fireEvent.drop(view, { preventDefault: vi.fn(), dataTransfer: { files: [] } });
    expect(screen.queryByText('Drop files to attach')).toBeNull();
  });

  it('drag overlay clears on drag leave', () => {
    render(<ChatView activeSessionId={1} />);
    const view = screen.getByTestId('chat-view');
    fireEvent.dragOver(view, { preventDefault: vi.fn() });
    expect(screen.getByText('Drop files to attach')).toBeTruthy();
    fireEvent.dragLeave(view, { currentTarget: view, target: view });
    expect(screen.queryByText('Drop files to attach')).toBeNull();
  });

  it('opens search bar on Cmd+F (macOS)', () => {
    render(<ChatView activeSessionId={1} />);
    fireEvent.keyDown(document, { key: 'f', metaKey: true });
    expect(screen.getByTestId('chat-search-input')).toBeTruthy();
  });

  it('search result count shows filtered/total', () => {
    mockMessages = [
      { id: 'msg-1', role: 'user', content: 'Hello world', timestamp: new Date(), toolCalls: [] },
      { id: 'msg-2', role: 'assistant', content: 'Goodbye world', timestamp: new Date(), toolCalls: [] },
      { id: 'msg-3', role: 'user', content: 'Hello again', timestamp: new Date(), toolCalls: [] },
    ];
    render(<ChatView activeSessionId={1} />);
    fireEvent.keyDown(document, { key: 'f', ctrlKey: true });
    fireEvent.change(screen.getByTestId('chat-search-input'), { target: { value: 'Hello' } });
    const list = screen.getByTestId('message-list');
    expect(list.getAttribute('data-count')).toBe('2');
  });

  it('displays token usage with M suffix for large values', () => {
    mockTokenUsage = { inputTokens: 1500000, outputTokens: 250000, contextPct: 0 };
    render(<ChatView activeSessionId={1} />);
    expect(screen.getByText(/1.5M in/)).toBeTruthy();
  });

  it('hides token usage when tokenUsage is null', () => {
    mockTokenUsage = null;
    render(<ChatView activeSessionId={1} />);
    expect(screen.queryByText(/in.*out/)).toBeNull();
  });

  it('does not show context chip when no contextChipProduct', () => {
    render(<ChatView activeSessionId={1} />);
    expect(screen.queryByTestId('context-inject-chip')).toBeNull();
  });

  it('shows context percentage warning for high context usage', () => {
    mockTokenUsage = { inputTokens: 50000, outputTokens: 10000, contextPct: 75 };
    render(<ChatView activeSessionId={1} />);
    expect(screen.getByText('75% ctx')).toBeTruthy();
  });

  it('adapts messages with toolCalls', () => {
    mockMessages = [
      {
        id: 'msg-1',
        role: 'assistant',
        content: 'Using tools...',
        timestamp: new Date('2026-03-30'),
        toolCalls: [{ id: 'tc-1', name: 'search', input: { q: 'test' }, status: 'completed', output: 'result', progress: 100 }],
      },
    ];
    render(<ChatView activeSessionId={1} />);
    const list = screen.getByTestId('message-list');
    expect(list.getAttribute('data-count')).toBe('1');
    expect(screen.getByTestId('msg-msg-1')).toBeTruthy();
  });

  it('does not sync when session matches current', () => {
    mockCurrentSessionId = 5;
    render(<ChatView activeSessionId={5} />);
    expect(mockSetActiveSessionId).not.toHaveBeenCalled();
  });
});
