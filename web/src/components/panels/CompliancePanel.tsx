import type { ScanResult } from '../../hooks/useScanResult.ts';
import type { FindingMessage } from '../../lib/types.ts';
import FindingsTable from './FindingsTable.tsx';
import ScanProgress from './ScanProgress.tsx';

interface CompliancePanelProps {
  findings?: FindingMessage[];
  progress?: Record<string, number>;
  scanResult?: ScanResult | null;
  isScanning?: boolean;
}

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'text-red-400',
  high: 'text-red-400',
  medium: 'text-amber-400',
  low: 'text-sky-400',
  info: 'text-zinc-400',
};

const SEVERITY_DOT_COLORS: Record<string, string> = {
  critical: 'bg-red-500',
  high: 'bg-red-400',
  medium: 'bg-amber-500',
  low: 'bg-sky-500',
  info: 'bg-zinc-500',
};

function scoreColor(score: number): string {
  if (score >= 80) return 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30';
  if (score >= 60) return 'bg-amber-500/20 text-amber-400 border-amber-500/30';
  return 'bg-red-500/20 text-red-400 border-red-500/30';
}

export default function CompliancePanel({
  findings = [],
  progress = {},
  scanResult = null,
  isScanning = false,
}: CompliancePanelProps) {
  const severityCounts = findings.reduce<Record<string, number>>((acc, f) => {
    const sev = f.severity.toLowerCase();
    acc[sev] = (acc[sev] ?? 0) + 1;
    return acc;
  }, {});

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800">
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-semibold text-zinc-200">Compliance</h2>
          {scanResult && (
            <span
              className={`px-2 py-0.5 rounded-full text-xs font-medium border ${scoreColor(scanResult.score)}`}
            >
              {scanResult.score}
            </span>
          )}
        </div>
      </div>

      {/* Severity summary */}
      {findings.length > 0 && (
        <div className="flex gap-3 px-4 py-3 border-b border-zinc-800">
          {(['critical', 'high', 'medium', 'low', 'info'] as const).map(
            (sev) => (
              <div key={sev} className="flex items-center gap-1.5">
                <span
                  className={`w-2 h-2 rounded-full ${SEVERITY_DOT_COLORS[sev]}`}
                />
                <span
                  className={`text-xs font-medium ${SEVERITY_COLORS[sev]}`}
                >
                  {severityCounts[sev] ?? 0}
                </span>
              </div>
            ),
          )}
        </div>
      )}

      {/* Content area */}
      <div className="flex-1 overflow-y-auto">
        {isScanning && <ScanProgress progress={progress} />}
        <FindingsTable findings={findings} />
      </div>
    </div>
  );
}
