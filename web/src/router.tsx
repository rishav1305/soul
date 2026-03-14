import { createBrowserRouter, useRouteError } from 'react-router';
import { AppLayout } from './layouts/AppLayout';

function RouteErrorFallback() {
  const error = useRouteError() as Error;
  // Use <a> tags instead of <Link> — React Router's errorElement doesn't clear
  // error state on client-side navigation, so a full page load is needed to recover.
  return (
    <div data-testid="route-error" className="flex items-center justify-center h-full bg-deep text-fg">
      <div className="text-center space-y-4 p-8">
        <h2 className="text-lg font-semibold">This page crashed</h2>
        <p className="text-fg-muted text-sm max-w-md">{error?.message || 'An unexpected error occurred'}</p>
        <div className="flex items-center justify-center gap-3">
          <a href="/" className="px-4 py-2 text-sm rounded-lg bg-elevated text-fg hover:bg-elevated/80 transition-colors">
            Dashboard
          </a>
          <a href="/chat" className="px-4 py-2 text-sm rounded-lg bg-soul text-deep font-medium hover:bg-soul/85 transition-colors">
            Go to Chat
          </a>
        </div>
      </div>
    </div>
  );
}

export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      {
        index: true,
        errorElement: <RouteErrorFallback />,
        lazy: () => import('./pages/DashboardPage').then(m => ({ Component: m.DashboardPage })),
      },
      {
        path: 'chat',
        errorElement: <RouteErrorFallback />,
        lazy: () => import('./pages/ChatPage').then(m => ({ Component: m.ChatPage })),
      },
      {
        path: 'tasks',
        errorElement: <RouteErrorFallback />,
        lazy: () => import('./pages/TasksPage').then(m => ({ Component: m.TasksPage })),
      },
      {
        path: 'tasks/:id',
        errorElement: <RouteErrorFallback />,
        lazy: () => import('./pages/TaskDetailPage').then(m => ({ Component: m.TaskDetailPage })),
      },
      {
        path: 'tutor',
        errorElement: <RouteErrorFallback />,
        lazy: () => import('./pages/TutorPage').then(m => ({ Component: m.TutorPage })),
      },
      {
        path: 'tutor/drill/:id',
        errorElement: <RouteErrorFallback />,
        lazy: () => import('./pages/DrillPage').then(m => ({ Component: m.DrillPage })),
      },
      {
        path: 'tutor/mock/:id',
        errorElement: <RouteErrorFallback />,
        lazy: () => import('./pages/MockPage').then(m => ({ Component: m.MockPage })),
      },
    ],
  },
]);
