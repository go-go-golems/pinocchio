
import { Provider } from 'react-redux';
import { AppRouter } from './routes';
import { store } from './store/store';
import './index.css';

export function DebugUIApp() {
  return (
    <Provider store={store}>
      <AppRouter />
    </Provider>
  );
}

export default DebugUIApp;
