import { useState, useEffect, useCallback } from 'react';
import { useWebSocket } from './useWebSocket.ts';
import type { FindingMessage, WSMessage } from '../lib/types.ts';

export interface ScanResult {
  score: number;
  total: number;
  bySeverity: Record<string, number>;
}

export function useScanResult() {
  const { onMessage } = useWebSocket();
  const [findings, setFindings] = useState<FindingMessage[]>([]);
  const [progress, setProgress] = useState<Record<string, number>>({});
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [isScanning, setIsScanning] = useState(false);

  useEffect(() => {
    const unsubscribe = onMessage((msg: WSMessage) => {
      switch (msg.type) {
        case 'scan.start': {
          setIsScanning(true);
          setFindings([]);
          setProgress({});
          setScanResult(null);
          break;
        }

        case 'tool.progress': {
          const data = msg.data as { analyzer?: string; progress: number };
          if (data.analyzer) {
            setProgress((prev) => ({
              ...prev,
              [data.analyzer as string]: data.progress,
            }));
          }
          break;
        }

        case 'tool.finding': {
          const data = msg.data as { finding: FindingMessage };
          setFindings((prev) => [...prev, data.finding]);
          break;
        }

        case 'tool.complete': {
          const data = msg.data as { output?: string };
          if (data.output) {
            try {
              const parsed = JSON.parse(data.output) as ScanResult;
              setScanResult(parsed);
            } catch {
              // Not a scan result
            }
          }
          setIsScanning(false);
          break;
        }
      }
    });

    return unsubscribe;
  }, [onMessage]);

  const resetScan = useCallback(() => {
    setFindings([]);
    setProgress({});
    setScanResult(null);
    setIsScanning(false);
  }, []);

  return { findings, progress, scanResult, isScanning, resetScan };
}
