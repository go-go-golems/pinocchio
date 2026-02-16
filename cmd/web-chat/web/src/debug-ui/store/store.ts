import { configureStore } from '@reduxjs/toolkit';
import { debugApi } from '../api/debugApi';
import { uiSlice } from './uiSlice';

export const store = configureStore({
  reducer: {
    [debugApi.reducerPath]: debugApi.reducer,
    ui: uiSlice.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(debugApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
