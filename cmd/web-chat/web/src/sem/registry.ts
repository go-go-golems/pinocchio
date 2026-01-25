import type { AppDispatch } from '../store/store';
import { timelineSlice, type TimelineEntity } from '../store/timelineSlice';
import { fromJson, type Message } from '@bufbuild/protobuf';
import type { GenMessage } from '@bufbuild/protobuf/codegenv2';

import { LlmDeltaSchema, LlmDoneSchema, LlmFinalSchema, LlmStartSchema, type LlmDelta, type LlmFinal, type LlmStart } from '../sem/pb/proto/sem/base/llm_pb';
import { ToolDeltaSchema, ToolDoneSchema, ToolResultSchema, ToolStartSchema, type ToolDelta, type ToolResult, type ToolStart } from '../sem/pb/proto/sem/base/tool_pb';
import { LogV1Schema, type LogV1 } from '../sem/pb/proto/sem/base/log_pb';
import { AgentModeV1Schema, type AgentModeV1 } from '../sem/pb/proto/sem/base/agent_pb';
import { DebuggerPauseV1Schema, type DebuggerPauseV1 } from '../sem/pb/proto/sem/base/debugger_pb';
import {
  ThinkingModeCompletedSchema,
  ThinkingModeStartedSchema,
  ThinkingModeUpdateSchema,
  type ThinkingModeCompleted,
  type ThinkingModeStarted,
  type ThinkingModeUpdate,
} from '../sem/pb/proto/sem/middleware/thinking_mode_pb';
import {
  PlanningCompletedSchema,
  PlanningIterationSchema,
  PlanningReflectionSchema,
  PlanningStartedSchema,
  ExecutionCompletedSchema,
  ExecutionStartedSchema,
  type PlanningCompleted,
  type PlanningIteration,
  type PlanningReflection,
  type PlanningStarted,
  type ExecutionCompleted,
  type ExecutionStarted,
} from '../sem/pb/proto/sem/middleware/planning_pb';

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

type PlanningAgg = {
  runId: string;
  createdAt: number;
  startedAt?: number;
  provider?: string;
  plannerModel?: string;
  maxIterations?: number;
  iterations: Array<{
    index: number;
    action: string;
    reasoning: string;
    strategy: string;
    progress: string;
    toolName: string;
    reflectionText: string;
    emittedAt?: number;
  }>;
  reflectionByIter: Record<number, { text: string; score: number; emittedAt?: number }>;
  completed?: { totalIterations: number; finalDecision: string; statusReason: string; finalDirective: string; completedAt?: number };
  execution?: { executorModel?: string; directive?: string; startedAt?: number; status?: string; errorMessage?: string; tokensUsed?: number; responseLength?: number };
};

const planningAggs = new Map<string, PlanningAgg>();

function ensurePlanningAgg(runId: string, now: number): PlanningAgg {
  const existing = planningAggs.get(runId);
  if (existing) return existing;
  const agg: PlanningAgg = { runId, createdAt: now, iterations: [], reflectionByIter: {} };
  planningAggs.set(runId, agg);
  return agg;
}

