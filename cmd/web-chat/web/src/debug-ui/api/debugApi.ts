import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';
import type {
  ConversationDetail,
  ConversationSummary,
  EventsResponse,
  OfflineRunSummary,
  RunDetailResponse,
  RunsResponse,
  TimelineEntity,
  TimelineSnapshot,
  TurnDetail,
  TurnPhase,
  TurnSnapshot,
} from '../types';
import { parseTurnPayload, toParsedTurn } from './turnParsing';

export interface TurnQuery {
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

export interface EventQuery {
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

interface DebugConversationItem {
  conv_id: string;
  session_id: string;
  runtime_key?: string;
  profile?: string;
  active_sockets: number;
  stream_running: boolean;
  queue_depth: number;
  buffered_events: number;
  last_activity_ms: number;
  has_timeline_source: boolean;
}

interface DebugTurnsEnvelope {
  conv_id: string;
  session_id: string;
  phase: string;
  items: Array<{
    conv_id: string;
    session_id: string;
    turn_id: string;
    phase: string;
    created_at_ms: number | string;
    payload: string | Record<string, unknown>;
  }>;
}

interface DebugTurnPhaseItem {
  phase: string;
  created_at_ms: number | string;
  payload: string | Record<string, unknown>;
  parsed?: Record<string, unknown>;
}

interface DebugEventsEnvelope {
  limit: number;
  items: Array<{
    seq: number | string;
    type?: string;
    id?: string;
    frame?: Record<string, unknown>;
  }>;
}

interface DebugRunsEnvelope {
  artifacts_root?: string;
  turns_db?: string;
  timeline_db?: string;
  limit?: number;
  items?: Array<{
    run_id?: string;
    kind?: string;
    display?: string;
    source_path?: string;
    timestamp_ms?: number | string;
    conv_id?: string;
    session_id?: string;
    counts?: Record<string, unknown>;
  }>;
}

interface DebugRunDetailEnvelope {
  run_id?: string;
  kind?: string;
  detail?: Record<string, unknown>;
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function asNumber(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string') {
    const parsed = Number(value.trim());
    return Number.isFinite(parsed) ? parsed : 0;
  }
  if (typeof value === 'bigint') {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

function toIsoFromMs(value: unknown): string {
  const ms = asNumber(value);
  if (ms <= 0) {
    return new Date(0).toISOString();
  }
  return new Date(ms).toISOString();
}

function normalizePhase(raw: string): TurnPhase {
  switch (raw) {
    case 'draft':
      return 'draft';
    case 'pre_inference':
      return 'pre_inference';
    case 'post_inference':
      return 'post_inference';
    case 'post_tools':
      return 'post_tools';
    case 'final':
      return 'final';
    default:
      return 'final';
  }
}

function flattenTimelineProps(entity: Record<string, unknown>): Record<string, unknown> {
  const keys = [
    'message',
    'toolCall',
    'toolResult',
    'status',
    'thinkingMode',
    'modeEvaluation',
    'innerThoughts',
    'teamAnalysis',
    'discoDialogueLine',
    'discoDialogueCheck',
    'discoDialogueState',
  ];

  for (const key of keys) {
    const direct = asRecord(entity[key]);
    if (Object.keys(direct).length > 0) {
      return direct;
    }
  }

  const snapshot = asRecord(entity.snapshot);
  for (const key of keys) {
    const nested = asRecord(snapshot[key]);
    if (Object.keys(nested).length > 0) {
      return nested;
    }
  }

  return asRecord(entity.props);
}

function toTimelineEntity(raw: unknown): TimelineEntity {
  const entity = asRecord(raw);
  return {
    id: asString(entity.id),
    kind: asString(entity.kind),
    created_at: asNumber(entity.createdAtMs ?? entity.created_at_ms ?? entity.createdAt ?? entity.created_at),
    updated_at: asNumber(entity.updatedAtMs ?? entity.updated_at_ms ?? entity.updatedAt ?? entity.updated_at) || undefined,
    version: asNumber(entity.version) || undefined,
    props: flattenTimelineProps(entity),
  };
}

function mapConversation(item: DebugConversationItem): ConversationSummary {
  const runtimeKey = (item.runtime_key ?? item.profile ?? '').trim();
  return {
    id: item.conv_id,
    profile_slug: runtimeKey,
    session_id: item.session_id,
    engine_config_sig: '',
    is_running: item.stream_running,
    ws_connections: item.active_sockets,
    last_activity: toIsoFromMs(item.last_activity_ms),
    turn_count: item.buffered_events,
    has_timeline: item.has_timeline_source,
  };
}

function toRunSummary(raw: unknown): OfflineRunSummary {
  const item = asRecord(raw);
  return {
    run_id: asString(item.run_id),
    kind: asString(item.kind),
    display: asString(item.display),
    source_path: asString(item.source_path),
    timestamp_ms: asNumber(item.timestamp_ms) || undefined,
    conv_id: asString(item.conv_id) || undefined,
    session_id: asString(item.session_id) || undefined,
    counts: asRecord(item.counts),
  };
}

export const debugApi = createApi({
  reducerPath: 'debugApi',
  baseQuery: fetchBaseQuery({ baseUrl: '/api/debug/' }),
  tagTypes: ['Conversations', 'Turns', 'Events', 'Timeline', 'Runs'],
  endpoints: (builder) => ({
    getConversations: builder.query<ConversationSummary[], void>({
      query: () => 'conversations',
      transformResponse: (response: { items?: DebugConversationItem[]; conversations?: ConversationSummary[] }) => {
        if (Array.isArray(response.items)) {
          return response.items.map(mapConversation);
        }
        if (Array.isArray(response.conversations)) {
          return response.conversations;
        }
        return [];
      },
      providesTags: ['Conversations'],
    }),

    getConversation: builder.query<ConversationDetail, string>({
      query: (id) => `conversations/${encodeURIComponent(id)}`,
      transformResponse: (response: DebugConversationItem & { active_request_key?: string }) => {
        const mapped = mapConversation(response);
        return {
          ...mapped,
          engine_config: {
            profile_slug: mapped.profile_slug,
            system_prompt: '',
            middlewares: [],
            tools: [],
          },
        };
      },
      providesTags: (_result, _error, id) => [{ type: 'Conversations', id }],
    }),

    getTurns: builder.query<TurnSnapshot[], TurnQuery>({
      query: ({ convId, sessionId, phase, sinceMs, limit }) => {
        const params = new URLSearchParams();
        if (convId) params.set('conv_id', convId);
        if (sessionId) params.set('session_id', sessionId);
        if (phase) params.set('phase', phase);
        if (sinceMs !== undefined) params.set('since_ms', String(sinceMs));
        if (limit !== undefined) params.set('limit', String(limit));
        return `turns?${params.toString()}`;
      },
      transformResponse: (response: DebugTurnsEnvelope | TurnSnapshot[]) => {
        if (Array.isArray(response)) {
          return response;
        }
        return (response.items ?? []).map((item) => ({
          conv_id: item.conv_id,
          session_id: item.session_id,
          turn_id: item.turn_id,
          phase: normalizePhase(item.phase),
          created_at_ms: asNumber(item.created_at_ms),
          turn: parseTurnPayload(item.payload, item.turn_id),
        }));
      },
      providesTags: ['Turns'],
    }),

    getTurnDetail: builder.query<TurnDetail, TurnDetailQuery>({
      query: ({ convId, sessionId, turnId }) =>
        `turn/${encodeURIComponent(convId)}/${encodeURIComponent(sessionId)}/${encodeURIComponent(turnId)}`,
      transformResponse: (response: {
        conv_id: string;
        session_id: string;
        turn_id: string;
        items?: DebugTurnPhaseItem[];
        phases?: TurnDetail['phases'];
      }) => {
        if (response.phases) {
          return {
            conv_id: response.conv_id,
            session_id: response.session_id,
            turn_id: response.turn_id,
            phases: response.phases,
          };
        }
        const phases: TurnDetail['phases'] = {};
        for (const item of response.items ?? []) {
          const phase = normalizePhase(item.phase);
          let parsed = parseTurnPayload(item.payload, response.turn_id);
          if (parsed.blocks.length === 0 && item.parsed) {
            parsed = toParsedTurn(item.parsed, response.turn_id);
          }
          phases[phase] = {
            captured_at: toIsoFromMs(item.created_at_ms),
            turn: parsed,
          };
        }
        return {
          conv_id: response.conv_id,
          session_id: response.session_id,
          turn_id: response.turn_id,
          phases,
        };
      },
      providesTags: (_result, _error, { turnId }) => [{ type: 'Turns', id: turnId }],
    }),

    getEvents: builder.query<EventsResponse, EventQuery>({
      query: ({ convId, type, sinceSeq, limit }) => {
        const params = new URLSearchParams();
        if (type) params.set('type', type);
        if (sinceSeq !== undefined) params.set('since_seq', String(sinceSeq));
        if (limit !== undefined) params.set('limit', String(limit));
        return `events/${encodeURIComponent(convId)}?${params.toString()}`;
      },
      transformResponse: (response: DebugEventsEnvelope | EventsResponse): EventsResponse => {
        if (Array.isArray((response as EventsResponse).events)) {
          return response as EventsResponse;
        }
        const envelope = response as DebugEventsEnvelope;
        const nowISO = new Date().toISOString();
        const events = (envelope.items ?? []).map((item) => {
          const frame = asRecord(item.frame);
          const event = asRecord(frame.event);
          const eventType = asString(item.type) || asString(event.type);
          const eventID = asString(item.id) || asString(event.id);
          return {
            type: eventType,
            id: eventID,
            seq: asNumber(item.seq),
            stream_id: asString(event.stream_id ?? event.streamId) || undefined,
            data: event.data ?? {},
            received_at: nowISO,
          };
        });
        return {
          events,
          total: events.length,
          buffer_capacity: asNumber(envelope.limit) || events.length,
        };
      },
      providesTags: ['Events'],
    }),

    getTimeline: builder.query<TimelineSnapshot, TimelineQuery>({
      query: ({ convId, sinceVersion, limit }) => {
        const params = new URLSearchParams();
        params.set('conv_id', convId);
        if (sinceVersion !== undefined) params.set('since_version', String(sinceVersion));
        if (limit !== undefined) params.set('limit', String(limit));
        return `timeline?${params.toString()}`;
      },
      transformResponse: (response: { entities?: unknown[]; version?: number }): TimelineSnapshot => ({
        entities: (response.entities ?? []).map(toTimelineEntity),
        version: asNumber(response.version),
      }),
      providesTags: ['Timeline'],
    }),

    getRuns: builder.query<RunsResponse, RunsQuery>({
      query: ({ artifactsRoot, turnsDB, timelineDB, limit }) => {
        const params = new URLSearchParams();
        if (artifactsRoot) params.set('artifacts_root', artifactsRoot);
        if (turnsDB) params.set('turns_db', turnsDB);
        if (timelineDB) params.set('timeline_db', timelineDB);
        if (limit !== undefined) params.set('limit', String(limit));
        return `runs?${params.toString()}`;
      },
      transformResponse: (response: DebugRunsEnvelope): RunsResponse => ({
        artifacts_root: asString(response.artifacts_root) || undefined,
        turns_db: asString(response.turns_db) || undefined,
        timeline_db: asString(response.timeline_db) || undefined,
        limit: asNumber(response.limit) || 0,
        items: (response.items ?? []).map(toRunSummary),
      }),
      providesTags: ['Runs'],
    }),

    getRunDetail: builder.query<RunDetailResponse, RunDetailQuery>({
      query: ({ runId, artifactsRoot, turnsDB, timelineDB, limit, sinceVersion }) => {
        const params = new URLSearchParams();
        if (artifactsRoot) params.set('artifacts_root', artifactsRoot);
        if (turnsDB) params.set('turns_db', turnsDB);
        if (timelineDB) params.set('timeline_db', timelineDB);
        if (limit !== undefined) params.set('limit', String(limit));
        if (sinceVersion !== undefined) params.set('since_version', String(sinceVersion));
        return `runs/${encodeURIComponent(runId)}?${params.toString()}`;
      },
      transformResponse: (response: DebugRunDetailEnvelope): RunDetailResponse => ({
        run_id: asString(response.run_id),
        kind: asString(response.kind),
        detail: asRecord(response.detail),
      }),
      providesTags: (_result, _error, { runId }) => [{ type: 'Runs', id: runId }],
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
