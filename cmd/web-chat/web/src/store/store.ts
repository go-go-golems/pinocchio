import { devToolsEnhancer } from '@redux-devtools/remote';
import { configureStore } from '@reduxjs/toolkit';
import { appSlice } from './appSlice';
import { errorsSlice } from './errorsSlice';
import { profileApi } from './profileApi';
import { timelineSlice } from './timelineSlice';

const enableRemoteDevtools = import.meta.env.DEV && import.meta.env.VITE_REMOTE_DEVTOOLS === '1';
const remoteDevtoolsHost = import.meta.env.VITE_REMOTE_DEVTOOLS_HOST ?? 'localhost';
const remoteDevtoolsPort = Number(import.meta.env.VITE_REMOTE_DEVTOOLS_PORT ?? '8000');
const remoteDevtoolsConfig = {
  realtime: true,
  hostname: remoteDevtoolsHost,
  port: Number.isFinite(remoteDevtoolsPort) ? remoteDevtoolsPort : 8000,
};
const middleware = (getDefaultMiddleware: any) => getDefaultMiddleware().concat(profileApi.middleware);

export const store = configureStore({
  reducer: {
    app: appSlice.reducer,
    timeline: timelineSlice.reducer,
    errors: errorsSlice.reducer,
    [profileApi.reducerPath]: profileApi.reducer,
  },
  devTools: !enableRemoteDevtools,
  enhancers: (getDefaultEnhancers) =>
    enableRemoteDevtools ? getDefaultEnhancers().concat(devToolsEnhancer(remoteDevtoolsConfig)) : getDefaultEnhancers(),
  middleware,
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
