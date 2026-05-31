import { DebugUiRoot } from './DebugUiRoot';
import { MainWebChatRoot } from './MainWebChatRoot';
import { ProviderDemoRoot } from './ProviderDemoRoot';
import { ProviderMultiDemoRoot } from './ProviderMultiDemoRoot';
import { routeModeFromLocation } from './routeMode';

export function App() {
  const mode = routeModeFromLocation(window.location);

  switch (mode.kind) {
    case 'debug':
      return <DebugUiRoot />;
    case 'provider-demo':
      return <ProviderDemoRoot />;
    case 'provider-multi-demo':
      return <ProviderMultiDemoRoot />;
    case 'chat':
      return <MainWebChatRoot />;
  }
}
