import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';
import { parse as parseYAML } from 'yaml';
import type {
  BlockKind,
  ConversationDetail,
  ConversationSummary,
  EventsResponse,
  ParsedBlock,
  ParsedTurn,
  TimelineEntity,
  TimelineSnapshot,
  TurnDetail,
  TurnPhase,
  TurnSnapshot,
} from '../types';

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

interface DebugConversationItem {
  conv_id: string;
  session_id: string;
  profile: string;
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
    created_at_ms: number;
    payload: string;
  }>;
}

interface DebugTurnPhaseItem {
  phase: string;
  created_at_ms: number;
  payload: string;
  parsed?: Record<string, unknown>;
}

interface DebugEventsEnvelope {
  limit: number;
  items: Array<{
    seq: number;
    type?: string;
    id?: string;
    frame?: Record<string, unknown>;
  }>;
}

const BLOCK_KINDS = new Set<BlockKind>([
  'system',
  'user',
  'llm_text',
  'tool_call',
  'tool_use',
  'reasoning',
  'other',
]);

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function asNumber(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
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

function toBlockKind(raw: unknown): BlockKind {
  const kind = asString(raw);
  if (BLOCK_KINDS.has(kind as BlockKind)) {
    return kind as BlockKind;
  }
  return 'other';
}

function toParsedBlock(raw: unknown, index: number): ParsedBlock {
  const obj = asRecord(raw);
  return {
    index,
    id: asString(obj.id) || undefined,
    kind: toBlockKind(obj.kind),
    role: asString(obj.role) || undefined,
    payload: asRecord(obj.payload),
    metadata: asRecord(obj.metadata),
  };
}

function toParsedTurn(raw: unknown, fallbackID = ''): ParsedTurn {
  const obj = asRecord(raw);
  const blocksRaw = Array.isArray(obj.blocks) ? obj.blocks : [];
  return {
    id: asString(obj.id) || fallbackID,
    blocks: blocksRaw.map((block, idx) => toParsedBlock(block, idx)),
    metadata: asRecord(obj.metadata),
    data: asRecord(obj.data),
  };
}

function parseTurnPayload(payload: string, fallbackID = ''): ParsedTurn {
  if (!payload.trim()) {
    return { id: fallbackID, blocks: [], metadata: {}, data: {} };
  }
  try {
    return toParsedTurn(parseYAML(payload), fallbackID);
  } catch {
    return { id: fallbackID, blocks: [], metadata: {}, data: {} };
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
    created_at: asNumber(entity.createdAtMs ?? entity.created_at_ms),
    updated_at: asNumber(entity.updatedAtMs ?? entity.updated_at_ms) || undefined,
    version: asNumber(entity.version) || undefined,
    props: flattenTimelineProps(entity),
  };
}

function mapConversation(item: DebugConversationItem): ConversationSummary {
  return {
    id: item.conv_id,
    profile_slug: item.profile,
    session_id: item.session_id,
    engine_config_sig: '',
    is_running: item.stream_running,
    ws_connections: item.active_sockets,
    last_activity: toIsoFromMs(item.last_activity_ms),
    turn_count: item.buffered_events,
    has_timeline: item.has_timeline_source,
  };
}

export const debugApi = createApi({
  reducerPath: 'debugApi',
  baseQuery: fetchBaseQuery({ baseUrl: '/api/debug/' }),
  tagTypes: ['Conversations', 'Turns', 'Events', 'Timeline'],
  endpoints: (builder) => ({
    getConversations: builder.query<ConversationSummary[], void>({
      query: () => 'conversations',
      transformResponse: (response: { items?: DebugConversationItem[] }) =>
        (response.items ?? []).map(mapConversation),
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
      transformResponse: (response: DebugTurnsEnvelope) =>
        (response.items ?? []).map((item) => ({
          conv_id: item.conv_id,
          session_id: item.session_id,
          turn_id: item.turn_id,
          phase: normalizePhase(item.phase),
          created_at_ms: item.created_at_ms,
          turn: parseTurnPayload(item.payload, item.turn_id),
        })),
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
      }) => {
        const phases: TurnDetail['phases'] = {};
        for (const item of response.items ?? []) {
          const phase = normalizePhase(item.phase);
          const parsed = item.parsed
            ? toParsedTurn(item.parsed, response.turn_id)
            : parseTurnPayload(item.payload, response.turn_id);
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
      transformResponse: (response: DebugEventsEnvelope): EventsResponse => {
        const nowISO = new Date().toISOString();
        const events = (response.items ?? []).map((item) => {
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
          buffer_capacity: asNumber(response.limit) || events.length,
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
  }),
});

export const {
  useGetConversationsQuery,
  useGetConversationQuery,
  useGetTurnsQuery,
  useGetTurnDetailQuery,
  useGetEventsQuery,
  useGetTimelineQuery,
} = debugApi;
