import { configureStore } from '@reduxjs/toolkit';
import { debugApi } from '../debug-api';
import { debugUiSlice } from './uiSlice';

export const debugStore = configureStore({
  reducer: {
    [debugApi.reducerPath]: debugApi.reducer,
    debugUi: debugUiSlice.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(debugApi.middleware),
});

export type DebugRootState = ReturnType<typeof debugStore.getState>;
export type DebugDispatch = typeof debugStore.dispatch;
