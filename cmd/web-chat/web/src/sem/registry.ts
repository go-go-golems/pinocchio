import type { AppDispatch } from '../store/store';
import { timelineSlice, type TimelineEntity } from '../store/timelineSlice';

export type SemEnvelope = { sem: true; event: SemEvent };
export type SemEvent = {
  type: string;
  id: string;
  data?: any;
  metadata?: any;
  seq?: number;
  stream_id?: string;
};

type Handler = (ev: SemEvent, dispatch: AppDispatch) => void;

const handlers = new Map<string, Handler>();

export function registerSem(type: string, handler: Handler) {
  handlers.set(type, handler);
}

export function handleSem(envelope: any, dispatch: AppDispatch) {
  if (!envelope || envelope.sem !== true || !envelope.event) return;
  const ev = envelope.event as SemEvent;
  const h = handlers.get(ev.type);
  if (!h) return;
  h(ev, dispatch);
}

function upsertEntity(dispatch: AppDispatch, entity: TimelineEntity) {
  dispatch(timelineSlice.actions.upsertEntity(entity));
}

function addEntity(dispatch: AppDispatch, entity: TimelineEntity) {
  dispatch(timelineSlice.actions.addEntity(entity));
}

export function registerDefaultSemHandlers() {
  handlers.clear();

  registerSem('llm.start', (ev, dispatch) => {
    const role = ev.data?.role ?? 'assistant';
    addEntity(dispatch, { id: ev.id, kind: 'message', createdAt: Date.now(), props: { role, content: '', streaming: true } });
  });

  registerSem('llm.delta', (ev, dispatch) => {
    const cumulative = ev.data?.cumulative;
    // Backend emits cumulative; prefer it to keep handlers idempotent.
    upsertEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: Date.now(),
      updatedAt: Date.now(),
      props: { content: typeof cumulative === 'string' ? cumulative : '', streaming: true },
    });
  });

  registerSem('llm.final', (ev, dispatch) => {
    upsertEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: Date.now(),
      updatedAt: Date.now(),
      props: { content: ev.data?.text ?? '', streaming: false },
    });
  });

  registerSem('tool.start', (ev, dispatch) => {
    addEntity(dispatch, { id: ev.id, kind: 'tool_call', createdAt: Date.now(), props: { name: ev.data?.name, input: ev.data?.input } });
  });

  registerSem('tool.delta', (ev, dispatch) => {
    upsertEntity(dispatch, { id: ev.id, kind: 'tool_call', createdAt: Date.now(), updatedAt: Date.now(), props: { ...(ev.data?.patch ?? {}) } });
  });

  registerSem('tool.result', (ev, dispatch) => {
    const customKind = ev.data?.customKind;
    const id = customKind ? `${ev.id}:custom` : `${ev.id}:result`;
    upsertEntity(dispatch, { id, kind: 'tool_result', createdAt: Date.now(), updatedAt: Date.now(), props: { result: ev.data?.result, customKind } });
  });

  registerSem('log', (ev, dispatch) => {
    addEntity(dispatch, { id: ev.id, kind: 'log', createdAt: Date.now(), props: { level: ev.data?.level, message: ev.data?.message, fields: ev.data?.fields } });
  });
}
