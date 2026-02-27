interface ScanProgressProps {
  progress: Record<string, number>;
}

const ANALYZERS = [
  'license',
  'secrets',
  'sast',
  'dependency',
  'iac',
];

export default function ScanProgress({ progress }: ScanProgressProps) {
  return (
    <div className="space-y-3 p-4">
      <h3 className="text-xs font-semibold uppercase tracking-wider text-zinc-500">
        Scan Progress
      </h3>
      {ANALYZERS.map((analyzer) => {
        const pct = progress[analyzer] ?? 0;
        return (
          <div key={analyzer} className="space-y-1">
            <div className="flex justify-between text-xs">
              <span className="text-zinc-400 capitalize">{analyzer}</span>
              <span className="text-zinc-500">{Math.round(pct)}%</span>
            </div>
            <div className="h-1.5 bg-zinc-800 rounded-full overflow-hidden">
              <div
                className="h-full bg-sky-500 rounded-full transition-all duration-500 ease-out"
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
