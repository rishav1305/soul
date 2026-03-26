import { Outlet } from 'react-router';
import { Sidebar } from '../components/Sidebar';

export function AppLayout() {
  return (
    <div data-testid="app-layout" className="h-screen bg-deep text-fg flex noise">
      {/* Skip to main content — visible on focus for keyboard navigation */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:z-[100] focus:top-2 focus:left-2 focus:px-4 focus:py-2 focus:bg-soul focus:text-deep focus:rounded-lg focus:text-sm focus:font-medium"
        data-testid="skip-nav"
      >
        Skip to main content
      </a>

      {/* Left sidebar — product navigation */}
      <Sidebar />

      {/* Main content area */}
      <main id="main-content" className="flex-1 flex flex-col min-w-0 min-h-0">
        <Outlet />
      </main>
    </div>
  );
}
