interface QueuedMessage {
  id: string;
  payload: Record<string, unknown>;
  enqueuedAt: number;
  sent: boolean;
}

let counter = 0;

/**
 * Offline-resilient message queue with localStorage persistence.
 * Messages are buffered until flushed via a sender callback.
 * Failed sends are retained for retry; successful sends are removed.
 */
export class SendQueue {
  private messages: QueuedMessage[] = [];
  private storageKey: string;

  constructor(storageKey = 'soul-v2-send-queue') {
    this.storageKey = storageKey;
  }

  /** Add a message to the queue. Returns a unique message ID. */
  enqueue(payload: Record<string, unknown>): string {
    const id = `msg-${Date.now()}-${++counter}`;
    this.messages.push({
      id,
      payload: { ...payload, messageId: id },
      enqueuedAt: Date.now(),
      sent: false,
    });
    return id;
  }

  /** Send all pending messages via the provided callback. Throws on first failure — unsent messages are retained. */
  flush(sender: (payload: Record<string, unknown>) => void): void {
    const pending = this.messages.filter((m) => !m.sent);
    for (const msg of pending) {
      sender(msg.payload); // throws on failure → message stays unsent
      msg.sent = true;
    }
    this.messages = this.messages.filter((m) => !m.sent);
  }

  /** Mark a specific message as sent (e.g. when confirmed by server ACK). */
  markSent(id: string): void {
    const msg = this.messages.find((m) => m.id === id);
    if (msg) msg.sent = true;
  }

  /** Number of unsent messages currently queued. */
  pending(): number {
    return this.messages.filter((m) => !m.sent).length;
  }

  /** Save pending messages to localStorage for recovery across page reloads. */
  persist(): void {
    try {
      const pending = this.messages.filter((m) => !m.sent);
      localStorage.setItem(this.storageKey, JSON.stringify(pending));
    } catch {
      // localStorage may be unavailable
    }
  }

  /** Restore previously persisted messages from localStorage. Removes the stored data after loading. */
  restore(): void {
    try {
      const raw = localStorage.getItem(this.storageKey);
      if (raw) {
        this.messages = JSON.parse(raw);
        localStorage.removeItem(this.storageKey);
      }
    } catch {
      // corrupted data — ignore
    }
  }

  /** Discard all queued messages and remove localStorage backup. */
  clear(): void {
    this.messages = [];
    try { localStorage.removeItem(this.storageKey); } catch { /* ignore */ }
  }
}
