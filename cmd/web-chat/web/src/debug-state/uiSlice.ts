import { createSlice, type PayloadAction } from '@reduxjs/toolkit';

export interface OfflineSourceConfig {
  artifactsRoot: string;
  turnsDB: string;
  timelineDB: string;
}

interface DebugUiState {
  selectedConvId: string | null;
  selectedSessionId: string | null;
  selectedTurnId: string | null;
  selectedRunId: string | null;
  selectedPhase: string;
  offline: OfflineSourceConfig;
}

const initialState: DebugUiState = {
  selectedConvId: null,
  selectedSessionId: null,
  selectedTurnId: null,
  selectedRunId: null,
  selectedPhase: 'final',
  offline: {
    artifactsRoot: '',
    turnsDB: '',
    timelineDB: '',
  },
};

export const debugUiSlice = createSlice({
  name: 'debugUi',
  initialState,
  reducers: {
    selectConversation: (state, action: PayloadAction<string | null>) => {
      state.selectedConvId = action.payload;
      state.selectedSessionId = null;
      state.selectedTurnId = null;
    },
    selectSession: (state, action: PayloadAction<string | null>) => {
      state.selectedSessionId = action.payload;
      state.selectedTurnId = null;
    },
    selectTurn: (state, action: PayloadAction<string | null>) => {
      state.selectedTurnId = action.payload;
    },
    selectRun: (state, action: PayloadAction<string | null>) => {
      state.selectedRunId = action.payload;
    },
    setPhase: (state, action: PayloadAction<string>) => {
      state.selectedPhase = action.payload;
    },
    setOfflineConfig: (state, action: PayloadAction<Partial<OfflineSourceConfig>>) => {
      state.offline = { ...state.offline, ...action.payload };
    },
  },
});

export const {
  selectConversation,
  selectSession,
  selectTurn,
  selectRun,
  setPhase,
  setOfflineConfig,
} = debugUiSlice.actions;
