import { NavLink, Outlet } from 'react-router';
import { useChatContext } from '../contexts/ChatContext';

function navLinkClass({ isActive }: { isActive: boolean }) {
  return `px-3 py-1 text-sm rounded-md transition-colors ${
    isActive
      ? 'bg-elevated text-fg'
      : 'text-fg-muted hover:text-fg hover:bg-elevated/50'
  }`;
}

const statusColor: Record<string, string> = {
  connected: 'bg-emerald-400',
  connecting: 'bg-yellow-400 animate-pulse',
  disconnected: 'bg-zinc-500',
  error: 'bg-red-400',
};

export function AppLayout() {
  const { status } = useChatContext();

  return (
    <div data-testid="app-layout" className="h-screen bg-deep text-fg flex flex-col noise">
      <header className="glass flex items-center justify-between px-4 h-11 shrink-0">
        <div className="flex items-center gap-3">
          <span className="text-soul text-xl drop-shadow-[0_0_8px_rgba(232,168,73,0.4)]" aria-hidden="true">&#9670;</span>
          <h1 className="text-base font-semibold text-fg tracking-tight">Soul</h1>
          <nav className="hidden sm:flex items-center gap-1 ml-4" data-testid="main-nav">
            <NavLink to="/" end className={navLinkClass}>Dashboard</NavLink>
            <NavLink to="/chat" className={navLinkClass}>Chat</NavLink>
            <NavLink to="/tasks" className={navLinkClass}>Tasks</NavLink>
            <NavLink to="/tutor" className={navLinkClass}>Tutor</NavLink>
          </nav>
        </div>
        <div className="flex items-center gap-2" data-testid="connection-status">
          <span className={`w-2 h-2 rounded-full ${statusColor[status] ?? 'bg-zinc-500'}`} title={`Chat: ${status}`} />
        </div>
      </header>
      <div className="flex-1 min-h-0">
        <Outlet />
      </div>
    </div>
  );
}
