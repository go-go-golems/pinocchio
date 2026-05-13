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

function parseJsonOrRaw(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}

function mergePropsWithPatches(existingProps: any, incomingProps: any, contentPatch?: string, inputRawPatch?: string): any {
  const merged = { ...(existingProps ?? {}), ...(incomingProps ?? {}) };
  if (contentPatch !== undefined) {
    const previous = typeof existingProps?.content === 'string' ? existingProps.content : '';
    merged.content = previous + contentPatch;
  }
  if (inputRawPatch !== undefined) {
    const previous = typeof existingProps?.inputRaw === 'string' ? existingProps.inputRaw : '';
    const inputRaw = previous + inputRawPatch;
    merged.inputRaw = inputRaw;
    merged.input = parseJsonOrRaw(inputRaw);
  }
  return merged;
}

function mergeTimelineEntity(state: TimelineState, e: TimelineEntity, createIfMissing: boolean) {
  const existing = state.byId[e.id];
  const incomingProps = { ...(e.props ?? {}) };
  const contentPatch = typeof incomingProps.contentPatch === 'string' ? incomingProps.contentPatch : undefined;
  const inputRawPatch = typeof incomingProps.inputRawPatch === 'string' ? incomingProps.inputRawPatch : undefined;
  delete incomingProps.contentPatch;
  delete incomingProps.inputRawPatch;
  if (!existing) {
    if (!createIfMissing) return;
    state.byId[e.id] = { ...e, props: mergePropsWithPatches({}, incomingProps, contentPatch, inputRawPatch) };
    state.order.push(e.id);
    return;
  }
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
      props: mergePropsWithPatches(existing.props, incomingProps, contentPatch, inputRawPatch),
    };
    return;
  }
  if (existingVersion > 0) {
    state.byId[e.id] = {
      ...existing,
      updatedAt: e.updatedAt ?? existing.updatedAt,
      props: mergePropsWithPatches(existing.props, incomingProps, contentPatch, inputRawPatch),
    };
    return;
  }
  state.byId[e.id] = {
    ...existing,
    ...e,
    createdAt: existing.createdAt,
    kind: e.kind || existing.kind,
    props: mergePropsWithPatches(existing.props, incomingProps, contentPatch, inputRawPatch),
  };
}

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
      mergeTimelineEntity(state, action.payload, true);
    },
    upsertEntityIfExists(state, action: PayloadAction<TimelineEntity>) {
      mergeTimelineEntity(state, action.payload, false);
    },
    deleteEntity(state, action: PayloadAction<string>) {
      const id = action.payload;
      if (!id || !state.byId[id]) return;
      delete state.byId[id];
      state.order = state.order.filter((entry) => entry !== id);
    },
    clear(state) {
      state.byId = {};
      state.order = [];
    },
  },
});

export const selectTimelineEntities = (s: RootState) => s.timeline.order.map((id) => s.timeline.byId[id]);