function planningEntityFromAgg(agg: PlanningAgg, now: number): TimelineEntity {
  // Important: never share mutable aggregates with Redux state.
  // RTK/Immer will freeze state trees; if we keep references in this Map,
  // later mutations will throw (e.g. "can't define array index ... non-writable length").
  const iterations = agg.iterations.map((it) => ({ ...it }));
  const reflectionByIter = Object.fromEntries(
    Object.entries(agg.reflectionByIter).map(([k, v]) => [k, { ...v }]),
  ) as PlanningAgg['reflectionByIter'];
  const completed = agg.completed ? { ...agg.completed } : undefined;
  const execution = agg.execution ? { ...agg.execution } : undefined;

  return {
    id: agg.runId,
    kind: 'planning',
    createdAt: agg.createdAt,
    updatedAt: now,
    props: {
      runId: agg.runId,
      provider: agg.provider,
      plannerModel: agg.plannerModel,
      maxIterations: agg.maxIterations,
      startedAt: agg.startedAt,
      iterations,
      reflectionByIter,
      completed,
      execution,
    },
  };
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

  // planning widget (aggregated)
  registerSem('planning.start', (ev, dispatch) => {
    const pb = decodeProto<PlanningStarted>(PlanningStartedSchema, ev.data);
    const runId = pb?.run?.runId || '';
    if (!runId) return;
    const now = Date.now();
    const agg = ensurePlanningAgg(runId, now);
    agg.provider = pb?.run?.provider;
    agg.plannerModel = pb?.run?.plannerModel;
    agg.maxIterations = pb?.run?.maxIterations;
    agg.startedAt = Number(pb?.startedAtUnixMs ?? 0n) || now;
    upsertEntity(dispatch, planningEntityFromAgg(agg, now));
  });

  registerSem('planning.iteration', (ev, dispatch) => {
    const pb = decodeProto<PlanningIteration>(PlanningIterationSchema, ev.data);
    const runId = pb?.run?.runId || '';
    if (!runId) return;
    const now = Date.now();
    const agg = ensurePlanningAgg(runId, now);
    agg.provider = pb?.run?.provider || agg.provider;
    agg.plannerModel = pb?.run?.plannerModel || agg.plannerModel;
    agg.maxIterations = pb?.run?.maxIterations || agg.maxIterations;
    const idx = pb?.iterationIndex ?? 0;
    const existingIndex = agg.iterations.findIndex((it) => it.index === idx);
    const iteration = {
      index: idx,
      action: pb?.action ?? '',
      reasoning: pb?.reasoning ?? '',
      strategy: pb?.strategy ?? '',
      progress: pb?.progress ?? '',
      toolName: pb?.toolName ?? '',
      reflectionText: pb?.reflectionText ?? '',
      emittedAt: Number(pb?.emittedAtUnixMs ?? 0n) || undefined,
    };
    if (existingIndex >= 0) agg.iterations[existingIndex] = iteration;
    else agg.iterations.push(iteration);
    agg.iterations.sort((a, b) => a.index - b.index);
    upsertEntity(dispatch, planningEntityFromAgg(agg, now));
  });

  registerSem('planning.reflection', (ev, dispatch) => {
    const pb = decodeProto<PlanningReflection>(PlanningReflectionSchema, ev.data);
    const runId = pb?.run?.runId || '';
    if (!runId) return;
    const now = Date.now();
    const agg = ensurePlanningAgg(runId, now);
    const idx = pb?.iterationIndex ?? 0;
    agg.reflectionByIter[idx] = {
      text: pb?.reflectionText ?? '',
      score: pb?.progressScore ?? 0,
      emittedAt: Number(pb?.emittedAtUnixMs ?? 0n) || undefined,
    };
    upsertEntity(dispatch, planningEntityFromAgg(agg, now));
  });

  registerSem('planning.complete', (ev, dispatch) => {
    const pb = decodeProto<PlanningCompleted>(PlanningCompletedSchema, ev.data);
    const runId = pb?.run?.runId || '';
    if (!runId) return;
    const now = Date.now();
    const agg = ensurePlanningAgg(runId, now);
    agg.completed = {
      totalIterations: pb?.totalIterations ?? 0,
      finalDecision: pb?.finalDecision ?? '',
      statusReason: pb?.statusReason ?? '',
      finalDirective: pb?.finalDirective ?? '',
      completedAt: Number(pb?.completedAtUnixMs ?? 0n) || undefined,
    };
    upsertEntity(dispatch, planningEntityFromAgg(agg, now));
  });

  registerSem('execution.start', (ev, dispatch) => {
    const pb = decodeProto<ExecutionStarted>(ExecutionStartedSchema, ev.data);
    const runId = pb?.runId || '';
    if (!runId) return;
    const now = Date.now();
    const agg = ensurePlanningAgg(runId, now);
    agg.execution = {
      ...(agg.execution ?? {}),
      executorModel: pb?.executorModel,
      directive: pb?.directive,
      startedAt: Number(pb?.startedAtUnixMs ?? 0n) || undefined,
      status: 'started',
    };
    upsertEntity(dispatch, planningEntityFromAgg(agg, now));
  });

  registerSem('execution.complete', (ev, dispatch) => {
    const pb = decodeProto<ExecutionCompleted>(ExecutionCompletedSchema, ev.data);
    const runId = pb?.runId || '';
    if (!runId) return;
    const now = Date.now();
    const agg = ensurePlanningAgg(runId, now);
    agg.execution = {
      ...(agg.execution ?? {}),
      status: pb?.status || 'completed',
      errorMessage: pb?.errorMessage,
      tokensUsed: pb?.tokensUsed,
      responseLength: pb?.responseLength,
    };
    upsertEntity(dispatch, planningEntityFromAgg(agg, now));
  });
}
