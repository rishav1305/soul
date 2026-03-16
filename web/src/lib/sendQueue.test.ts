import { describe, it, expect, vi } from 'vitest';
import { SendQueue } from './sendQueue';

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
});
