import { Outlet } from 'react-router';
import { Sidebar } from '../components/Sidebar';

export function AppLayout() {
  return (
    <div data-testid="app-layout" className="h-screen bg-deep text-fg flex noise">
      {/* Left sidebar — product navigation */}
      <Sidebar />

      {/* Main content area */}
      <div className="flex-1 flex flex-col min-w-0 min-h-0">
        <Outlet />
      </div>
    </div>
  );
}
