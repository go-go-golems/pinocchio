import { Provider } from 'react-redux';
import { store } from '../store/store';
import { ChatWidget } from '../webchat';

export function MainWebChatRoot() {
  return (
    <Provider store={store}>
      <ChatWidget />
    </Provider>
  );
}
