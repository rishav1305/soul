export interface TelemetryEvent {
  event: string;
  properties?: Record<string, unknown>;
}

export class Telemetry {
  private enabled = false;
  enable(): void { this.enabled = true; }
  disable(): void { this.enabled = false; }
  track(_event: TelemetryEvent): void {
    if (!this.enabled) return;
  }
}
