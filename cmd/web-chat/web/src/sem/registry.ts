import type { AppDispatch } from '../store/store';
import { timelineSlice, type TimelineEntity } from '../store/timelineSlice';
import { fromJson, type Message } from '@bufbuild/protobuf';
import type { GenMessage } from '@bufbuild/protobuf/codegenv2';

import { LlmDeltaSchema, LlmDoneSchema, LlmFinalSchema, LlmStartSchema, type LlmDelta, type LlmFinal, type LlmStart } from '../sem/pb/proto/sem/base/llm_pb';
import { ToolDeltaSchema, ToolDoneSchema, ToolResultSchema, ToolStartSchema, type ToolDelta, type ToolResult, type ToolStart } from '../sem/pb/proto/sem/base/tool_pb';
import { LogV1Schema, type LogV1 } from '../sem/pb/proto/sem/base/log_pb';
import { AgentModeV1Schema, type AgentModeV1 } from '../sem/pb/proto/sem/base/agent_pb';
import { DebuggerPauseV1Schema, type DebuggerPauseV1 } from '../sem/pb/proto/sem/base/debugger_pb';

export type SemEnvelope = { sem: true; event: SemEvent };
export type SemEvent = {
  type: string;
  id: string;
  data?: unknown;
  metadata?: unknown;
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

function createdAtFromEvent(ev: SemEvent): number {
  if (typeof ev.seq === 'number' && Number.isFinite(ev.seq)) return ev.seq;
  return Date.now();
}

function decodeProto<T extends Message>(schema: GenMessage<T>, raw: unknown): T | null {
  if (!raw || typeof raw !== 'object') return null;
  try {
    return fromJson(schema as any, raw as any, { ignoreUnknownFields: true }) as T;
  } catch {
    return null;
  }
}

export function registerDefaultSemHandlers() {
  handlers.clear();

  registerSem('llm.start', (ev, dispatch) => {
    const data = decodeProto<LlmStart>(LlmStartSchema, ev.data);
    const role = data?.role || 'assistant';
    addEntity(dispatch, { id: ev.id, kind: 'message', createdAt: createdAtFromEvent(ev), props: { role, content: '', streaming: true } });
  });

  registerSem('llm.delta', (ev, dispatch) => {
    const data = decodeProto<LlmDelta>(LlmDeltaSchema, ev.data);
    const cumulative = data?.cumulative;
    // Backend emits cumulative; prefer it to keep handlers idempotent.
    upsertEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { content: typeof cumulative === 'string' ? cumulative : '', streaming: true },
    });
  });

  registerSem('llm.final', (ev, dispatch) => {
    const data = decodeProto<LlmFinal>(LlmFinalSchema, ev.data);
    upsertEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { content: data?.text ?? '', streaming: false },
    });
  });

  registerSem('llm.thinking.start', (ev, dispatch) => {
    const data = decodeProto<LlmStart>(LlmStartSchema, ev.data);
    const role = data?.role || 'thinking';
    addEntity(dispatch, { id: ev.id, kind: 'message', createdAt: createdAtFromEvent(ev), props: { role, content: '', streaming: true } });
  });

  registerSem('llm.thinking.delta', (ev, dispatch) => {
    const data = decodeProto<LlmDelta>(LlmDeltaSchema, ev.data);
    const cumulative = data?.cumulative;
    upsertEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { content: typeof cumulative === 'string' ? cumulative : '', streaming: true },
    });
  });

  registerSem('llm.thinking.final', (ev, dispatch) => {
    const _data = decodeProto(LlmDoneSchema, ev.data);
    upsertEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { streaming: false },
    });
  });

  registerSem('tool.start', (ev, dispatch) => {
    const data = decodeProto<ToolStart>(ToolStartSchema, ev.data);
    addEntity(dispatch, { id: ev.id, kind: 'tool_call', createdAt: createdAtFromEvent(ev), props: { name: data?.name, input: data?.input } });
  });

  registerSem('tool.delta', (ev, dispatch) => {
    const data = decodeProto<ToolDelta>(ToolDeltaSchema, ev.data);
    upsertEntity(dispatch, { id: ev.id, kind: 'tool_call', createdAt: createdAtFromEvent(ev), updatedAt: Date.now(), props: { ...(data?.patch ?? {}) } });
  });

  registerSem('tool.result', (ev, dispatch) => {
    const data = decodeProto<ToolResult>(ToolResultSchema, ev.data);
    const customKind = data?.customKind;
    const id = customKind ? `${ev.id}:custom` : `${ev.id}:result`;
    upsertEntity(dispatch, { id, kind: 'tool_result', createdAt: createdAtFromEvent(ev), updatedAt: Date.now(), props: { result: data?.result, customKind } });
  });

  registerSem('tool.done', (ev, dispatch) => {
    const _data = decodeProto(ToolDoneSchema, ev.data);
    upsertEntity(dispatch, { id: ev.id, kind: 'tool_call', createdAt: createdAtFromEvent(ev), updatedAt: Date.now(), props: { done: true } });
  });

  registerSem('log', (ev, dispatch) => {
    const data = decodeProto<LogV1>(LogV1Schema, ev.data);
    addEntity(dispatch, {
      id: ev.id,
      kind: 'log',
      createdAt: createdAtFromEvent(ev),
      props: { level: data?.level, message: data?.message, fields: data?.fields ?? {} },
    });
  });

  registerSem('agent.mode', (ev, dispatch) => {
    const data = decodeProto<AgentModeV1>(AgentModeV1Schema, ev.data);
    upsertEntity(dispatch, {
      id: ev.id,
      kind: 'agent_mode',
      createdAt: createdAtFromEvent(ev),
      props: { title: data?.title, data: data?.data ?? {} },
    });
  });

  registerSem('debugger.pause', (ev, dispatch) => {
    const data = decodeProto<DebuggerPauseV1>(DebuggerPauseV1Schema, ev.data);
    upsertEntity(dispatch, {
      id: ev.id,
      kind: 'debugger_pause',
      createdAt: createdAtFromEvent(ev),
      props: {
        pauseId: data?.pauseId,
        phase: data?.phase,
        summary: data?.summary,
        deadlineMs: data?.deadlineMs?.toString?.() ?? '',
        extra: data?.extra ?? {},
      },
    });
  });
}
