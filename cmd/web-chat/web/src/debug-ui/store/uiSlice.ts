import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type { TurnPhase } from '../types';
import type { RootState } from './store';

export type FollowStatus =
  | 'idle'
  | 'connecting'
  | 'bootstrapping'
  | 'connected'
  | 'error'
  | 'closed';

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

  // Realtime follow state
  follow: {
    enabled: boolean;
    targetConvId: string | null;
    status: FollowStatus;
    reconnectToken: number;
    lastError: string | null;
  };
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
  follow: {
    enabled: false,
    targetConvId: null,
    status: 'idle',
    reconnectToken: 0,
    lastError: null,
  },
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
      if (state.follow.enabled) {
        state.follow.targetConvId = action.payload;
        state.follow.status = action.payload ? 'connecting' : 'idle';
      }
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
    startFollow: (state, action: PayloadAction<string>) => {
      state.follow.enabled = true;
      state.follow.targetConvId = action.payload;
      state.follow.status = 'connecting';
      state.follow.lastError = null;
    },
    pauseFollow: (state) => {
      state.follow.enabled = false;
      state.follow.status = state.follow.targetConvId ? 'closed' : 'idle';
    },
    resumeFollow: (state) => {
      if (!state.follow.targetConvId) {
        return;
      }
      state.follow.enabled = true;
      state.follow.status = 'connecting';
      state.follow.lastError = null;
    },
    stopFollow: (state) => {
      state.follow.enabled = false;
      state.follow.targetConvId = null;
      state.follow.status = 'idle';
      state.follow.lastError = null;
    },
    setFollowTarget: (state, action: PayloadAction<string | null>) => {
      state.follow.targetConvId = action.payload;
      if (!action.payload) {
        state.follow.enabled = false;
        state.follow.status = 'idle';
      }
    },
    setFollowStatus: (state, action: PayloadAction<FollowStatus>) => {
      state.follow.status = action.payload;
    },
    setFollowError: (state, action: PayloadAction<string | null>) => {
      state.follow.lastError = action.payload;
      if (action.payload) {
        state.follow.status = 'error';
      }
    },
    requestFollowReconnect: (state) => {
      state.follow.reconnectToken += 1;
      if (state.follow.enabled && state.follow.targetConvId) {
        state.follow.status = 'connecting';
      }
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
  startFollow,
  pauseFollow,
  resumeFollow,
  stopFollow,
  setFollowTarget,
  setFollowStatus,
  setFollowError,
  requestFollowReconnect,
} = uiSlice.actions;

export const selectFollowState = (state: RootState) => state.ui.follow;
export const selectFollowStatus = (state: RootState) => state.ui.follow.status;
export const selectFollowEnabled = (state: RootState) => state.ui.follow.enabled;
export const selectFollowTargetConvId = (state: RootState) => state.ui.follow.targetConvId;
