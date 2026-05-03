import { createSlice, type PayloadAction } from '@reduxjs/toolkit';

export interface DebugEntity {
  id: string;
  kind: string;
  tombstone?: boolean;
  props: Record<string, unknown>;
}

export interface DebugEvent {
  name: string;
  ordinal: number;
  sessionId: string;
  payload: Record<string, unknown>;
  receivedAt: string;
}

interface DebugState {
  entities: Record<string, DebugEntity>;
  events: DebugEvent[];
  snapshotOrdinal: number;
}

const initialState: DebugState = {
  entities: {},
  events: [],
  snapshotOrdinal: 0,
};

export const debugSlice = createSlice({
  name: 'debug',
  initialState,
  reducers: {
    clear(state) {
      state.entities = {};
      state.events = [];
      state.snapshotOrdinal = 0;
    },
    upsertEntity(state, action: PayloadAction<DebugEntity>) {
      const e = action.payload;
      state.entities[e.id] = e;
    },
    deleteEntity(state, action: PayloadAction<string>) {
      delete state.entities[action.payload];
    },
    appendEvent(state, action: PayloadAction<DebugEvent>) {
      state.events.push(action.payload);
    },
    setSnapshotOrdinal(state, action: PayloadAction<number>) {
      state.snapshotOrdinal = action.payload;
    },
  },
});

export const { clear, upsertEntity, deleteEntity, appendEvent, setSnapshotOrdinal } = debugSlice.actions;
