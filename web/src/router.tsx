import { createBrowserRouter } from 'react-router';
import { AppLayout } from './layouts/AppLayout';

export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      {
        index: true,
        lazy: () => import('./pages/DashboardPage').then(m => ({ Component: m.DashboardPage })),
      },
      {
        path: 'chat',
        lazy: () => import('./pages/ChatPage').then(m => ({ Component: m.ChatPage })),
      },
      {
        path: 'tasks',
        lazy: () => import('./pages/TasksPage').then(m => ({ Component: m.TasksPage })),
      },
      {
        path: 'tasks/:id',
        lazy: () => import('./pages/TaskDetailPage').then(m => ({ Component: m.TaskDetailPage })),
      },
    ],
  },
]);
