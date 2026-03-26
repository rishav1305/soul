import { useState, useEffect, useCallback } from 'react';
import { NavLink } from 'react-router';

const STORAGE_KEY = 'soul-v2-sidebar';

interface NavItem {
  to: string;
  label: string;
  icon: React.ReactNode;
  end?: boolean;
}

/* ---------- SVG Icons (14x14, 1.5px stroke) ---------- */

function IconDashboard() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1" y="1" width="5" height="5" rx="1" />
      <rect x="8" y="1" width="5" height="5" rx="1" />
      <rect x="1" y="8" width="5" height="5" rx="1" />
      <rect x="8" y="8" width="5" height="5" rx="1" />
    </svg>
  );
}

function IconChat() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 2h10a1 1 0 011 1v6a1 1 0 01-1 1H5l-3 3V3a1 1 0 011-1z" />
    </svg>
  );
}

function IconTasks() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 3.5l2 2 3-3" />
      <path d="M9 4h3" />
      <path d="M2 8.5l2 2 3-3" />
      <path d="M9 9h3" />
    </svg>
  );
}

function IconTutor() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1 4l6-2.5L13 4 7 6.5z" />
      <path d="M3 5v4c0 1 2 2.5 4 2.5s4-1.5 4-2.5V5" />
    </svg>
  );
}

function IconProjects() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M2 3h3l1.5 1.5H12a1 1 0 011 1V11a1 1 0 01-1 1H2a1 1 0 01-1-1V4a1 1 0 011-1z" />
    </svg>
  );
}

function IconObserve() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="7" cy="7" r="3" />
      <circle cx="7" cy="7" r="5.5" />
    </svg>
  );
}

function IconScout() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="7" cy="7" r="5.5" />
      <circle cx="7" cy="7" r="2" />
      <line x1="7" y1="1" x2="7" y2="3" />
      <line x1="7" y1="11" x2="7" y2="13" />
      <line x1="1" y1="7" x2="3" y2="7" />
      <line x1="11" y1="7" x2="13" y2="7" />
    </svg>
  );
}

function IconSentinel() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M7 1L2 3v4c0 3.5 2.5 5.5 5 6.5 2.5-1 5-3 5-6.5V3z" />
    </svg>
  );
}

function IconMesh() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="7" cy="2.5" r="1.5" />
      <circle cx="2.5" cy="11" r="1.5" />
      <circle cx="11.5" cy="11" r="1.5" />
      <line x1="7" y1="4" x2="3.5" y2="9.5" />
      <line x1="7" y1="4" x2="10.5" y2="9.5" />
      <line x1="4" y1="11" x2="10" y2="11" />
    </svg>
  );
}

function IconBench() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <line x1="2" y1="12" x2="2" y2="6" />
      <line x1="5" y1="12" x2="5" y2="3" />
      <line x1="8" y1="12" x2="8" y2="8" />
      <line x1="11" y1="12" x2="11" y2="2" />
    </svg>
  );
}

function IconSearch() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="6" cy="6" r="4" />
      <line x1="9" y1="9" x2="12.5" y2="12.5" />
    </svg>
  );
}

/* ---------- Nav items ---------- */

// Generic icon for chat-only products (no dedicated page)
function IconGeneric() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="7" cy="7" r="5.5" />
      <path d="M7 4.5v3l2 1" />
    </svg>
  );
}

function IconCompliance() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M7 1L2 3v4c0 3 2.5 5 5 6 2.5-1 5-3 5-6V3z" />
      <path d="M5 7l2 2 3-3" />
    </svg>
  );
}

function IconQA() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="6" cy="6" r="4" />
      <line x1="9" y1="9" x2="12.5" y2="12.5" />
      <path d="M6 4v2.5" />
      <circle cx="6" cy="8" r="0.5" fill="currentColor" stroke="none" />
    </svg>
  );
}

function IconAnalytics() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="1,10 4,6 7,8 10,3 13,5" />
      <line x1="1" y1="12" x2="13" y2="12" />
    </svg>
  );
}

function IconDevOps() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="7" cy="7" r="2.5" />
      <path d="M7 1v2M7 11v2M1 7h2M11 7h2M2.8 2.8l1.4 1.4M9.8 9.8l1.4 1.4M11.2 2.8l-1.4 1.4M4.2 9.8l-1.4 1.4" />
    </svg>
  );
}

