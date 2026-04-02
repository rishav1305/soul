// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';

// --- Capture onMessage from useWebSocket ---

let capturedOnMessage: ((type: string, data: unknown, sessionID: string, messageId?: string) => void) | null = null;
const mockSend = vi.fn();
const mockReconnect = vi.fn();

vi.mock('./useWebSocket', () => ({
  useWebSocket: (opts: { onMessage?: (...args: unknown[]) => void }) => {
    capturedOnMessage = opts.onMessage as typeof capturedOnMessage ?? null;
    return {
      status: 'connected' as const,
      send: mockSend,
      reconnectAttempt: 0,
      reconnect: mockReconnect,
      authError: false,
    };
  },
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportWSLatency: vi.fn(),
  reportUsage: vi.fn(),
  reportAuthFailure: vi.fn(),
}));

vi.mock('../components/AuthGate', () => ({
  getToken: vi.fn().mockReturnValue('test-token'),
}));

import { useChat } from './useChat';

function fireWS(type: string, data: unknown, sessionID = 'sess-1', messageId?: string) {
  act(() => {
    capturedOnMessage!(type, data, sessionID, messageId);
  });
}

describe('useChat', () => {
  beforeEach(() => {
    capturedOnMessage = null;
    mockSend.mockClear();
    mockReconnect.mockClear();
    localStorage.clear();
  });
  afterEach(() => cleanup());

  // --- Initial state ---

  it('starts with empty messages', () => {
    const { result } = renderHook(() => useChat());
    expect(result.current.messages).toEqual([]);
    expect(result.current.isStreaming).toBe(false);
    expect(result.current.currentSessionID).toBeNull();
    expect(result.current.sessions).toEqual([]);
    expect(result.current.activeProduct).toBe('');
  });

  it('captures onMessage handler from useWebSocket', () => {
    renderHook(() => useChat());
    expect(capturedOnMessage).not.toBeNull();
  });

  // --- connection.ready ---

  it('dispatches ws:connected on connection.ready', () => {
    renderHook(() => useChat());
    const listener = vi.fn();
    window.addEventListener('ws:connected', listener);

    fireWS('connection.ready', {}, '');
    expect(listener).toHaveBeenCalled();
    window.removeEventListener('ws:connected', listener);
  });

  it('restores session from localStorage on first connection.ready', async () => {
    localStorage.setItem('soul-v2-session', 'saved-sess');
    renderHook(() => useChat());

    fireWS('connection.ready', {}, '');

    // The hook uses queueMicrotask for session.switch — wait for it
    await waitFor(() => {
      expect(mockSend).toHaveBeenCalledWith('session.switch', { sessionId: 'saved-sess' });
    });
  });

  // --- session.created ---

  it('adds session to list on session.created', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.created', {
      session: { id: 'new-sess', title: 'New Session', messageCount: 0 },
    }, '');

    expect(result.current.sessions.length).toBe(1);
    expect(result.current.sessions[0].id).toBe('new-sess');
    expect(result.current.currentSessionID).toBe('new-sess');
  });

  it('does not duplicate session on repeated session.created', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.created', {
      session: { id: 'new-sess', title: 'New', messageCount: 0 },
    }, '');
    fireWS('session.created', {
      session: { id: 'new-sess', title: 'New', messageCount: 0 },
    }, '');

    expect(result.current.sessions.length).toBe(1);
  });

  // --- session.list ---

  it('populates sessions on session.list', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.list', {
      sessions: [
        { id: 's1', title: 'Session 1', messageCount: 5 },
        { id: 's2', title: 'Session 2', messageCount: 3 },
      ],
    }, '');

    expect(result.current.sessions.length).toBe(2);
    // Auto-selects first session when none active
    expect(result.current.currentSessionID).toBe('s1');
  });

  // --- session.updated ---

  it('updates session in list on session.updated', () => {
    const { result } = renderHook(() => useChat());

    // First populate
    fireWS('session.list', {
      sessions: [{ id: 's1', title: 'Old', messageCount: 1 }],
    }, '');

    // Then update
    fireWS('session.updated', {
      session: { id: 's1', title: 'Updated', messageCount: 5 },
    }, '');

    expect(result.current.sessions[0].title).toBe('Updated');
  });

  // --- session.deleted ---

  it('removes session on session.deleted', () => {
    const { result } = renderHook(() => useChat());

    // Populate with 2 sessions
    fireWS('session.list', {
      sessions: [
        { id: 's1', title: 'S1', messageCount: 1 },
        { id: 's2', title: 'S2', messageCount: 2 },
      ],
    }, '');

    // Delete the active session
    fireWS('session.deleted', { sessionId: 's1' }, '');

    expect(result.current.sessions.length).toBe(1);
    expect(result.current.sessions[0].id).toBe('s2');
    // Should switch to next available session
    expect(result.current.currentSessionID).toBe('s2');
  });

  it('clears state when last session is deleted', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.created', {
      session: { id: 'only', title: 'Only', messageCount: 0 },
    }, '');

    fireWS('session.deleted', { sessionId: 'only' }, '');

    expect(result.current.sessions.length).toBe(0);
    expect(result.current.currentSessionID).toBeNull();
  });

  // --- session.history ---

  it('hydrates messages on session.history', () => {
    const { result } = renderHook(() => useChat());

    // Set up current session
    fireWS('session.created', {
      session: { id: 'sess-1', title: 'Chat', messageCount: 1 },
    }, '');

    fireWS('session.history', {
      messages: [
        { id: 'msg-1', role: 'user', content: 'Hello', sessionId: 'sess-1' },
        { id: 'msg-2', role: 'assistant', content: 'Hi!', sessionId: 'sess-1' },
      ],
    }, 'sess-1');

    expect(result.current.messages.length).toBe(2);
    expect(result.current.messages[0].role).toBe('user');
    expect(result.current.messages[1].role).toBe('assistant');
  });

  it('sets product from session.history payload', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.created', {
      session: { id: 'sess-1', title: 'Chat', messageCount: 0 },
    }, '');

    fireWS('session.history', {
      messages: [],
      session: { product: 'tasks' },
    }, 'sess-1');

    expect(result.current.activeProduct).toBe('tasks');
  });

  // --- session.productSet ---

  it('updates active product on session.productSet', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.productSet', { product: 'scout' }, 'sess-1');

    expect(result.current.activeProduct).toBe('scout');
  });

  // --- chat.token streaming ---

  it('creates streaming placeholder on first chat.token', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.token', { token: 'Hello', messageId: 'api-1' }, 'sess-1');

    expect(result.current.messages.length).toBe(1);
    expect(result.current.messages[0].content).toBe('Hello');
    expect(result.current.messages[0].role).toBe('assistant');
  });

  it('appends tokens to streaming message', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.token', { token: 'Hello', messageId: 'api-1' }, 'sess-1');
    fireWS('chat.token', { token: ' world', messageId: 'api-1' }, 'sess-1');

    expect(result.current.messages[0].content).toBe('Hello world');
  });

  // --- chat.thinking ---

  it('creates thinking placeholder on chat.thinking', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.thinking', { text: 'Let me think...' }, 'sess-1');

    expect(result.current.messages.length).toBe(1);
    expect(result.current.messages[0].thinking).toBe('Let me think...');
  });

  it('appends thinking text', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.thinking', { text: 'Step 1. ' }, 'sess-1');
    fireWS('chat.thinking', { text: 'Step 2.' }, 'sess-1');

    expect(result.current.messages[0].thinking).toBe('Step 1. Step 2.');
  });

  // --- chat.done ---

  it('finalizes streaming message on chat.done', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.token', { token: 'Response', messageId: 'api-1' }, 'sess-1');
    act(() => { /* ensure isStreaming is set */ });

    fireWS('chat.done', { messageId: 'final-id', model: 'claude-3' }, 'sess-1');

    expect(result.current.messages[0].id).toBe('final-id');
    expect(result.current.messages[0].model).toBe('claude-3');
    expect(result.current.isStreaming).toBe(false);
  });

  it('includes usage data on chat.done', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.token', { token: 'Hi', messageId: 'api-1' }, 'sess-1');

    const usage = { inputTokens: 100, outputTokens: 50 };
    fireWS('chat.done', { messageId: 'final', usage }, 'sess-1');

    expect(result.current.messages[0].usage).toEqual(usage);
  });

  // --- chat.error ---

  it('creates error message on chat.error', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.error', { error: 'Rate limit exceeded' }, 'sess-1');

    expect(result.current.messages.length).toBe(1);
    expect(result.current.messages[0].content).toContain('Rate limit exceeded');
    expect(result.current.isStreaming).toBe(false);
  });

  it('replaces streaming placeholder on chat.error', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.token', { token: 'Partial...', messageId: 'api-1' }, 'sess-1');
    fireWS('chat.error', { error: 'Server error' }, 'sess-1');

    expect(result.current.messages.length).toBe(1);
    expect(result.current.messages[0].content).toContain('Server error');
  });

  it('sets authError on authentication error', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.error', { error: 'Authentication failed' }, 'sess-1');

    expect(result.current.authError).toBe(true);
  });

  // --- tool.call / tool.complete / tool.error ---

  it('adds tool call to streaming message', () => {
    const { result } = renderHook(() => useChat());

    // Start with a token to create streaming message
    fireWS('chat.token', { token: 'Using tool...', messageId: 'api-1' }, 'sess-1');
    fireWS('tool.call', { id: 't1', name: 'search', input: { query: 'test' } }, 'sess-1');

    const msg = result.current.messages[0];
    expect(msg.toolCalls).toHaveLength(1);
    expect(msg.toolCalls![0].name).toBe('search');
    expect(msg.toolCalls![0].status).toBe('running');
  });

  it('creates streaming placeholder for tool.call without prior tokens', () => {
    const { result } = renderHook(() => useChat());

    fireWS('tool.call', { id: 't1', name: 'search', input: {} }, 'sess-1');

    expect(result.current.messages.length).toBe(1);
    expect(result.current.messages[0].toolCalls).toHaveLength(1);
  });

  it('completes tool on tool.complete', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.token', { token: '', messageId: 'api-1' }, 'sess-1');
    fireWS('tool.call', { id: 't1', name: 'search', input: {} }, 'sess-1');
    fireWS('tool.complete', { id: 't1', output: 'Found 5 results' }, 'sess-1');

    const tool = result.current.messages[0].toolCalls![0];
    expect(tool.status).toBe('complete');
    expect(tool.output).toBe('Found 5 results');
  });

  it('marks tool as error on tool.error', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.token', { token: '', messageId: 'api-1' }, 'sess-1');
    fireWS('tool.call', { id: 't1', name: 'search', input: {} }, 'sess-1');
    fireWS('tool.error', { id: 't1', output: 'Timeout' }, 'sess-1');

    const tool = result.current.messages[0].toolCalls![0];
    expect(tool.status).toBe('error');
    expect(tool.output).toBe('Timeout');
  });

  // --- tool.progress ---

  it('updates tool progress', () => {
    const { result } = renderHook(() => useChat());

    fireWS('chat.token', { token: '', messageId: 'api-1' }, 'sess-1');
    fireWS('tool.call', { id: 't1', name: 'search', input: {} }, 'sess-1');
    fireWS('tool.progress', { id: 't1', progress: 50, detail: 'Halfway' }, 'sess-1');

    const tool = result.current.messages[0].toolCalls![0];
    expect(tool.progress).toBe(50);
    expect(tool.steps).toHaveLength(1);
    expect(tool.steps![0].detail).toBe('Halfway');
  });

  // --- Task event forwarding ---

  it('forwards task.* events to window CustomEvent', () => {
    renderHook(() => useChat());

    const listener = vi.fn();
    window.addEventListener('ws:task-event', listener);

    fireWS('task.created' as any, { id: 1, title: 'Test' }, 'sess-1');

    expect(listener).toHaveBeenCalled();
    const detail = (listener.mock.calls[0][0] as CustomEvent).detail;
    expect(detail.type).toBe('task.created');
    expect(detail.data.id).toBe(1);

    window.removeEventListener('ws:task-event', listener);
  });

  // --- sendMessage ---

  it('sendMessage adds user message optimistically', () => {
    const { result } = renderHook(() => useChat());

    // Set up a session first
    fireWS('session.created', {
      session: { id: 'sess-1', title: 'Chat', messageCount: 0 },
    }, '');

    act(() => {
      result.current.sendMessage('Hello AI');
    });

    // User message should appear immediately
    const userMsg = result.current.messages.find(m => m.role === 'user');
    expect(userMsg).toBeTruthy();
    expect(userMsg!.content).toBe('Hello AI');
    expect(result.current.isStreaming).toBe(true);
  });

  it('sendMessage ignores empty content', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.created', {
      session: { id: 'sess-1', title: 'Chat', messageCount: 0 },
    }, '');

    act(() => {
      result.current.sendMessage('   ');
    });

    expect(result.current.messages.length).toBe(0);
  });

  it('sendMessage creates session if none exists', () => {
    const { result } = renderHook(() => useChat());

    act(() => {
      result.current.sendMessage('First message');
    });

    // Should create session first
    expect(mockSend).toHaveBeenCalledWith('session.create', {});
  });

  // --- stopGeneration ---

  it('stopGeneration sends chat.stop and clears streaming', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.created', {
      session: { id: 'sess-1', title: 'Chat', messageCount: 0 },
    }, '');

    act(() => {
      result.current.stopGeneration();
    });

    expect(mockSend).toHaveBeenCalledWith('chat.stop', { sessionId: 'sess-1' });
  });

  // --- createSession ---

  it('createSession sends session.create', () => {
    const { result } = renderHook(() => useChat());

    act(() => {
      result.current.createSession();
    });

    expect(mockSend).toHaveBeenCalledWith('session.create', {});
  });

  // --- switchSession ---

  it('switchSession updates state and sends WS message', () => {
    const { result } = renderHook(() => useChat());

    // Set up initial session
    fireWS('session.created', {
      session: { id: 'sess-1', title: 'Chat', messageCount: 0 },
    }, '');

    mockSend.mockClear();

    act(() => {
      result.current.switchSession('sess-2');
    });

    expect(result.current.currentSessionID).toBe('sess-2');
    expect(result.current.messages).toEqual([]);
    expect(mockSend).toHaveBeenCalledWith('session.switch', { sessionId: 'sess-2' });
    expect(localStorage.getItem('soul-v2-session')).toBe('sess-2');
  });

  it('switchSession is noop for current session', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.created', {
      session: { id: 'sess-1', title: 'Chat', messageCount: 0 },
    }, '');

    mockSend.mockClear();

    act(() => {
      result.current.switchSession('sess-1');
    });

    // Should not send again
    expect(mockSend).not.toHaveBeenCalledWith('session.switch', expect.anything());
  });

  // --- deleteSession ---

  it('deleteSession sends WS delete command', () => {
    const { result } = renderHook(() => useChat());

    act(() => {
      result.current.deleteSession('sess-1');
    });

    expect(mockSend).toHaveBeenCalledWith('session.delete', { sessionId: 'sess-1' });
  });

  // --- renameSession ---

  it('renameSession sends WS rename command', () => {
    const { result } = renderHook(() => useChat());

    act(() => {
      result.current.renameSession('sess-1', 'New Title');
    });

    expect(mockSend).toHaveBeenCalledWith('session.rename', { sessionId: 'sess-1', content: 'New Title' });
  });

  // --- setProduct ---

  it('setProduct sends WS command and updates state', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.created', {
      session: { id: 'sess-1', title: 'Chat', messageCount: 0 },
    }, '');

    mockSend.mockClear();

    act(() => {
      result.current.setProduct('tasks');
    });

    expect(mockSend).toHaveBeenCalledWith('session.setProduct', {
      sessionId: 'sess-1',
      product: 'tasks',
    });
    expect(result.current.activeProduct).toBe('tasks');
  });

  it('setProduct is noop without active session', () => {
    const { result } = renderHook(() => useChat());

    mockSend.mockClear();

    act(() => {
      result.current.setProduct('tasks');
    });

    // Should not send when no session
    expect(mockSend).not.toHaveBeenCalledWith('session.setProduct', expect.anything());
  });

  // --- hydrateHistory tool_use/tool_result merging ---

  it('hydrates tool_use + tool_result messages into assistant with toolCalls', () => {
    const { result } = renderHook(() => useChat());

    fireWS('session.created', {
      session: { id: 'sess-1', title: 'Chat', messageCount: 0 },
    }, '');

    const toolBlocks = JSON.stringify([
      { type: 'text', text: 'Let me search...' },
      { type: 'tool_use', id: 'tu-1', name: 'search', input: { q: 'test' } },
    ]);
    const toolResult = JSON.stringify({ tool_use_id: 'tu-1', content: 'Found 3 results' });

    fireWS('session.history', {
      messages: [
        { id: 'm1', role: 'user', content: 'Find something' },
        { id: 'm2', role: 'tool_use', content: toolBlocks },
        { id: 'm3', role: 'tool_result', content: toolResult },
        { id: 'm4', role: 'assistant', content: 'Here are the results.' },
      ],
    }, 'sess-1');

    expect(result.current.messages.length).toBe(3); // user, assistant (with tools), assistant
    const toolMsg = result.current.messages[1];
    expect(toolMsg.role).toBe('assistant');
    expect(toolMsg.content).toBe('Let me search...');
    expect(toolMsg.toolCalls).toHaveLength(1);
    expect(toolMsg.toolCalls![0].name).toBe('search');
    expect(toolMsg.toolCalls![0].output).toBe('Found 3 results');
  });

  // --- Default / unknown message type ---

  it('ignores unknown message types gracefully', () => {
    const { result } = renderHook(() => useChat());

    fireWS('unknown.type' as any, { foo: 'bar' }, 'sess-1');

    expect(result.current.messages.length).toBe(0);
  });
});
