import { devToolsEnhancer } from '@redux-devtools/remote';
import { configureStore } from '@reduxjs/toolkit';
import { appSlice } from './appSlice';
import { errorsSlice } from './errorsSlice';
import { timelineSlice } from './timelineSlice';

const enableRemoteDevtools = import.meta.env.DEV && import.meta.env.VITE_REMOTE_DEVTOOLS === '1';
const remoteDevtoolsHost = import.meta.env.VITE_REMOTE_DEVTOOLS_HOST ?? 'localhost';
const remoteDevtoolsPort = Number(import.meta.env.VITE_REMOTE_DEVTOOLS_PORT ?? '8000');
const remoteDevtoolsConfig = {
  realtime: true,
  hostname: remoteDevtoolsHost,
  port: Number.isFinite(remoteDevtoolsPort) ? remoteDevtoolsPort : 8000,
};

export const store = configureStore({
  reducer: {
    app: appSlice.reducer,
    timeline: timelineSlice.reducer,
    errors: errorsSlice.reducer,
  },
  devTools: !enableRemoteDevtools,
  enhancers: (getDefaultEnhancers) =>
    enableRemoteDevtools ? getDefaultEnhancers().concat(devToolsEnhancer(remoteDevtoolsConfig)) : getDefaultEnhancers(),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
