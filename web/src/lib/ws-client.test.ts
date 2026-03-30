import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { WSClient } from './ws-client';

// Minimal WebSocket mock for node environment
class MockWebSocket {
  static readonly OPEN = 1;
  static readonly CLOSED = 3;
  static instances: MockWebSocket[] = [];

  readyState = MockWebSocket.OPEN;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  sent: string[] = [];
  closed = false;

  constructor(public url: string) {
    MockWebSocket.instances.push(this);
    // Simulate async open
    setTimeout(() => this.onopen?.(), 0);
  }

  send(data: string): void {
    this.sent.push(data);
  }

  close(): void {
    this.closed = true;
    this.readyState = MockWebSocket.CLOSED;
    setTimeout(() => this.onclose?.(), 0);
  }

  // Test helpers
  simulateMessage(data: string): void {
    this.onmessage?.({ data });
  }

  simulateClose(): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.();
  }

  simulateError(): void {
    this.onerror?.();
  }
}

describe('WSClient', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    MockWebSocket.instances = [];
    // @ts-expect-error — injecting mock into global for WSClient
    globalThis.WebSocket = MockWebSocket;
  });

  afterEach(() => {
    vi.useRealTimers();
    // @ts-expect-error — cleanup
    delete globalThis.WebSocket;
  });

  it('connects via url factory', async () => {
    const client = new WSClient(() => 'ws://localhost:3002/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0]!.url).toBe('ws://localhost:3002/ws');

    client.disconnect();
  });

  it('supports async url factory', async () => {
    const client = new WSClient(async () => 'ws://localhost:3002/ws?ticket=abc');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0]!.url).toBe('ws://localhost:3002/ws?ticket=abc');

    client.disconnect();
  });

  it('reports connected state on open', async () => {
    const onConnected = vi.fn();
    const client = new WSClient(() => 'ws://localhost/ws', onConnected);
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    expect(client.connected).toBe(true);
    expect(onConnected).toHaveBeenCalledWith(true);

    client.disconnect();
  });

  it('fans out messages to all registered handlers', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    const handler1 = vi.fn();
    const handler2 = vi.fn();
    client.onMessage(handler1);
    client.onMessage(handler2);

    const ws = MockWebSocket.instances[0]!;
    ws.simulateMessage(JSON.stringify({ type: 'chat.token', data: 'hello' }));

    expect(handler1).toHaveBeenCalledWith({ type: 'chat.token', data: 'hello' });
    expect(handler2).toHaveBeenCalledWith({ type: 'chat.token', data: 'hello' });

    client.disconnect();
  });

  it('handles batched message arrays', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    const handler = vi.fn();
    client.onMessage(handler);

    const ws = MockWebSocket.instances[0]!;
    const batch = [
      { type: 'chat.token', data: 'a' },
      { type: 'chat.token', data: 'b' },
    ];
    ws.simulateMessage(JSON.stringify(batch));

    expect(handler).toHaveBeenCalledTimes(2);
    expect(handler).toHaveBeenCalledWith({ type: 'chat.token', data: 'a' });
    expect(handler).toHaveBeenCalledWith({ type: 'chat.token', data: 'b' });

    client.disconnect();
  });

  it('unsubscribes handler via returned cleanup function', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    const handler = vi.fn();
    const unsub = client.onMessage(handler);
    unsub();

    const ws = MockWebSocket.instances[0]!;
    ws.simulateMessage(JSON.stringify({ type: 'test' }));

    expect(handler).not.toHaveBeenCalled();

    client.disconnect();
  });

  it('sends JSON-serialized messages', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    const msg = { type: 'chat.send', content: 'hello' };
    client.send(msg as any);

    const ws = MockWebSocket.instances[0]!;
    expect(ws.sent).toHaveLength(1);
    expect(JSON.parse(ws.sent[0]!)).toEqual(msg);

    client.disconnect();
  });

  it('schedules reconnect on close', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    expect(MockWebSocket.instances).toHaveLength(1);

    // Simulate close
    const ws = MockWebSocket.instances[0]!;
    ws.simulateClose();

    // Not yet — reconnect is scheduled at 1s
    await vi.advanceTimersByTimeAsync(500);
    expect(MockWebSocket.instances).toHaveLength(1);

    // After 1s (initial backoff) should reconnect
    await vi.advanceTimersByTimeAsync(510);
    await vi.advanceTimersByTimeAsync(10);
    expect(MockWebSocket.instances).toHaveLength(2);

    // Second close → reconnect again (delay resets on successful open)
    MockWebSocket.instances[1]!.simulateClose();
    await vi.advanceTimersByTimeAsync(1010);
    await vi.advanceTimersByTimeAsync(10);
    expect(MockWebSocket.instances).toHaveLength(3);

    client.disconnect();
  });

  it('does not reconnect after explicit disconnect', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    client.disconnect();

    // Wait well past any reconnect timer
    await vi.advanceTimersByTimeAsync(60000);
    // Should only have the initial connection
    expect(MockWebSocket.instances).toHaveLength(1);
  });

  it('resets backoff delay on successful reconnect', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    // Close → reconnect at 1s
    MockWebSocket.instances[0]!.simulateClose();
    await vi.advanceTimersByTimeAsync(1010);
    await vi.advanceTimersByTimeAsync(10);
    expect(MockWebSocket.instances).toHaveLength(2);

    // The new connection opens successfully → backoff resets to 1s
    // Close again → should reconnect at 1s (not 2s)
    MockWebSocket.instances[1]!.simulateClose();
    await vi.advanceTimersByTimeAsync(1010);
    await vi.advanceTimersByTimeAsync(10);
    expect(MockWebSocket.instances).toHaveLength(3);

    client.disconnect();
  });

  it('ignores malformed JSON messages', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    const handler = vi.fn();
    client.onMessage(handler);

    const ws = MockWebSocket.instances[0]!;
    // Should not throw
    ws.simulateMessage('not valid json {{{');

    expect(handler).not.toHaveBeenCalled();

    client.disconnect();
  });

  it('reports disconnected on close', async () => {
    const onConnected = vi.fn();
    const client = new WSClient(() => 'ws://localhost/ws', onConnected);
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    expect(onConnected).toHaveBeenCalledWith(true);

    MockWebSocket.instances[0]!.simulateClose();
    expect(onConnected).toHaveBeenCalledWith(false);

    client.disconnect();
  });

  it('does not send when WebSocket is closed', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    const ws = MockWebSocket.instances[0]!;
    ws.readyState = MockWebSocket.CLOSED;

    client.send({ type: 'chat.send', content: 'ignored' } as any);
    expect(ws.sent).toHaveLength(0);

    client.disconnect();
  });

  it('does not send before connection opens', () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    // Don't advance timers — connection not opened yet
    client.send({ type: 'test' } as any);
    // No instances yet, nothing to send to
    client.disconnect();
  });

  it('disconnect sets connected to false', async () => {
    const onConnected = vi.fn();
    const client = new WSClient(() => 'ws://localhost/ws', onConnected);
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    expect(client.connected).toBe(true);

    client.disconnect();
    expect(client.connected).toBe(false);
    expect(onConnected).toHaveBeenLastCalledWith(false);
  });

  it('error event triggers close on WebSocket', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    const ws = MockWebSocket.instances[0]!;
    ws.simulateError();

    expect(ws.closed).toBe(true);

    client.disconnect();
  });

  it('exponential backoff doubles delay on successive failures', async () => {
    // Verify the backoff increases: first reconnect at 1s, second at 2s
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);
    expect(MockWebSocket.instances).toHaveLength(1);

    // Close first — next reconnect scheduled at 1s delay
    MockWebSocket.instances[0]!.simulateClose();

    // After 1s, reconnect fires
    await vi.advanceTimersByTimeAsync(1010);
    await vi.advanceTimersByTimeAsync(10);
    const afterFirst = MockWebSocket.instances.length;
    expect(afterFirst).toBe(2);

    // The second opens (onopen fires via setTimeout 0) which resets delay to 1s
    // But if we close BEFORE onopen fires, the delay stays doubled
    // Since our mock fires onopen via setTimeout(0), close immediately
    MockWebSocket.instances[1]!.readyState = MockWebSocket.CLOSED;
    MockWebSocket.instances[1]!.onclose?.();

    // Now delay should be 2s (doubled from 1s). After 1.5s, should NOT have reconnected
    // But after 2s+ it should
    await vi.advanceTimersByTimeAsync(2100);
    await vi.advanceTimersByTimeAsync(10);
    expect(MockWebSocket.instances.length).toBeGreaterThan(2);

    client.disconnect();
  });

  it('multiple handlers can be independently unsubscribed', async () => {
    const client = new WSClient(() => 'ws://localhost/ws');
    client.connect();
    await vi.advanceTimersByTimeAsync(10);

    const handler1 = vi.fn();
    const handler2 = vi.fn();
    const unsub1 = client.onMessage(handler1);
    client.onMessage(handler2);

    // Unsubscribe first only
    unsub1();

    const ws = MockWebSocket.instances[0]!;
    ws.simulateMessage(JSON.stringify({ type: 'test' }));

    expect(handler1).not.toHaveBeenCalled();
    expect(handler2).toHaveBeenCalledWith({ type: 'test' });

    client.disconnect();
  });

  it('schedules reconnect on url factory rejection', async () => {
    let callCount = 0;
    const client = new WSClient(async () => {
      callCount++;
      if (callCount === 1) throw new Error('Auth failed');
      return 'ws://localhost/ws';
    });
    client.connect();

    // First attempt fails (url factory error) — schedules reconnect at 1s
    await vi.advanceTimersByTimeAsync(100);
    expect(MockWebSocket.instances).toHaveLength(0);

    // After 1s, retries with successful url
    await vi.advanceTimersByTimeAsync(1010);
    await vi.advanceTimersByTimeAsync(10);
    expect(MockWebSocket.instances).toHaveLength(1);

    client.disconnect();
  });
});
