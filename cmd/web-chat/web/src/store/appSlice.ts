import { createSlice, PayloadAction } from '@reduxjs/toolkit';

export const appSlice = createSlice({
  name: 'app',
  initialState: {
    convId: '' as string,
    runId: '' as string,
    status: 'idle' as string,
  },
  reducers: {
    setConvId(state, action: PayloadAction<string>) {
      state.convId = action.payload;
    },
    setRunId(state, action: PayloadAction<string>) {
      state.runId = action.payload;
    },
    setStatus(state, action: PayloadAction<string>) {
      state.status = action.payload;
    },
  },
});

