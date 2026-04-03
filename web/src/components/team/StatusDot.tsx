/** StatusDot — green/yellow/red presence indicator for agent cards. */

export type AgentStatus = 'working' | 'idle' | 'blocked' | 'offline';

interface StatusDotProps {
  status: AgentStatus;
  size?: 'sm' | 'md' | 'lg';
  /** Accessible label for screen readers */
  label?: string;
}

const COLOR: Record<AgentStatus, string> = {
  working: 'bg-green-500',
  idle:    'bg-yellow-400',
  blocked: 'bg-red-500',
  offline: 'bg-zinc-600',
};

const PULSE: Record<AgentStatus, string> = {
  working: 'animate-pulse',
  idle:    '',
  blocked: '',
  offline: '',
};

const SIZE: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'w-2 h-2',
  md: 'w-2.5 h-2.5',
  lg: 'w-3 h-3',
};

export function StatusDot({ status, size = 'md', label }: StatusDotProps) {
  const ariaLabel = label ?? `Status: ${status}`;
  return (
    <span
      data-testid={`status-dot-${status}`}
      role="img"
      aria-label={ariaLabel}
      className={`inline-block rounded-full shrink-0 ${SIZE[size]} ${COLOR[status]} ${PULSE[status]}`}
    />
  );
}
