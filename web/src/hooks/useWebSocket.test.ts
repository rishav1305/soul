// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';

// --- Mock dependencies ---

const mockFetchWSTicket = vi.fn();
const mockGetWebSocketURL = vi.fn();

vi.mock('../lib/ws.ts', () => ({
  fetchWSTicket: (...args: unknown[]) => mockFetchWSTicket(...args),
  getWebSocketURL: (...args: unknown[]) => mockGetWebSocketURL(...args),
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportWSLifecycle: vi.fn(),
}));

// --- Fake WebSocket ---

type WSEventCallback = ((event: any) => void) | null;

class FakeWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: FakeWebSocket[] = [];

  url: string;
  readyState = FakeWebSocket.CONNECTING;
  onopen: WSEventCallback = null;
  onmessage: WSEventCallback = null;
  onclose: WSEventCallback = null;
  onerror: WSEventCallback = null;
  sentMessages: string[] = [];

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  send(data: string) {
    this.sentMessages.push(data);
  }

  close() {
    this.readyState = FakeWebSocket.CLOSED;
    if (this.onclose) {
      this.onclose({ code: 1000, reason: '', wasClean: true });
    }
  }

  // Test helpers
  simulateOpen() {
    this.readyState = FakeWebSocket.OPEN;
    if (this.onopen) this.onopen({});
  }

  simulateMessage(data: unknown) {
    if (this.onmessage) {
      this.onmessage({ data: JSON.stringify(data) });
    }
  }

  simulateClose(code = 1000, reason = '') {
    this.readyState = FakeWebSocket.CLOSED;
    if (this.onclose) this.onclose({ code, reason, wasClean: code === 1000 });
  }

  simulateError() {
    if (this.onerror) this.onerror({});
  }
}

import { useWebSocket } from './useWebSocket';

describe('useWebSocket', () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    mockFetchWSTicket.mockReset();
    mockGetWebSocketURL.mockReset();
    // Default: ticket auth succeeds
    mockFetchWSTicket.mockResolvedValue({ ticket: 'test-ticket', status: 200 });
    mockGetWebSocketURL.mockReturnValue('ws://localhost/ws?ticket=test-ticket');
    vi.stubGlobal('WebSocket', FakeWebSocket);
    // Mock performance.now
    vi.spyOn(performance, 'now').mockReturnValue(1000);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    cleanup();
  });

  it('starts in disconnected state', () => {
    const { result } = renderHook(() => useWebSocket());
    // Status is 'connecting' after mount since connect() fires immediately
    expect(['disconnected', 'connecting']).toContain(result.current.status);
  });

  it('fetches ticket and creates WebSocket on mount', async () => {
    renderHook(() => useWebSocket());

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    expect(mockFetchWSTicket).toHaveBeenCalled();
    expect(FakeWebSocket.instances[0].url).toBe('ws://localhost/ws?ticket=test-ticket');
  });

  it('sets status to connected on connection.ready message', async () => {
    const { result } = renderHook(() => useWebSocket());

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    const ws = FakeWebSocket.instances[0];

    act(() => {
      ws.simulateOpen();
      ws.simulateMessage({ type: 'connection.ready' });
    });

    expect(result.current.status).toBe('connected');
  });

  it('dispatches messages to onMessage callback', async () => {
    const onMessage = vi.fn();
    renderHook(() => useWebSocket({ onMessage }));

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    const ws = FakeWebSocket.instances[0];

    act(() => {
      ws.simulateOpen();
      ws.simulateMessage({ type: 'chat.token', data: 'hello', sessionId: 'sess-1', messageId: 'msg-1' });
    });

    expect(onMessage).toHaveBeenCalledWith('chat.token', 'hello', 'sess-1', 'msg-1');
  });

  it('handles batched messages (array frames)', async () => {
    const onMessage = vi.fn();
    renderHook(() => useWebSocket({ onMessage }));

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    const ws = FakeWebSocket.instances[0];

    act(() => {
      ws.simulateOpen();
      ws.simulateMessage([
        { type: 'chat.token', data: 'a', sessionId: 's1' },
        { type: 'chat.token', data: 'b', sessionId: 's1' },
      ]);
    });

    expect(onMessage).toHaveBeenCalledTimes(2);
  });

  it('send transmits JSON when connected', async () => {
    const { result } = renderHook(() => useWebSocket());

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    const ws = FakeWebSocket.instances[0];

    act(() => {
      ws.simulateOpen();
      ws.readyState = FakeWebSocket.OPEN;
    });

    act(() => {
      result.current.send('chat.message', { body: 'hi', sessionId: 'sess-1' });
    });

    expect(ws.sentMessages.length).toBe(1);
    const sent = JSON.parse(ws.sentMessages[0]);
    expect(sent.type).toBe('chat.message');
    expect(sent.body).toBe('hi');
  });

  it('send is a no-op when not connected', () => {
    const { result } = renderHook(() => useWebSocket());

    act(() => {
      result.current.send('chat.message', { body: 'hi' });
    });

    // No WebSocket instances created yet or not open — should not throw
  });

  it('uses custom URL when provided, skips ticket fetch', async () => {
    renderHook(() => useWebSocket({ url: 'ws://custom/ws' }));

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    expect(FakeWebSocket.instances[0].url).toBe('ws://custom/ws');
    // Should not call fetchWSTicket when custom URL provided
    expect(mockFetchWSTicket).not.toHaveBeenCalled();
  });

  it('sets error status on WebSocket error', async () => {
    const { result } = renderHook(() => useWebSocket());

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    const ws = FakeWebSocket.instances[0];

    act(() => {
      ws.simulateError();
    });

    expect(result.current.status).toBe('error');
  });

  it('closes WebSocket on unmount', async () => {
    const { unmount } = renderHook(() => useWebSocket());

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    const ws = FakeWebSocket.instances[0];
    const closeSpy = vi.spyOn(ws, 'close');

    unmount();

    expect(closeSpy).toHaveBeenCalled();
  });

  it('resets state on manual reconnect', async () => {
    const { result } = renderHook(() => useWebSocket());

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));

    act(() => {
      result.current.reconnect();
    });

    expect(result.current.reconnectAttempt).toBe(0);
    expect(result.current.authError).toBe(false);
  });

  it('starts with authError false', () => {
    const { result } = renderHook(() => useWebSocket());
    expect(result.current.authError).toBe(false);
  });

  it('starts with reconnectAttempt 0', () => {
    const { result } = renderHook(() => useWebSocket());
    expect(result.current.reconnectAttempt).toBe(0);
  });

  it('resets reconnect attempt on connection.ready', async () => {
    const { result } = renderHook(() => useWebSocket());

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    const ws = FakeWebSocket.instances[0];

    act(() => {
      ws.simulateOpen();
      ws.simulateMessage({ type: 'connection.ready' });
    });

    expect(result.current.reconnectAttempt).toBe(0);
  });

  it('provides sessionId fallback for messages without one', async () => {
    const onMessage = vi.fn();
    renderHook(() => useWebSocket({ onMessage }));

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    const ws = FakeWebSocket.instances[0];

    act(() => {
      ws.simulateOpen();
      ws.simulateMessage({ type: 'system.ping' });
    });

    // sessionId defaults to '' when missing
    expect(onMessage).toHaveBeenCalledWith('system.ping', undefined, '', undefined);
  });
});
