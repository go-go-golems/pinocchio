import { fromJson, type Message } from '@bufbuild/protobuf';
import type { GenMessage } from '@bufbuild/protobuf/codegenv2';
import {
  type ThinkingModeCompleted,
  ThinkingModeCompletedSchema,
  type ThinkingModeStarted,
  ThinkingModeStartedSchema,
  type ThinkingModeUpdate,
  ThinkingModeUpdateSchema,
} from '../../sem/pb/proto/sem/middleware/thinking_mode_pb';
import { registerSem, type SemEvent } from '../../sem/registry';
import { registerTimelinePropsNormalizer } from '../../sem/timelinePropsRegistry';
import type { AppDispatch } from '../../store/store';
import { type TimelineEntity, timelineSlice } from '../../store/timelineSlice';
import { Markdown } from '../../webchat/Markdown';
import { registerTimelineRenderer } from '../../webchat/rendererRegistry';
import type { RenderEntity } from '../../webchat/types';
import { fmtSentAt } from '../../webchat/utils';

function decodeProto<T extends Message>(schema: GenMessage<T>, raw: unknown): T | null {
  if (!raw || typeof raw !== 'object') return null;
  try {
    return fromJson(schema as any, raw as any, { ignoreUnknownFields: true }) as T;
  } catch {
    return null;
  }
}

function createdAtFromEvent(_ev: SemEvent): number {
  return Date.now();
}

function upsertEntity(dispatch: AppDispatch, entity: TimelineEntity) {
  dispatch(timelineSlice.actions.upsertEntity(entity));
}

function asString(v: unknown): string {
  return typeof v === 'string' ? v : '';
}

function asBoolean(v: unknown): boolean | undefined {
  return typeof v === 'boolean' ? v : undefined;
}

function ThinkingModeCard({ e }: { e: RenderEntity }) {
  const mode = String(e.props?.mode ?? '');
  const phase = String(e.props?.phase ?? '');
  const status = String(e.props?.status ?? '');
  const success = e.props?.success;
  const error = e.props?.error ? String(e.props.error) : '';
  const reasoning = e.props?.reasoning ? String(e.props.reasoning) : '';
  const header = mode ? `Thinking mode: ${mode}` : 'Thinking mode';

  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="card-header-title">{header}</div>
        {phase ? (
          <div data-part="pill" data-mono="true">
            {phase}
          </div>
        ) : null}
        {status ? <div data-part="pill">{status}</div> : null}
        {typeof success === 'boolean' ? (
          <div data-part="pill" data-variant={success ? 'ok' : 'error'}>
            {success ? 'ok' : 'fail'}
          </div>
        ) : null}
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        {reasoning ? <Markdown text={reasoning} /> : <div data-part="pill">No reasoning</div>}
        {error ? (
          <div data-part="status-text" data-variant="error" style={{ marginTop: 10 }}>
            {error}
          </div>
        ) : null}
      </div>
    </div>
  );
}

// registerThinkingModeModule wires thinking-mode SEM projection + props normalization + renderer dispatch.
export function registerThinkingModeModule() {
  registerTimelinePropsNormalizer('thinking_mode', (props) => {
    const status = asString(props.status);
    const successFromStatus = status === 'completed' ? true : status === 'error' ? false : undefined;
    const success = asBoolean(props.success);
    return {
      ...props,
      status,
      success: typeof success === 'boolean' ? success : successFromStatus,
      error: asString(props.error),
    };
  });

  registerTimelineRenderer('thinking_mode', ThinkingModeCard);

  registerSem('thinking.mode.started', (ev, dispatch) => {
    const pb = decodeProto<ThinkingModeStarted>(ThinkingModeStartedSchema, ev.data);
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
        status: 'started',
      },
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
      props: {
        mode: data?.mode,
        phase: data?.phase,
        reasoning: data?.reasoning,
        extraData: data?.extraData ?? {},
        status: 'update',
      },
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
