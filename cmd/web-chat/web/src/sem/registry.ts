import { fromJson, type Message } from '@bufbuild/protobuf';
import type { GenMessage } from '@bufbuild/protobuf/codegenv2';
import { type AgentModeV1, AgentModeV1Schema } from '../sem/pb/proto/sem/base/agent_pb';
import { type DebuggerPauseV1, DebuggerPauseV1Schema } from '../sem/pb/proto/sem/base/debugger_pb';

import { type LlmDelta, LlmDeltaSchema, LlmDoneSchema, type LlmFinal, LlmFinalSchema, type LlmStart, LlmStartSchema } from '../sem/pb/proto/sem/base/llm_pb';
import { type LogV1, LogV1Schema } from '../sem/pb/proto/sem/base/log_pb';
import { type ToolDelta, ToolDeltaSchema, ToolDoneSchema, type ToolResult, ToolResultSchema, type ToolStart, ToolStartSchema } from '../sem/pb/proto/sem/base/tool_pb';
import {
  type ThinkingModeCompleted,
  ThinkingModeCompletedSchema,
  type ThinkingModeStarted,
  ThinkingModeStartedSchema,
  type ThinkingModeUpdate,
  ThinkingModeUpdateSchema,
} from '../sem/pb/proto/sem/middleware/thinking_mode_pb';
import { type TimelineUpsertV1, TimelineUpsertV1Schema } from '../sem/pb/proto/sem/timeline/transport_pb';
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

export function registerDefaultSemHandlers() {
  handlers.clear();

  registerSem('timeline.upsert', (ev, dispatch) => {
    const data = decodeProto<TimelineUpsertV1>(TimelineUpsertV1Schema, ev.data);
    const entity = data?.entity;
    if (!entity) return;
    const mapped = timelineEntityFromProto(entity, data?.version);
    if (!mapped) return;
    upsertEntity(dispatch, mapped);
  });

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

  // thinking mode selection widget (separate from llm.thinking.* streaming)
  registerSem('thinking.mode.started', (ev, dispatch) => {
    const pb = decodeProto<ThinkingModeStarted>(ThinkingModeStartedSchema, ev.data);
    const id = pb?.itemId || ev.id;
    const data = pb?.data;
    upsertEntity(dispatch, {
      id,
      kind: 'thinking_mode',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { mode: data?.mode, phase: data?.phase, reasoning: data?.reasoning, extraData: data?.extraData ?? {}, status: 'started' },
    });
  });

  registerSem('thinking.mode.update', (ev, dispatch) => {
    const pb = decodeProto<ThinkingModeUpdate>(ThinkingModeUpdateSchema, ev.data);
    const id = pb?.itemId || ev.id;
    const data = pb?.data;
    upsertEntity(dispatch, {
      id,
      kind: 'thinking_mode',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: { mode: data?.mode, phase: data?.phase, reasoning: data?.reasoning, extraData: data?.extraData ?? {}, status: 'update' },
    });
  });

  registerSem('thinking.mode.completed', (ev, dispatch) => {
    const pb = decodeProto<ThinkingModeCompleted>(ThinkingModeCompletedSchema, ev.data);
    const id = pb?.itemId || ev.id;
    const data = pb?.data;
    upsertEntity(dispatch, {
      id,
      kind: 'thinking_mode',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: {
        mode: data?.mode,
        phase: data?.phase,
        reasoning: data?.reasoning,
        extraData: data?.extraData ?? {},
        status: 'completed',
        success: pb?.success,
        error: pb?.error,
      },
    });
  });

}
