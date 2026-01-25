import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { errorToString } from '../utils/logger';

export type AppError = {
  id: string;
  time: number;
  message: string;
  scope: string;
  detail?: string;
  extra?: Record<string, unknown>;
};

const MAX_ERRORS = 50;

export function makeAppError(
  message: string,
  scope: string,
  err?: unknown,
  extra?: Record<string, unknown>,
): AppError {
  return {
    id: `err-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    time: Date.now(),
    message,
    scope,
    detail: err ? errorToString(err) : undefined,
    extra,
  };
}

export const errorsSlice = createSlice({
  name: 'errors',
  initialState: [] as AppError[],
  reducers: {
    reportError(state, action: PayloadAction<AppError>) {
      state.push(action.payload);
      if (state.length > MAX_ERRORS) {
        state.splice(0, state.length - MAX_ERRORS);
      }
    },
    clearErrors() {
      return [] as AppError[];
    },
  },
});
