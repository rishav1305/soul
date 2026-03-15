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
  const current = THINKING_STATES[currentIndex >= 0 ? currentIndex : 0];

  const cycle = () => {
    const nextIndex = (currentIndex + 1) % THINKING_STATES.length;
    onChange(THINKING_STATES[nextIndex].type);
  };

  return (
    <button
      onClick={cycle}
      className={`flex items-center gap-1 px-2 py-1 text-xs rounded transition-colors ${current.className}`}
      title={`Thinking: ${current.label} (click to cycle)`}
      data-testid="thinking-toggle"
    >
      <span aria-hidden="true">&#129504;</span>
      <span>{current.label}</span>
    </button>
  );
}
