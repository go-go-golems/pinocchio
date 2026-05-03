import { configureStore } from '@reduxjs/toolkit';
import { debugSlice } from './debugSlice';
import { uiSlice } from './uiSlice';

export const store = configureStore({
  reducer: {
    debug: debugSlice.reducer,
    ui: uiSlice.reducer,
  },
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
