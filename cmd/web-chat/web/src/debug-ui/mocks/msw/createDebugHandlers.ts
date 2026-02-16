import { HttpResponse, http } from 'msw';
import type {
  ConversationDetail,
  ConversationSummary,
  MwTrace,
  OfflineRunSummary,
  RunDetailResponse,
  SemEvent,
  SessionSummary,
  TimelineEntity,
  TurnDetail,
  TurnSnapshot,
} from '../../types';

export interface DebugHandlerData {
  conversations: ConversationSummary[];
  conversationDetail: ConversationDetail;
  sessions: SessionSummary[];
  turns: TurnSnapshot[];
  turnDetail: TurnDetail;
  events: SemEvent[];
  timelineEntities: TimelineEntity[];
  mwTrace: MwTrace;
  offlineRuns: OfflineRunSummary[];
  runDetails: Record<string, RunDetailResponse>;
}

export interface CreateDebugHandlersOptions {
  data: DebugHandlerData;
  nowMs?: () => number;
  delayMs?: {
    conversations?: number;
  };
}

export function createDebugHandlers(options: CreateDebugHandlersOptions) {
  const {
    data,
    nowMs = () => Date.now(),
    delayMs,
  } = options;
  const conversationsDelayMs = Math.max(0, delayMs?.conversations ?? 0);
  const {
    conversations,
    conversationDetail,
    sessions,
    turns,
    turnDetail,
    events,
    timelineEntities,
    mwTrace,
    offlineRuns,
    runDetails,
  } = data;

  return [
    http.get('/api/debug/conversations', async () => {
      if (conversationsDelayMs > 0) {
        await new Promise((resolve) => setTimeout(resolve, conversationsDelayMs));
      }
      return HttpResponse.json({
        items: conversations.map((conversation) => ({
          conv_id: conversation.id,
          session_id: conversation.session_id,
          runtime_key: conversation.profile_slug,
          active_sockets: conversation.ws_connections,
          stream_running: conversation.is_running,
          queue_depth: 0,
          buffered_events: conversation.turn_count,
          last_activity_ms: Date.parse(conversation.last_activity),
          has_timeline_source: conversation.has_timeline,
        })),
      });
    }),

    http.get('/api/debug/conversations/:id', ({ params }) => {
      const { id } = params;
      const conversationId = String(id ?? '');

      const summary = conversations.find((conversation) => conversation.id === conversationId);
      if (!summary) {
        return HttpResponse.json({ error: 'Conversation not found' }, { status: 404 });
      }

      return HttpResponse.json(
        {
          conv_id: summary.id,
          session_id: summary.session_id,
          runtime_key: summary.profile_slug,
          active_sockets: summary.ws_connections,
          stream_running: summary.is_running,
          queue_depth: 0,
          buffered_events: summary.turn_count,
          last_activity_ms: Date.parse(summary.last_activity),
          has_timeline_source: summary.has_timeline,
          active_request_key: conversationId === conversationDetail.id ? 'req-001' : '',
        },
        { status: 200 }
      );
    }),

    http.get('/api/debug/conversations/:id/sessions', () => {
      return HttpResponse.json({ sessions });
    }),

    http.get('/api/debug/turns', ({ request }) => {
      const url = new URL(request.url);
      const convId = url.searchParams.get('conv_id');
      const sessionId = url.searchParams.get('session_id');
      const phase = url.searchParams.get('phase') ?? 'final';
      const sinceMs = Number(url.searchParams.get('since_ms') ?? '0');
      const limit = Number(url.searchParams.get('limit') ?? '200');

      let filtered = turns;
      if (convId) {
        filtered = filtered.filter((turn) => turn.conv_id === convId);
      }
      if (sessionId) {
        filtered = filtered.filter((turn) => turn.session_id === sessionId);
      }
      if (Number.isFinite(sinceMs) && sinceMs > 0) {
        filtered = filtered.filter((turn) => turn.created_at_ms >= sinceMs);
      }
      filtered = filtered.slice(0, Math.max(1, limit));

      return HttpResponse.json({
        conv_id: convId ?? filtered[0]?.conv_id ?? '',
        session_id: sessionId ?? filtered[0]?.session_id ?? '',
        phase,
        since_ms: sinceMs,
        items: filtered.map((turn) => ({
          conv_id: turn.conv_id,
          session_id: turn.session_id,
          turn_id: turn.turn_id,
          phase: turn.phase,
          created_at_ms: turn.created_at_ms,
          payload: turn.turn,
        })),
      });
    }),

    http.get('/api/debug/turn/:convId/:sessionId/:turnId', ({ params }) => {
      const turnId = String(params.turnId ?? '');
      if (turnId === turnDetail.turn_id) {
        return HttpResponse.json(turnDetail);
      }

      const turn = turns.find((candidate) => candidate.turn_id === turnId);
      if (!turn) {
        return HttpResponse.json({ error: 'Turn not found' }, { status: 404 });
      }

      return HttpResponse.json({
        conv_id: turn.conv_id,
        session_id: turn.session_id,
        turn_id: turn.turn_id,
        items: [
          {
            phase: 'final',
            created_at_ms: turn.created_at_ms,
            payload: turn.turn,
            parsed: turn.turn,
          },
        ],
      });
    }),

    http.get('/api/debug/events/:convId', ({ request }) => {
      const url = new URL(request.url);
      const type = url.searchParams.get('type');
      const sinceSeq = Number(url.searchParams.get('since_seq') ?? '0');
      const limit = Number(url.searchParams.get('limit') ?? '1000');

      let filtered = events;
      if (type) {
        filtered = filtered.filter((event) => event.type === type);
      }
      if (Number.isFinite(sinceSeq) && sinceSeq > 0) {
        filtered = filtered.filter((event) => event.seq >= sinceSeq);
      }
      filtered = filtered.slice(0, Math.max(1, limit));

      return HttpResponse.json({
        limit,
        items: filtered.map((event) => ({
          seq: String(event.seq),
          type: event.type,
          id: event.id,
          frame: {
            event: {
              id: event.id,
              type: event.type,
              streamId: event.stream_id,
              data: event.data,
              receivedAt: event.received_at,
            },
          },
        })),
      });
    }),

    http.get('/api/debug/timeline', () => {
      return HttpResponse.json({
        entities: timelineEntities,
        version: nowMs(),
      });
    }),

    http.get('/api/debug/runs', ({ request }) => {
      const url = new URL(request.url);
      const artifactsRoot = url.searchParams.get('artifacts_root');
      const turnsDB = url.searchParams.get('turns_db');
      const timelineDB = url.searchParams.get('timeline_db');
      const limit = Number(url.searchParams.get('limit') ?? '200');

      if (!artifactsRoot && !turnsDB && !timelineDB) {
        return HttpResponse.json(
          { error: 'at least one source is required (artifacts_root, turns_db, timeline_db)' },
          { status: 400 }
        );
      }

      const filtered = offlineRuns
        .filter((run) => {
          if (run.kind === 'artifact') return !!artifactsRoot;
          if (run.kind === 'turns') return !!turnsDB;
          if (run.kind === 'timeline') return !!timelineDB;
          return true;
        })
        .slice(0, Math.max(1, limit));

      return HttpResponse.json({
        artifacts_root: artifactsRoot ?? '',
        turns_db: turnsDB ?? '',
        timeline_db: timelineDB ?? '',
        limit,
        items: filtered,
      });
    }),

    http.get('/api/debug/runs/:runId', ({ params }) => {
      const runId = decodeURIComponent(String(params.runId ?? ''));
      const detail = runDetails[runId];
      if (!detail) {
        return HttpResponse.json({ error: 'run not found' }, { status: 404 });
      }
      return HttpResponse.json(detail);
    }),

    http.get('/api/debug/mw-trace/:convId/:inferenceId', () => {
      return HttpResponse.json(mwTrace);
    }),
  ];
}
