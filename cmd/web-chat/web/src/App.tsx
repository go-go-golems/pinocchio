import { DebugAppProvider } from './debug-app';
import { ChatWidget } from './webchat';

export function App() {
  const debugMode =
    typeof window !== 'undefined' &&
    new URLSearchParams(window.location.search).get('debug') === '1';

  return debugMode ? <DebugAppProvider /> : <ChatWidget />;
}
