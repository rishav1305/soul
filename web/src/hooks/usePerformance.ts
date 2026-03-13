import { useEffect, useRef } from 'react';
import { reportRender } from '../lib/telemetry';

export function usePerformance(componentName: string, thresholdMs = 50): void {
  const renderStart = useRef(performance.now());
  renderStart.current = performance.now();

  useEffect(() => {
    const duration = performance.now() - renderStart.current;
    if (duration > thresholdMs) {
      reportRender(componentName, Math.round(duration));
    }
  });
}
