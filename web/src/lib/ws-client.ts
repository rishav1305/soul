/**
 * WSClient — simple WebSocket client with handler fan-out and auto-reconnect.
 * Supports async URL factory for ticket-based auth (fresh ticket per connection).
 */
import type { WSMessage } from './types.ts';

type MessageHandler = (msg: WSMessage) => void;
type UrlFactory = () => Promise<string> | string;

export class WSClient {
  private ws: WebSocket | null = null;
  private urlFactory: UrlFactory;
  private handlers: MessageHandler[] = [];
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private shouldReconnect = true;
  private _connected = false;
  private onConnectedChange?: (connected: boolean) => void;

  constructor(urlFactory: UrlFactory, onConnectedChange?: (connected: boolean) => void) {
    this.urlFactory = urlFactory;
    this.onConnectedChange = onConnectedChange;
  }

  get connected(): boolean {
    return this._connected;
  }

  private setConnected(value: boolean) {
    this._connected = value;
    this.onConnectedChange?.(value);
  }

  connect(): void {
    this.shouldReconnect = true;
    this.createConnection();
  }

  private async createConnection(): Promise<void> {
    try {
      const url = await this.urlFactory();
      this.ws = new WebSocket(url);

      this.ws.onopen = () => {
        this.setConnected(true);
        this.reconnectDelay = 1000;
      };

      this.ws.onmessage = (event: MessageEvent) => {
        try {
          const data = event.data as string;
          // Handle batched messages (JSON array frames from WritePump coalescing)
          if (data.startsWith('[')) {
            const msgs = JSON.parse(data) as WSMessage[];
            for (const msg of msgs) {
              for (const handler of this.handlers) handler(msg);
            }
          } else {
            const msg = JSON.parse(data) as WSMessage;
            for (const handler of this.handlers) handler(msg);
          }
        } catch {
          // Ignore malformed messages
        }
      };

      this.ws.onclose = () => {
        this.setConnected(false);
        this.scheduleReconnect();
      };

      this.ws.onerror = () => {
        this.ws?.close();
      };
    } catch {
      this.scheduleReconnect();
    }
  }

  private scheduleReconnect(): void {
    if (!this.shouldReconnect) return;
    if (this.reconnectTimer) return;

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.createConnection();
    }, this.reconnectDelay);

    this.reconnectDelay = Math.min(
      this.reconnectDelay * 2,
      this.maxReconnectDelay,
    );
  }

  send(msg: WSMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  onMessage(handler: MessageHandler): () => void {
    this.handlers.push(handler);
    return () => {
      this.handlers = this.handlers.filter((h) => h !== handler);
    };
  }

  disconnect(): void {
    this.shouldReconnect = false;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
    this.setConnected(false);
  }
}
