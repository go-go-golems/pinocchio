import { fromJson, type Message } from '@bufbuild/protobuf';
import type { GenMessage } from '@bufbuild/protobuf/codegenv2';
import { type AgentModeV1, AgentModeV1Schema } from '../sem/pb/proto/sem/base/agent_pb';
import { type DebuggerPauseV1, DebuggerPauseV1Schema } from '../sem/pb/proto/sem/base/debugger_pb';

import { type LlmDelta, LlmDeltaSchema, LlmDoneSchema, type LlmFinal, LlmFinalSchema, type LlmStart, LlmStartSchema } from '../sem/pb/proto/sem/base/llm_pb';
import { type LogV1, LogV1Schema } from '../sem/pb/proto/sem/base/log_pb';
import { type ToolDelta, ToolDeltaSchema, ToolDoneSchema, type ToolResult, ToolResultSchema, type ToolStart, ToolStartSchema } from '../sem/pb/proto/sem/base/tool_pb';
import { type TimelineUpsertV2, TimelineUpsertV2Schema } from '../sem/pb/proto/sem/timeline/transport_pb';
import type { AppDispatch } from '../store/store';
import { type TimelineEntity, timelineSlice } from '../store/timelineSlice';
import { timelineEntityFromProto } from './timelineMapper';

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
type TimelineMessageState = { emitted: boolean };

const handlers = new Map<string, Handler>();
const timelineMessageStates = new Map<string, TimelineMessageState>();

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

function createdAtFromEvent(_ev: SemEvent): number {
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

function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function toVisibleText(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  if (value.trim().length === 0) return null;
  return value;
}

function pruneEmptyTimelineMessageUpsert(entity: TimelineEntity): TimelineEntity | null {
  if (entity.kind !== 'message') return entity;
  const props = asRecord(entity.props);
  const text = toVisibleText(props.content);
  const streaming = props.streaming === true;
  const state = timelineMessageStates.get(entity.id);

  if (text) {
    timelineMessageStates.set(entity.id, { emitted: true });
    return entity;
  }

  if (!state?.emitted) {
    if (!streaming) {
      timelineMessageStates.delete(entity.id);
    }
    return null;
  }

  const nextProps = { ...props };
  delete nextProps.content;
  if (!streaming) {
    timelineMessageStates.delete(entity.id);
  }

  return {
    ...entity,
    props: nextProps,
  };
}

function upsertTimelineEntity(dispatch: AppDispatch, entity: TimelineEntity) {
  const normalized = pruneEmptyTimelineMessageUpsert(entity);
  if (!normalized) return;
  upsertEntity(dispatch, normalized);
}

export function registerDefaultSemHandlers() {
  handlers.clear();
  timelineMessageStates.clear();

  registerSem('timeline.upsert', (ev, dispatch) => {
    const data = decodeProto<TimelineUpsertV2>(TimelineUpsertV2Schema, ev.data);
    const entity = data?.entity;
    if (!entity) return;
    const mapped = timelineEntityFromProto(entity, data?.version);
    if (!mapped) return;
    upsertTimelineEntity(dispatch, mapped);
  });

  registerSem('llm.start', (ev, dispatch) => {
    decodeProto<LlmStart>(LlmStartSchema, ev.data);
  });

  registerSem('llm.delta', (ev, dispatch) => {
    const data = decodeProto<LlmDelta>(LlmDeltaSchema, ev.data);
    // Backend emits cumulative; prefer it to keep handlers idempotent.
    const content = toVisibleText(data?.cumulative) ?? toVisibleText(data?.delta);
    if (!content) return;
    upsertTimelineEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { role: 'assistant', content, streaming: true },
    });
  });

  registerSem('llm.final', (ev, dispatch) => {
    const data = decodeProto<LlmFinal>(LlmFinalSchema, ev.data);
    upsertTimelineEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { role: 'assistant', content: data?.text ?? '', streaming: false },
    });
  });

  registerSem('llm.thinking.start', (ev, dispatch) => {
    decodeProto<LlmStart>(LlmStartSchema, ev.data);
  });

  registerSem('llm.thinking.delta', (ev, dispatch) => {
    const data = decodeProto<LlmDelta>(LlmDeltaSchema, ev.data);
    const content = toVisibleText(data?.cumulative) ?? toVisibleText(data?.delta);
    if (!content) return;
    upsertTimelineEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { role: 'thinking', content, streaming: true },
    });
  });

  registerSem('llm.thinking.final', (ev, dispatch) => {
    const _data = decodeProto(LlmDoneSchema, ev.data);
    upsertTimelineEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { role: 'thinking', streaming: false },
    });
  });

  registerSem('llm.thinking.summary', (ev, dispatch) => {
    const data = decodeProto<LlmFinal>(LlmFinalSchema, ev.data);
    upsertTimelineEntity(dispatch, {
      id: ev.id,
      kind: 'message',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { role: 'thinking', content: data?.text ?? '', streaming: false },
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
