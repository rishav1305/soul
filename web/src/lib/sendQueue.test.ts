import { describe, it, expect, vi, beforeEach } from 'vitest';
import { SendQueue } from './sendQueue';

// Minimal localStorage mock for node environment
function createLocalStorageMock(): Storage {
  const store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => { store[key] = value; },
    removeItem: (key: string) => { delete store[key]; },
    clear: () => { for (const k of Object.keys(store)) delete store[k]; },
    get length() { return Object.keys(store).length; },
    key: (i: number) => Object.keys(store)[i] ?? null,
  };
}

describe('SendQueue', () => {
  it('generates idempotent message IDs', () => {
    const queue = new SendQueue();
    const id1 = queue.enqueue({ type: 'chat.send', content: 'hello' });
    const id2 = queue.enqueue({ type: 'chat.send', content: 'hello' });
    expect(id1).not.toBe(id2);
    expect(id1).toMatch(/^msg-/);
  });

  it('flushes pending messages via sender', () => {
    const queue = new SendQueue();
    const sender = vi.fn();
    queue.enqueue({ type: 'chat.send', content: 'hello' });
    queue.enqueue({ type: 'chat.send', content: 'world' });
    queue.flush(sender);
    expect(sender).toHaveBeenCalledTimes(2);
  });

  it('marks messages as sent after flush', () => {
    const queue = new SendQueue();
    const sender = vi.fn();
    queue.enqueue({ type: 'chat.send', content: 'hello' });
    queue.flush(sender);
    expect(queue.pending()).toBe(0);
  });

  it('retains messages if sender throws', () => {
    const queue = new SendQueue();
    const sender = vi.fn().mockImplementation(() => { throw new Error('offline'); });
    queue.enqueue({ type: 'chat.send', content: 'hello' });
    try { queue.flush(sender); } catch { /* expected */ }
    expect(queue.pending()).toBe(1);
  });

  it('deduplicates by message ID on markSent', () => {
    const queue = new SendQueue();
    const sender = vi.fn();
    const id = queue.enqueue({ type: 'chat.send', content: 'hello' });
    queue.markSent(id);
    queue.flush(sender);
    expect(sender).not.toHaveBeenCalled();
  });

  it('markSent is a no-op for unknown IDs', () => {
    const queue = new SendQueue();
    queue.enqueue({ type: 'test' });
    queue.markSent('nonexistent-id');
    expect(queue.pending()).toBe(1);
  });

  it('includes messageId in flushed payload', () => {
    const queue = new SendQueue();
    const sender = vi.fn();
    const id = queue.enqueue({ type: 'chat.send', content: 'hi' });
    queue.flush(sender);
    expect(sender).toHaveBeenCalledWith(
      expect.objectContaining({ messageId: id }),
    );
  });

  it('reports pending count accurately', () => {
    const queue = new SendQueue();
    expect(queue.pending()).toBe(0);
    queue.enqueue({ type: 'a' });
    queue.enqueue({ type: 'b' });
    expect(queue.pending()).toBe(2);
    queue.flush(vi.fn());
    expect(queue.pending()).toBe(0);
  });
});

describe('SendQueue persistence', () => {
  let mockStorage: Storage;

  beforeEach(() => {
    mockStorage = createLocalStorageMock();
    // @ts-expect-error — inject mock localStorage for testing
    globalThis.localStorage = mockStorage;
  });

  it('persists pending messages to localStorage', () => {
    const queue = new SendQueue('test-queue');
    queue.enqueue({ type: 'chat.send', content: 'unsent' });
    queue.persist();

    const stored = mockStorage.getItem('test-queue');
    expect(stored).not.toBeNull();
    const parsed = JSON.parse(stored!);
    expect(parsed).toHaveLength(1);
    expect(parsed[0].payload.content).toBe('unsent');
  });

  it('does not persist already-sent messages', () => {
    const queue = new SendQueue('test-queue');
    queue.enqueue({ type: 'chat.send', content: 'will-send' });
    queue.flush(vi.fn());
    queue.persist();

    const stored = mockStorage.getItem('test-queue');
    // After flush + persist, only empty array remains
    expect(JSON.parse(stored ?? '[]')).toHaveLength(0);
  });

  it('restores messages from localStorage', () => {
    // Pre-seed localStorage with a queued message
    const payload = [{
      id: 'msg-12345-1',
      payload: { type: 'chat.send', content: 'restored', messageId: 'msg-12345-1' },
      enqueuedAt: Date.now(),
      sent: false,
    }];
    mockStorage.setItem('test-queue', JSON.stringify(payload));

    const queue = new SendQueue('test-queue');
    queue.restore();
    expect(queue.pending()).toBe(1);

    // Should be flushable
    const sender = vi.fn();
    queue.flush(sender);
    expect(sender).toHaveBeenCalledTimes(1);
    expect(sender).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'restored' }),
    );
  });

  it('removes localStorage key after restore', () => {
    mockStorage.setItem('test-queue', JSON.stringify([]));
    const queue = new SendQueue('test-queue');
    queue.restore();
    expect(mockStorage.getItem('test-queue')).toBeNull();
  });

  it('clear() empties queue and localStorage', () => {
    const queue = new SendQueue('test-queue');
    queue.enqueue({ type: 'test' });
    queue.persist();
    expect(mockStorage.getItem('test-queue')).not.toBeNull();

    queue.clear();
    expect(queue.pending()).toBe(0);
    expect(mockStorage.getItem('test-queue')).toBeNull();
  });

  it('handles corrupted localStorage data gracefully', () => {
    mockStorage.setItem('test-queue', 'not valid json!!!');
    const queue = new SendQueue('test-queue');
    // Should not throw
    queue.restore();
    expect(queue.pending()).toBe(0);
  });

  it('handles missing localStorage gracefully on persist', () => {
    // @ts-expect-error — simulate unavailable localStorage
    delete globalThis.localStorage;
    const queue = new SendQueue('test-queue');
    queue.enqueue({ type: 'test' });
    // Should not throw
    queue.persist();
  });
});
