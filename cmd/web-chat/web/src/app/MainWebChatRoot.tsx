import { Provider } from 'react-redux';
import { WebChatProviderShell } from '../features/web-chat';
import { store } from '../store/store';

export function MainWebChatRoot() {
  return (
    <Provider store={store}>
      <WebChatProviderShell />
    </Provider>
  );
}
