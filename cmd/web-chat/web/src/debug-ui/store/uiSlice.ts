import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type { Anomaly, TurnPhase } from '../types';

interface UiState {
  // Selection state
  selectedConvId: string | null;
  selectedSessionId: string | null;
  selectedTurnId: string | null;
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

  // View state
  viewMode: 'semantic' | 'sem' | 'raw';
  liveStreamEnabled: boolean;
  sidebarCollapsed: boolean;
  filterBarOpen: boolean;
  anomalyPanelOpen: boolean;
  inspectorPanel: 'none' | 'event' | 'turn' | 'mw';

  // Filters
  filters: {
    eventTypes: string[];
    snapshotPhases: string[];
    middlewares: string[];
    blockKinds: string[];
  };

  // Anomalies
  pinnedAnomalies: Anomaly[];
  autoDetectedAnomalies: Anomaly[];
}

const initialState: UiState = {
  selectedConvId: null,
  selectedSessionId: null,
  selectedTurnId: null,
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

  viewMode: 'semantic',
  liveStreamEnabled: true,
  sidebarCollapsed: false,
  filterBarOpen: false,
  anomalyPanelOpen: false,
  inspectorPanel: 'none',

  filters: {
    eventTypes: [],
    snapshotPhases: [],
    middlewares: [],
    blockKinds: [],
  },

  pinnedAnomalies: [],
  autoDetectedAnomalies: [],
};

export const uiSlice = createSlice({
  name: 'ui',
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
    setViewMode: (state, action: PayloadAction<'semantic' | 'sem' | 'raw'>) => {
      state.viewMode = action.payload;
    },
    toggleLiveStream: (state) => {
      state.liveStreamEnabled = !state.liveStreamEnabled;
    },
    toggleSidebar: (state) => {
      state.sidebarCollapsed = !state.sidebarCollapsed;
    },
    toggleFilterBar: (state) => {
      state.filterBarOpen = !state.filterBarOpen;
    },
    toggleAnomalyPanel: (state) => {
      state.anomalyPanelOpen = !state.anomalyPanelOpen;
    },
    setInspectorPanel: (
      state,
      action: PayloadAction<'none' | 'event' | 'turn' | 'mw'>
    ) => {
      state.inspectorPanel = action.payload;
    },
    setFilters: (
      state,
      action: PayloadAction<Partial<UiState['filters']>>
    ) => {
      state.filters = { ...state.filters, ...action.payload };
    },
    pinAnomaly: (state, action: PayloadAction<Anomaly>) => {
      state.pinnedAnomalies.push(action.payload);
    },
    unpinAnomaly: (state, action: PayloadAction<string>) => {
      state.pinnedAnomalies = state.pinnedAnomalies.filter(
        (a) => a.id !== action.payload
      );
    },
    setAutoDetectedAnomalies: (state, action: PayloadAction<Anomaly[]>) => {
      state.autoDetectedAnomalies = action.payload;
    },
  },
});

export const {
  selectConversation,
  selectSession,
  selectTurn,
  selectRun,
  setOfflineConfig,
  selectPhase,
  selectEvent,
  setComparePhases,
  setViewMode,
  toggleLiveStream,
  toggleSidebar,
  toggleFilterBar,
  toggleAnomalyPanel,
  setInspectorPanel,
  setFilters,
  pinAnomaly,
  unpinAnomaly,
  setAutoDetectedAnomalies,
} = uiSlice.actions;
