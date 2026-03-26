import { useChatContext } from '../contexts/ChatContext';

interface TopBarProps {
  title: string;
  children?: React.ReactNode;
}

export function TopBar({ title, children }: TopBarProps) {
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
      role="banner"
      className="h-10 flex items-center justify-between pl-11 md:pl-3 pr-3 border-b border-border-subtle bg-deep sticky top-0 z-20"
    >
      {/* Left: diamond + title + status (pl-11 on mobile leaves space for Sidebar hamburger) */}
      <div className="flex items-center gap-2">
        <svg width="14" height="14" viewBox="0 0 16 16" className="shrink-0" aria-hidden="true">
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
          role="status"
          aria-label={`Connection ${status}`}
        />
      </div>

      {/* Right: product-specific actions */}
      {children && <div className="flex items-center gap-1.5">{children}</div>}
    </div>
  );
}
