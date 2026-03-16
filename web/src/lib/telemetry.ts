import { getToken } from '../components/AuthGate';

type TelemetryEvent =
  | 'frontend.error'
  | 'frontend.render'
  | 'frontend.ws'
  | 'frontend.usage'
  | 'frontend.ws.disconnect'
  | 'frontend.ws.reconnect'
  | 'frontend.auth.fail';

function sendTelemetry(type: TelemetryEvent, data: Record<string, unknown>): void {
  try {
    const token = getToken()?.trim();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    fetch('/api/telemetry', {
      method: 'POST',
      headers,
      body: JSON.stringify({ type, data }),
    }).catch(() => {
      // Telemetry failure is non-critical — silently ignore.
    });
  } catch {
    // Ignore — telemetry must never throw.
  }
}

export function reportError(component: string, error: unknown): void {
  const message = error instanceof Error ? error.message : String(error);
  const stack = error instanceof Error ? error.stack : undefined;
  sendTelemetry('frontend.error', { component, error: message, stack });
}

export function reportRender(component: string, durationMs: number): void {
  sendTelemetry('frontend.render', { component, duration_ms: durationMs });
}

export function reportWSLatency(firstTokenMs: number, totalMs: number): void {
  sendTelemetry('frontend.ws', { event: 'round_trip', first_token_ms: firstTokenMs, total_ms: totalMs });
}

export function reportUsage(action: string, data?: Record<string, unknown>): void {
  sendTelemetry('frontend.usage', { action, ...data });
}

export function reportDisconnect(data: {
  closeCode: number;
  reasonClass: string;
  connectionDurationMs?: number;
}): void {
  sendTelemetry('frontend.ws.disconnect', data);
}

export function reportReconnect(data: {
  attempt: number;
  backoffMs: number;
  success: boolean;
  totalDowntimeMs?: number;
}): void {
  sendTelemetry('frontend.ws.reconnect', data);
}

export function reportAuthFailure(data: {
  source: 'ws' | 'api';
  reason: string;
}): void {
  sendTelemetry('frontend.auth.fail', data);
}

// --- WS lifecycle telemetry batching ---
// Accumulate WS lifecycle events and flush every 5s or on page unload.
// This avoids hitting the 60 RPM per-IP rate limit during reconnect storms.

interface TelemetryEntry {
  type: TelemetryEvent;
  data: Record<string, unknown>;
}

const lifecycleBatch: TelemetryEntry[] = [];
let flushTimer: ReturnType<typeof setTimeout> | null = null;

function flushLifecycleBatch(): void {
  if (lifecycleBatch.length === 0) return;
  const entries = lifecycleBatch.splice(0);
  const token = getToken()?.trim();
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  fetch('/api/telemetry', {
    method: 'POST',
    headers,
    body: JSON.stringify({ batch: entries }),
    keepalive: true,
  }).catch(() => {});
}

function scheduleFlush(): void {
  if (flushTimer !== null) return;
  flushTimer = setTimeout(() => {
    flushTimer = null;
    flushLifecycleBatch();
  }, 5000);
}

if (typeof window !== 'undefined') {
  window.addEventListener('unload', () => {
    if (flushTimer !== null) {
      clearTimeout(flushTimer);
      flushTimer = null;
    }
    flushLifecycleBatch();
  });
}

/**
 * Reports a WS lifecycle event. Events are batched and flushed every 5s
 * or on page unload (fire-and-forget, keepalive).
 */
export function reportWSLifecycle(event: string, data: Record<string, unknown>): void {
  try {
    lifecycleBatch.push({ type: 'frontend.ws', data: { event, ...data } });
    scheduleFlush();
  } catch {
    // Telemetry must never throw.
  }
}
