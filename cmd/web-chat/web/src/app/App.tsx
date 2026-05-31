import { DebugUiRoot } from './DebugUiRoot';
import { MainWebChatRoot } from './MainWebChatRoot';
import { routeModeFromLocation } from './routeMode';

export function App() {
  const mode = routeModeFromLocation(window.location);

  switch (mode.kind) {
    case 'debug':
      return <DebugUiRoot />;
    case 'chat':
      return <MainWebChatRoot />;
  }
}
