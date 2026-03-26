import type { ThinkingType } from '../lib/types';

const THINKING_STATES: { type: ThinkingType; label: string; className: string }[] = [
  { type: 'disabled', label: 'Off', className: 'bg-zinc-700 text-zinc-400' },
  { type: 'adaptive', label: 'Auto', className: 'bg-blue-500/20 text-blue-400' },
  { type: 'enabled', label: 'Max', className: 'bg-amber-500/20 text-amber-400' },
];

interface ThinkingToggleProps {
  value: ThinkingType;
  onChange: (value: ThinkingType) => void;
}

export function ThinkingToggle({ value, onChange }: ThinkingToggleProps) {
  const currentIndex = THINKING_STATES.findIndex(s => s.type === value);
  const safeIndex = currentIndex >= 0 ? currentIndex : 0;
  const current = THINKING_STATES[safeIndex] ?? THINKING_STATES[0]!;

  const cycle = () => {
    const nextIndex = (safeIndex + 1) % THINKING_STATES.length;
    const next = THINKING_STATES[nextIndex];
    if (next) onChange(next.type);
  };

  return (
    <button
      onClick={cycle}
      className={`flex items-center gap-1 px-2 py-1 text-xs rounded transition-colors ${current.className}`}
      title={`Thinking: ${current.label} (click to cycle)`}
      data-testid="thinking-toggle"
    >
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" aria-hidden="true">
        <path d="M8 2a4.5 4.5 0 0 1 2.5 8.2V12h-5v-1.8A4.5 4.5 0 0 1 8 2z" />
        <path d="M6 14h4" />
      </svg>
      <span className="hidden sm:inline">Think ·</span>
      <span>{current.label}</span>
    </button>
  );
}
