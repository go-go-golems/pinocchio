import { Provider } from 'react-redux';
import { debugStore } from '../debug-state';
import { DebugApp } from './DebugApp';

export const DebugAppProvider: React.FC = () => (
  <Provider store={debugStore}>
    <DebugApp />
  </Provider>
);
