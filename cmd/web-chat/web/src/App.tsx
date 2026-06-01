import { Provider } from 'react-redux';
import { WebChatProviderShell } from './features/web-chat';
import { store } from './store/store';

export function App() {
  return (
    <Provider store={store}>
      <WebChatProviderShell />
    </Provider>
  );
}
