type TelemetryEvent = 'frontend.error' | 'frontend.render' | 'frontend.ws' | 'frontend.usage';

function sendTelemetry(type: TelemetryEvent, data: Record<string, unknown>): void {
  try {
    fetch('/api/telemetry', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
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
