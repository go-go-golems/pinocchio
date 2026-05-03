import { createSlice, type PayloadAction } from '@reduxjs/toolkit';

export type FollowStatus =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'closed';

interface UiState {
  selectedSessionId: string | null;
  follow: {
    enabled: boolean;
    status: FollowStatus;
  };
}

const initialState: UiState = {
  selectedSessionId: null,
  follow: {
    enabled: false,
    status: 'idle',
  },
};

export const uiSlice = createSlice({
  name: 'ui',
  initialState,
  reducers: {
    selectSession: (state, action: PayloadAction<string | null>) => {
      state.selectedSessionId = action.payload;
    },
    setFollowEnabled: (state, action: PayloadAction<boolean>) => {
      state.follow.enabled = action.payload;
      if (!action.payload) {
        state.follow.status = 'idle';
      }
    },
    setFollowStatus: (state, action: PayloadAction<FollowStatus>) => {
      state.follow.status = action.payload;
    },
  },
});

export const { selectSession, setFollowEnabled, setFollowStatus } = uiSlice.actions;
