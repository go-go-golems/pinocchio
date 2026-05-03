
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { routerBasenameFromRuntimeConfig } from '../../utils/basePrefix';
import { AppShell } from '../components/AppShell';
import { EventsPage } from './EventsPage';
import { OverviewPage } from './OverviewPage';
import { TimelinePage } from './TimelinePage';

export const router = createBrowserRouter(
  [
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
      ],
    },
  ],
  { basename: routerBasenameFromRuntimeConfig() },
);

export function AppRouter() {
  return <RouterProvider router={router} />;
}

export default AppRouter;
