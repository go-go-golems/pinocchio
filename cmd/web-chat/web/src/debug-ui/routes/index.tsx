
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { AppShell } from '../components/AppShell';
import { EventsPage } from './EventsPage';
import { OfflinePage } from './OfflinePage';
import { OverviewPage } from './OverviewPage';
import { TimelinePage } from './TimelinePage';
import { TurnDetailPage } from './TurnDetailPage';

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      {
        index: true,
        element: <OverviewPage />,
      },
      {
        path: 'timeline',
        element: <TimelinePage />,
      },
      {
        path: 'events',
        element: <EventsPage />,
      },
      {
        path: 'offline',
        element: <OfflinePage />,
      },
      {
        path: 'session/:sessionId',
        children: [
          {
            index: true,
            element: <OverviewPage />,
          },
          {
            path: 'turn/:turnId',
            element: <TurnDetailPage />,
          },
        ],
      },
    ],
  },
]);

export function AppRouter() {
  return <RouterProvider router={router} />;
}

export default AppRouter;
