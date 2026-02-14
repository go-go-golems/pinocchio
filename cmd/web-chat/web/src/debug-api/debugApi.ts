import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';
import type {
  ConversationDetail,
  ConversationSummary,
  EventsEnvelope,
  RunDetailEnvelope,
  RunsEnvelope,
  TimelineSnapshotEnvelope,
  TurnDetailEnvelope,
  TurnsEnvelope,
} from '../debug-contract';

export interface TurnsQuery {
  convId: string;
  sessionId?: string;
  phase?: string;
  sinceMs?: number;
  limit?: number;
}

export interface TurnDetailQuery {
  convId: string;
  sessionId: string;
  turnId: string;
}

export interface EventsQuery {
  convId: string;
  type?: string;
  sinceSeq?: number;
  limit?: number;
}

export interface TimelineQuery {
  convId: string;
  sinceVersion?: number;
  limit?: number;
}

export interface RunsQuery {
  artifactsRoot?: string;
  turnsDB?: string;
  timelineDB?: string;
  limit?: number;
}

export interface RunDetailQuery {
  runId: string;
  artifactsRoot?: string;
  turnsDB?: string;
  timelineDB?: string;
  limit?: number;
  sinceVersion?: number;
}

export const debugApi = createApi({
  reducerPath: 'debugApi',
  baseQuery: fetchBaseQuery({ baseUrl: '/api/debug/' }),
  tagTypes: ['DebugConversations', 'DebugTurns', 'DebugEvents', 'DebugTimeline', 'DebugRuns'],
  endpoints: (builder) => ({
    getConversations: builder.query<{ items: ConversationSummary[] }, void>({
      query: () => 'conversations',
      providesTags: ['DebugConversations'],
    }),

    getConversation: builder.query<ConversationDetail, string>({
      query: (convId) => `conversations/${encodeURIComponent(convId)}`,
      providesTags: (_res, _err, convId) => [{ type: 'DebugConversations', id: convId }],
    }),

    getTurns: builder.query<TurnsEnvelope, TurnsQuery>({
      query: ({ convId, sessionId, phase, sinceMs, limit }) => {
        const params = new URLSearchParams();
        params.set('conv_id', convId);
        if (sessionId) params.set('session_id', sessionId);
        if (phase) params.set('phase', phase);
        if (sinceMs !== undefined) params.set('since_ms', String(sinceMs));
        if (limit !== undefined) params.set('limit', String(limit));
        return `turns?${params.toString()}`;
      },
      providesTags: ['DebugTurns'],
    }),

    getTurnDetail: builder.query<TurnDetailEnvelope, TurnDetailQuery>({
      query: ({ convId, sessionId, turnId }) =>
        `turn/${encodeURIComponent(convId)}/${encodeURIComponent(sessionId)}/${encodeURIComponent(turnId)}`,
      providesTags: (_res, _err, q) => [{ type: 'DebugTurns', id: q.turnId }],
    }),

    getEvents: builder.query<EventsEnvelope, EventsQuery>({
      query: ({ convId, type, sinceSeq, limit }) => {
        const params = new URLSearchParams();
        if (type) params.set('type', type);
        if (sinceSeq !== undefined) params.set('since_seq', String(sinceSeq));
        if (limit !== undefined) params.set('limit', String(limit));
        return `events/${encodeURIComponent(convId)}?${params.toString()}`;
      },
      providesTags: ['DebugEvents'],
    }),

    getTimeline: builder.query<TimelineSnapshotEnvelope, TimelineQuery>({
      query: ({ convId, sinceVersion, limit }) => {
        const params = new URLSearchParams();
        params.set('conv_id', convId);
        if (sinceVersion !== undefined) params.set('since_version', String(sinceVersion));
        if (limit !== undefined) params.set('limit', String(limit));
        return `timeline?${params.toString()}`;
      },
      providesTags: ['DebugTimeline'],
    }),

    getRuns: builder.query<RunsEnvelope, RunsQuery>({
      query: ({ artifactsRoot, turnsDB, timelineDB, limit }) => {
        const params = new URLSearchParams();
        if (artifactsRoot) params.set('artifacts_root', artifactsRoot);
        if (turnsDB) params.set('turns_db', turnsDB);
        if (timelineDB) params.set('timeline_db', timelineDB);
        if (limit !== undefined) params.set('limit', String(limit));
        return `runs?${params.toString()}`;
      },
      providesTags: ['DebugRuns'],
    }),

    getRunDetail: builder.query<RunDetailEnvelope, RunDetailQuery>({
      query: ({ runId, artifactsRoot, turnsDB, timelineDB, limit, sinceVersion }) => {
        const params = new URLSearchParams();
        if (artifactsRoot) params.set('artifacts_root', artifactsRoot);
        if (turnsDB) params.set('turns_db', turnsDB);
        if (timelineDB) params.set('timeline_db', timelineDB);
        if (limit !== undefined) params.set('limit', String(limit));
        if (sinceVersion !== undefined) params.set('since_version', String(sinceVersion));
        return `runs/${encodeURIComponent(runId)}?${params.toString()}`;
      },
      providesTags: (_res, _err, q) => [{ type: 'DebugRuns', id: q.runId }],
    }),
  }),
});

export const {
  useGetConversationsQuery,
  useGetConversationQuery,
  useGetTurnsQuery,
  useGetTurnDetailQuery,
  useGetEventsQuery,
  useGetTimelineQuery,
  useGetRunsQuery,
  useGetRunDetailQuery,
} = debugApi;
