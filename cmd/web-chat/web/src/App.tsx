import { Provider } from 'react-redux';
import { DebugUIApp } from './debug-ui';
import { store } from './store/store';
import { ChatWidget } from './webchat';
import { ProviderDemoPage } from './webchat/ProviderDemoPage';

export function App() {
  const debugMode =
    typeof window !== 'undefined' &&
    new URLSearchParams(window.location.search).get('debug') === '1';

  if (debugMode) {
    return <DebugUIApp />;
  }

  const providerDemoMode =
    typeof window !== 'undefined' &&
    new URLSearchParams(window.location.search).get('providerDemo') === '1';

  if (providerDemoMode) {
    return <ProviderDemoPage />;
  }

  return (
    <Provider store={store}>
      <ChatWidget />
    </Provider>
  );
}
