import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type { RootState } from './store';

export type TimelineEntity = {
  id: string;
  kind: string;
  createdAt: number;
  updatedAt?: number;
  version?: number;
  props: any;
};

type TimelineState = {
  byId: Record<string, TimelineEntity>;
  order: string[];
};

export const timelineSlice = createSlice({
  name: 'timeline',
  initialState: { byId: {}, order: [] } as TimelineState,
  reducers: {
    rekeyEntity(state, action: PayloadAction<{ fromId: string; toId: string }>) {
      const { fromId, toId } = action.payload;
      if (!fromId || !toId || fromId === toId) return;
      const from = state.byId[fromId];
      if (!from) return;
      const existing = state.byId[toId];
      if (existing) {
        state.byId[toId] = {
          ...from,
          ...existing,
          id: toId,
          createdAt: existing.createdAt || from.createdAt,
          updatedAt: existing.updatedAt ?? from.updatedAt,
          version: existing.version ?? from.version,
          props: { ...(from.props ?? {}), ...(existing.props ?? {}) },
        };
      } else {
        state.byId[toId] = { ...from, id: toId };
      }
      delete state.byId[fromId];

      const fromIdx = state.order.indexOf(fromId);
      if (fromIdx >= 0) {
        const toIdx = state.order.indexOf(toId);
        if (toIdx >= 0) {
          state.order.splice(fromIdx, 1);
        } else {
          state.order[fromIdx] = toId;
        }
      }
    },
    addEntity(state, action: PayloadAction<TimelineEntity>) {
      const e = action.payload;
      if (state.byId[e.id]) return;
      state.byId[e.id] = e;
      state.order.push(e.id);
    },
    upsertEntity(state, action: PayloadAction<TimelineEntity>) {
      const e = action.payload;
      if (!state.byId[e.id]) {
        state.byId[e.id] = e;
        state.order.push(e.id);
        return;
      }
      const existing = state.byId[e.id];
      const incomingVersion = typeof e.version === 'number' && Number.isFinite(e.version) ? e.version : 0;
      const existingVersion = typeof existing.version === 'number' && Number.isFinite(existing.version) ? existing.version : 0;
      if (incomingVersion > 0) {
        if (incomingVersion < existingVersion) {
          return;
        }
        state.byId[e.id] = {
          ...existing,
          ...e,
          createdAt: e.createdAt || existing.createdAt,
          kind: e.kind || existing.kind,
          version: incomingVersion,
          props: { ...(existing.props ?? {}), ...(e.props ?? {}) },
        };
        return;
      }
      if (existingVersion > 0) {
        state.byId[e.id] = {
          ...existing,
          updatedAt: e.updatedAt ?? existing.updatedAt,
          props: { ...(existing.props ?? {}), ...(e.props ?? {}) },
        };
        return;
      }
      state.byId[e.id] = {
        ...existing,
        ...e,
        createdAt: existing.createdAt,
        kind: e.kind || existing.kind,
        props: { ...(existing.props ?? {}), ...(e.props ?? {}) },
      };
    },
    clear(state) {
      state.byId = {};
      state.order = [];
    },
  },
});

export const selectTimelineEntities = (s: RootState) => s.timeline.order.map((id) => s.timeline.byId[id]);
