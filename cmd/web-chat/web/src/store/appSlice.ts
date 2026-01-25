import { createSlice, type PayloadAction } from '@reduxjs/toolkit';

export const appSlice = createSlice({
  name: 'app',
  initialState: {
    convId: '' as string,
    runId: '' as string,
    status: 'idle' as string,
    wsStatus: 'disconnected' as string,
    lastSeq: 0 as number,
    queueDepth: 0 as number,
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
    setWsStatus(state, action: PayloadAction<string>) {
      state.wsStatus = action.payload;
    },
    setLastSeq(state, action: PayloadAction<number>) {
      state.lastSeq = action.payload;
    },
    setQueueDepth(state, action: PayloadAction<number>) {
      state.queueDepth = action.payload;
    },
  },
});