function IconDBA() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <ellipse cx="7" cy="3.5" rx="5" ry="2" />
      <path d="M2 3.5v7c0 1.1 2.2 2 5 2s5-.9 5-2v-7" />
      <path d="M2 7c0 1.1 2.2 2 5 2s5-.9 5-2" />
    </svg>
  );
}

function IconMigrate() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M1 7h12M9 3l4 4-4 4" />
    </svg>
  );
}

function IconDataEng() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M5 2L2 5l3 3" />
      <path d="M9 6l3 3-3 3" />
      <line x1="8" y1="1" x2="6" y2="13" />
    </svg>
  );
}

function IconCostOps() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <line x1="7" y1="1" x2="7" y2="13" />
      <path d="M10 4.5C10 3.1 8.7 2 7 2S4 3.1 4 4.5 5.3 7 7 7s3 1.1 3 2.5S8.7 12 7 12s-3-1.1-3-2.5" />
    </svg>
  );
}

function IconViz() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1" y="6" width="3" height="6" rx="0.5" />
      <rect x="5.5" y="3" width="3" height="9" rx="0.5" />
      <rect x="10" y="1" width="3" height="11" rx="0.5" />
    </svg>
  );
}

function IconDocs() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 1h6l3 3v9H3z" />
      <path d="M9 1v3h3" />
      <line x1="5" y1="7" x2="9" y2="7" />
      <line x1="5" y1="9.5" x2="9" y2="9.5" />
    </svg>
  );
}

function IconAPI() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 4L1 7l3 3" />
      <path d="M10 4l3 3-3 3" />
      <line x1="8.5" y1="2" x2="5.5" y2="12" />
    </svg>
  );
}

const navItems: NavItem[] = [
  { to: '/', label: 'Dashboard', icon: <IconDashboard />, end: true },
  { to: '/chat', label: 'Chat', icon: <IconChat /> },
  { to: '/tasks', label: 'Tasks', icon: <IconTasks /> },
  { to: '/tutor', label: 'Tutor', icon: <IconTutor /> },
  { to: '/projects', label: 'Projects', icon: <IconProjects /> },
  { to: '/observe', label: 'Observe', icon: <IconObserve /> },
  { to: '/scout', label: 'Scout', icon: <IconScout /> },
  { to: '/sentinel', label: 'Sentinel', icon: <IconSentinel /> },
  { to: '/mesh', label: 'Mesh', icon: <IconMesh /> },
  { to: '/bench', label: 'Bench', icon: <IconBench /> },
  { to: '/chat?product=compliance', label: 'Compliance', icon: <IconCompliance /> },
  { to: '/chat?product=qa', label: 'QA', icon: <IconQA /> },
  { to: '/chat?product=analytics', label: 'Analytics', icon: <IconAnalytics /> },
  { to: '/chat?product=devops', label: 'DevOps', icon: <IconDevOps /> },
  { to: '/chat?product=dba', label: 'DBA', icon: <IconDBA /> },
  { to: '/chat?product=migrate', label: 'Migrate', icon: <IconMigrate /> },
  { to: '/chat?product=dataeng', label: 'DataEng', icon: <IconDataEng /> },
  { to: '/chat?product=costops', label: 'CostOps', icon: <IconCostOps /> },
  { to: '/chat?product=viz', label: 'Viz', icon: <IconViz /> },
  { to: '/chat?product=docs', label: 'Docs', icon: <IconDocs /> },
  { to: '/chat?product=api', label: 'API', icon: <IconAPI /> },
];

/* ---------- Logo ---------- */

function DiamondLogo({ size = 24 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden="true">
      <defs>
        <linearGradient id="soul-gold" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#f0c040" />
          <stop offset="50%" stopColor="#d4a018" />
          <stop offset="100%" stopColor="#b8860b" />
        </linearGradient>
      </defs>
      <path d="M8 0L14 8L8 16L2 8Z" fill="url(#soul-gold)" />
    </svg>
  );
}

/* ---------- Sidebar ---------- */

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'collapsed';
  } catch {
    return false;
  }
}

function writeCollapsed(v: boolean) {
  try {
    localStorage.setItem(STORAGE_KEY, v ? 'collapsed' : 'expanded');
  } catch {
    // storage unavailable
  }
}

