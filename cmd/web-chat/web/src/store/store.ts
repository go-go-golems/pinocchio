import { configureStore } from '@reduxjs/toolkit';
import { appSlice } from './appSlice';
import { profileApi } from './profileApi';

const middleware = (getDefaultMiddleware: any) => getDefaultMiddleware().concat(profileApi.middleware);

export const store = configureStore({
  reducer: {
    app: appSlice.reducer,
    [profileApi.reducerPath]: profileApi.reducer,
  },
  devTools: false,
  middleware,
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
