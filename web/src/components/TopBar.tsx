import { useChatContext } from '../contexts/ChatContext';

interface TopBarProps {
  title: string;
  children?: React.ReactNode;
}

export default function TopBar({ title, children }: TopBarProps) {
  const { status } = useChatContext();

  const statusColor =
    status === 'connected'
      ? 'bg-emerald-400'
      : status === 'connecting'
        ? 'bg-yellow-400 animate-pulse'
        : 'bg-red-500';

  return (
    <div
      data-testid="top-bar"
      className="h-10 flex items-center justify-between px-3 border-b border-border-subtle bg-deep"
    >
      {/* Left: diamond + title + status */}
      <div className="flex items-center gap-2">
        <svg width="14" height="14" viewBox="0 0 16 16" className="shrink-0">
          <defs>
            <linearGradient id="gold-diamond" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#f0c040" />
              <stop offset="50%" stopColor="#d4a018" />
              <stop offset="100%" stopColor="#b8860b" />
            </linearGradient>
          </defs>
          <path d="M8 0L14 8L8 16L2 8Z" fill="url(#gold-diamond)" />
        </svg>
        <span data-testid="top-bar-title" className="text-sm font-medium text-fg">
          {title}
        </span>
        <span
          data-testid="top-bar-status"
          className={`w-2 h-2 rounded-full ${statusColor}`}
        />
      </div>

      {/* Right: product-specific actions */}
      {children && <div className="flex items-center gap-1.5">{children}</div>}
    </div>
  );
}
