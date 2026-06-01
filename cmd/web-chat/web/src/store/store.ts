import { configureStore } from '@reduxjs/toolkit';
import { appSlice } from './appSlice';
import { profileApi } from './profileApi';

export const store = configureStore({
  reducer: {
    app: appSlice.reducer,
    [profileApi.reducerPath]: profileApi.reducer,
  },
  devTools: false,
  middleware: (getDefaultMiddleware) => getDefaultMiddleware().concat(profileApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
