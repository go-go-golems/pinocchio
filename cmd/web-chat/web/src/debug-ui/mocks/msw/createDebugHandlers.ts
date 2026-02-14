import { HttpResponse, http } from 'msw';
import type {
  ConversationDetail,
  ConversationSummary,
  MwTrace,
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
}

export interface CreateDebugHandlersOptions {
  data: DebugHandlerData;
  nowMs?: () => number;
  nowIso?: () => string;
  delayMs?: {
    conversations?: number;
  };
}

export function createDebugHandlers(options: CreateDebugHandlersOptions) {
  const {
    data,
    nowMs = () => Date.now(),
    nowIso = () => new Date().toISOString(),
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
  } = data;

  return [
    http.get('/api/debug/conversations', async () => {
      if (conversationsDelayMs > 0) {
        await new Promise((resolve) => setTimeout(resolve, conversationsDelayMs));
      }
      return HttpResponse.json({ conversations });
    }),

    http.get('/api/debug/conversations/:id', ({ params }) => {
      const { id } = params;
      const conversationId = String(id ?? '');

      if (conversationId === conversationDetail.id) {
        return HttpResponse.json(conversationDetail);
      }

      const summary = conversations.find((conversation) => conversation.id === conversationId);
      if (!summary) {
        return HttpResponse.json({ error: 'Conversation not found' }, { status: 404 });
      }

      return HttpResponse.json(
        {
          ...summary,
          engine_config: conversationDetail.engine_config,
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

      let filtered = turns;
      if (convId) {
        filtered = filtered.filter((turn) => turn.conv_id === convId);
      }
      if (sessionId) {
        filtered = filtered.filter((turn) => turn.session_id === sessionId);
      }
      return HttpResponse.json(filtered);
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
        phases: {
          final: { captured_at: nowIso(), turn: turn.turn },
        },
      });
    }),

    http.get('/api/debug/events/:convId', () => {
      return HttpResponse.json({
        events,
        total: events.length,
        buffer_capacity: 1000,
      });
    }),

    http.get('/api/debug/timeline', () => {
      return HttpResponse.json({
        entities: timelineEntities,
        version: nowMs(),
      });
    }),

    http.get('/api/debug/mw-trace/:convId/:inferenceId', () => {
      return HttpResponse.json(mwTrace);
    }),
  ];
}
