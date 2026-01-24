import { configureStore } from '@reduxjs/toolkit';
import { appSlice } from './appSlice';
import { timelineSlice } from './timelineSlice';

export const store = configureStore({
  reducer: {
    app: appSlice.reducer,
    timeline: timelineSlice.reducer,
  },
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;

