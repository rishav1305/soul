/** NavigationSidebar — persistent left nav for the team section with 5 pages and notification badges. */

interface NavItem {
  path: string;
  label: string;
  testId: string;
  icon: React.ReactNode;
  badge?: number;
}

interface NavigationSidebarProps {
  currentPath: string;
  onNavigate: (to: string) => void;
  unreadCount?: number;
  alertCount?: number;
}

function TeamIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="6" cy="5" r="2.5" />
      <circle cx="11" cy="5" r="2" />
      <path d="M1 13c0-2.5 2.2-4 5-4s5 1.5 5 4" />
      <path d="M11 9c1.8.4 3 1.5 3 3" />
    </svg>
  );
}

function ChatIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 3h12a1 1 0 0 1 1 1v7a1 1 0 0 1-1 1H5l-3 2V4a1 1 0 0 1 1-1z" />
    </svg>
  );
}

function TaskIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="2" width="12" height="12" rx="2" />
      <path d="M5 8l2 2 4-4" />
    </svg>
  );
}

function HealthIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1 8h3l2-5 2 10 2-6 1 3 1-2h3" />
    </svg>
  );
}

export function NavigationSidebar({ currentPath, onNavigate, unreadCount = 0, alertCount = 0 }: NavigationSidebarProps) {
  const navItems: NavItem[] = [
    {
      path: '/team',
      label: 'Dashboard',
      testId: 'nav-team-dashboard',
      icon: <TeamIcon />,
    },
    {
      path: '/team/chat',
      label: 'Chat',
      testId: 'nav-team-chat',
      icon: <ChatIcon />,
      badge: unreadCount,
    },
    {
      path: '/team/tasks',
      label: 'Tasks',
      testId: 'nav-team-tasks',
      icon: <TaskIcon />,
    },
    {
      path: '/team/health',
      label: 'Health',
      testId: 'nav-team-health',
      icon: <HealthIcon />,
      badge: alertCount,
    },
  ];

  return (
    <nav
      data-testid="team-nav-sidebar"
      className="flex flex-col gap-1 py-3 px-2 border-r border-border-subtle bg-surface/30 w-44 shrink-0"
      aria-label="Team navigation"
    >
      <div className="px-2 py-1 mb-1">
        <span className="text-[10px] text-fg-muted font-medium uppercase tracking-widest">
          Team
        </span>
      </div>

      {navItems.map((item) => {
        const isActive = currentPath === item.path
          || (item.path !== '/team' && currentPath.startsWith(item.path));

        return (
          <button
            key={item.path}
            type="button"
            data-testid={item.testId}
            onClick={() => onNavigate(item.path)}
            className={`relative flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors cursor-pointer w-full text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-soul/50 ${
              isActive
                ? 'bg-soul/10 text-soul'
                : 'text-fg-secondary hover:text-fg hover:bg-overlay/60'
            }`}
            aria-current={isActive ? 'page' : undefined}
            aria-label={item.label}
          >
            <span aria-hidden="true">{item.icon}</span>
            <span>{item.label}</span>

            {item.badge !== undefined && item.badge > 0 && (
              <span
                data-testid={`nav-badge-${item.testId}`}
                className="ml-auto inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[9px] font-bold rounded-full bg-soul text-deep"
                aria-label={`${item.badge} notifications`}
              >
                {item.badge > 9 ? '9+' : item.badge}
              </span>
            )}
          </button>
        );
      })}
    </nav>
  );
}