export function Sidebar() {
  const [collapsed, setCollapsed] = useState(readCollapsed);
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    writeCollapsed(collapsed);
  }, [collapsed]);

  const toggleCollapse = useCallback(() => {
    setCollapsed(prev => !prev);
  }, []);

  const closeMobile = useCallback(() => {
    setMobileOpen(false);
  }, []);

  const toggleMobile = useCallback(() => {
    setMobileOpen(prev => !prev);
  }, []);


  function navLinkClass({ isActive }: { isActive: boolean }) {
    if (collapsed && !mobileOpen) {
      return [
        'flex items-center justify-center w-9 h-9 rounded-lg transition-colors',
        isActive
          ? 'bg-soul/10 text-soul'
          : 'text-fg-muted hover:text-fg hover:bg-elevated/50',
      ].join(' ');
    }
    return [
      'flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors',
      isActive
        ? 'bg-soul/10 text-soul'
        : 'text-fg-muted hover:text-fg hover:bg-elevated/50',
    ].join(' ');
  }

  const sidebarContent = (
    <>
      {/* Logo + title */}
      <div className="flex items-center gap-2.5 px-3 pt-4 pb-2">
        <DiamondLogo size={collapsed && !mobileOpen ? 20 : 24} />
        {(!collapsed || mobileOpen) && (
          <span className="text-base font-semibold text-fg tracking-tight">Soul</span>
        )}
      </div>

      {/* Search bar (expanded only) */}
      {(!collapsed || mobileOpen) && (
        <div className="px-3 py-2" data-testid="sidebar-search">
          <div className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg bg-surface border border-border-subtle text-fg-muted text-sm">
            <IconSearch />
            <span className="flex-1">Search...</span>
            <kbd className="text-[10px] px-1.5 py-0.5 rounded bg-elevated border border-border-subtle text-fg-secondary font-mono">
              ⌘K
            </kbd>
          </div>
        </div>
      )}

      {/* Nav items */}
      <nav className="flex-1 overflow-y-auto px-2 py-2 space-y-0.5" data-testid="sidebar-nav" aria-label="Product pages">
        {navItems.map(item => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            className={navLinkClass}
            onClick={mobileOpen ? closeMobile : undefined}
            title={collapsed && !mobileOpen ? item.label : undefined}
            data-testid={`sidebar-nav-${item.label.toLowerCase()}`}
          >
            <span className="shrink-0 flex items-center justify-center w-5 h-5" aria-hidden="true">{item.icon}</span>
            {(!collapsed || mobileOpen) && <span>{item.label}</span>}
          </NavLink>
        ))}
      </nav>

      {/* Collapse toggle (desktop only) */}
      <div className="hidden md:block px-2 pb-3">
        <button
          onClick={toggleCollapse}
          className="w-full flex items-center justify-center py-1.5 rounded-lg text-fg-muted hover:text-fg hover:bg-elevated/50 text-xs transition-colors"
          data-testid="sidebar-collapse-btn"
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          aria-expanded={!collapsed}
        >
          {collapsed ? '\u00BB' : '\u00AB'}
        </button>
      </div>
    </>
  );

  return (
    <>
      {/* Desktop sidebar */}
      <aside
        data-testid="sidebar"
        role="navigation"
        aria-label="Product navigation"
        className="hidden md:flex flex-col bg-deep border-r border-border-subtle h-full sidebar-transition shrink-0"
        style={{ width: collapsed ? 52 : 200 }}
      >
        {sidebarContent}
      </aside>

      {/* Mobile hamburger button */}
      <button
        data-testid="sidebar-hamburger"
        className="md:hidden fixed top-1.5 left-2 z-50 flex items-center justify-center w-8 h-8 rounded-lg text-fg-muted hover:text-fg transition-colors cursor-pointer"
        onClick={toggleMobile}
        aria-label="Toggle navigation"
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
          {mobileOpen ? (
            <>
              <line x1="3" y1="3" x2="13" y2="13" />
              <line x1="13" y1="3" x2="3" y2="13" />
            </>
          ) : (
            <>
              <line x1="2" y1="4" x2="14" y2="4" />
              <line x1="2" y1="8" x2="14" y2="8" />
              <line x1="2" y1="12" x2="14" y2="12" />
            </>
          )}
        </svg>
      </button>

      {/* Mobile overlay */}
      {mobileOpen && (
        <>
          <div
            data-testid="sidebar-backdrop"
            className="md:hidden fixed inset-0 bg-black/60 z-40"
            onClick={closeMobile}
          />
          <aside
            role="navigation"
            aria-label="Product navigation"
            className="md:hidden fixed top-0 left-0 bottom-0 w-[min(85vw,280px)] bg-deep border-r border-border-subtle z-50 flex flex-col sidebar-transition"
          >
            {sidebarContent}
          </aside>
        </>
      )}
    </>
  );
}
