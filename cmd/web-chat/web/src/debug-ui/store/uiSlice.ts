import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type { TurnPhase } from '../types';

interface UiState {
  // Selection state
  selectedConvId: string | null;
  selectedSessionId: string | null;
  selectedTurnId: string | null;
  selectedEntityId: string | null;
  selectedRunId: string | null;
  selectedPhase: TurnPhase;
  selectedSeq: number | null;

  // Offline source configuration
  offline: {
    artifactsRoot: string;
    turnsDB: string;
    timelineDB: string;
  };

  // Diff state
  comparePhaseA: TurnPhase | null;
  comparePhaseB: TurnPhase | null;
}

const initialState: UiState = {
  selectedConvId: null,
  selectedSessionId: null,
  selectedTurnId: null,
  selectedEntityId: null,
  selectedRunId: null,
  selectedPhase: 'final',
  selectedSeq: null,
  offline: {
    artifactsRoot: '',
    turnsDB: '',
    timelineDB: '',
  },

  comparePhaseA: null,
  comparePhaseB: null,
};

export const uiSlice = createSlice({
  name: 'ui',
  initialState,
  reducers: {
    selectConversation: (state, action: PayloadAction<string | null>) => {
      state.selectedConvId = action.payload;
      state.selectedSessionId = null;
      state.selectedTurnId = null;
      state.selectedEntityId = null;
      state.selectedSeq = null;
    },
    selectSession: (state, action: PayloadAction<string | null>) => {
      state.selectedSessionId = action.payload;
      state.selectedTurnId = null;
      state.selectedEntityId = null;
      state.selectedSeq = null;
    },
    selectTurn: (state, action: PayloadAction<string | null>) => {
      state.selectedTurnId = action.payload;
    },
    selectRun: (state, action: PayloadAction<string | null>) => {
      state.selectedRunId = action.payload;
    },
    selectEntity: (state, action: PayloadAction<string | null>) => {
      state.selectedEntityId = action.payload;
    },
    setOfflineConfig: (
      state,
      action: PayloadAction<Partial<UiState['offline']>>
    ) => {
      state.offline = { ...state.offline, ...action.payload };
    },
    selectPhase: (state, action: PayloadAction<TurnPhase>) => {
      state.selectedPhase = action.payload;
    },
    selectEvent: (state, action: PayloadAction<number | null>) => {
      state.selectedSeq = action.payload;
    },
    setComparePhases: (
      state,
      action: PayloadAction<{ a: TurnPhase | null; b: TurnPhase | null }>
    ) => {
      state.comparePhaseA = action.payload.a;
      state.comparePhaseB = action.payload.b;
    },
  },
});

export const {
  selectConversation,
  selectSession,
  selectTurn,
  selectRun,
  selectEntity,
  setOfflineConfig,
  selectPhase,
  selectEvent,
  setComparePhases,
} = uiSlice.actions;
