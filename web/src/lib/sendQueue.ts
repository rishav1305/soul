interface QueuedMessage {
  id: string;
  payload: Record<string, unknown>;
  enqueuedAt: number;
  sent: boolean;
}

let counter = 0;

export class SendQueue {
  private messages: QueuedMessage[] = [];
  private storageKey: string;

  constructor(storageKey = 'soul-v2-send-queue') {
    this.storageKey = storageKey;
  }

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

  flush(sender: (payload: Record<string, unknown>) => void): void {
    const pending = this.messages.filter((m) => !m.sent);
    for (const msg of pending) {
      sender(msg.payload); // throws on failure → message stays unsent
      msg.sent = true;
    }
    this.messages = this.messages.filter((m) => !m.sent);
  }

  markSent(id: string): void {
    const msg = this.messages.find((m) => m.id === id);
    if (msg) msg.sent = true;
  }

  pending(): number {
    return this.messages.filter((m) => !m.sent).length;
  }

  persist(): void {
    try {
      const pending = this.messages.filter((m) => !m.sent);
      localStorage.setItem(this.storageKey, JSON.stringify(pending));
    } catch {
      // localStorage may be unavailable
    }
  }

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

  clear(): void {
    this.messages = [];
    try { localStorage.removeItem(this.storageKey); } catch { /* ignore */ }
  }
}
