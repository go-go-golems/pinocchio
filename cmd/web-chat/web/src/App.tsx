import { Provider } from 'react-redux';
import { DebugUIApp } from './debug-ui';
import { store } from './store/store';
import { ChatWidget } from './webchat';

export function App() {
  const debugMode =
    typeof window !== 'undefined' &&
    new URLSearchParams(window.location.search).get('debug') === '1';

  if (debugMode) {
    return <DebugUIApp />;
  }

  return (
    <Provider store={store}>
      <ChatWidget />
    </Provider>
  );
}
