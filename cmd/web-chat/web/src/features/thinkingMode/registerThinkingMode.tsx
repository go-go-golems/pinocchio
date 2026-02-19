import { registerSem, type SemEvent } from '../../sem/registry';
import { registerTimelinePropsNormalizer } from '../../sem/timelinePropsRegistry';
import type { AppDispatch } from '../../store/store';
import { type TimelineEntity, timelineSlice } from '../../store/timelineSlice';
import { Markdown } from '../../webchat/Markdown';
import { registerTimelineRenderer } from '../../webchat/rendererRegistry';
import type { RenderEntity } from '../../webchat/types';
import { fmtSentAt } from '../../webchat/utils';

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

function asRecord(v: unknown): Record<string, unknown> | null {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return null;
  return v as Record<string, unknown>;
}

type ParsedThinkingModeData = {
  mode: string;
  phase: string;
  reasoning: string;
  extraData: Record<string, unknown>;
};

type ParsedThinkingModeSem = {
  itemId: string;
  data: ParsedThinkingModeData;
  success?: boolean;
  error: string;
};

function parseThinkingModeSem(raw: unknown): ParsedThinkingModeSem {
  const obj = asRecord(raw) ?? {};
  const dataObj = asRecord(obj.data) ?? {};
  const extraData = asRecord(dataObj.extraData) ?? {};
  return {
    itemId: asString(obj.itemId),
    data: {
      mode: asString(dataObj.mode),
      phase: asString(dataObj.phase),
      reasoning: asString(dataObj.reasoning),
      extraData,
    },
    success: asBoolean(obj.success),
    error: asString(obj.error),
  };
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
    const parsed = parseThinkingModeSem(ev.data);
    const id = parsed.itemId || ev.id;
    upsertEntity(dispatch, {
      id,
      kind: 'thinking_mode',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: {
        mode: parsed.data.mode,
        phase: parsed.data.phase,
        reasoning: parsed.data.reasoning,
        extraData: parsed.data.extraData,
        status: 'started',
      },
    });
  });

  registerSem('thinking.mode.update', (ev, dispatch) => {
    const parsed = parseThinkingModeSem(ev.data);
    const id = parsed.itemId || ev.id;
    upsertEntity(dispatch, {
      id,
      kind: 'thinking_mode',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: {
        mode: parsed.data.mode,
        phase: parsed.data.phase,
        reasoning: parsed.data.reasoning,
        extraData: parsed.data.extraData,
        status: 'update',
      },
    });
  });

  registerSem('thinking.mode.completed', (ev, dispatch) => {
    const parsed = parseThinkingModeSem(ev.data);
    const id = parsed.itemId || ev.id;
    upsertEntity(dispatch, {
      id,
      kind: 'thinking_mode',
      createdAt: createdAtFromEvent(ev),
      updatedAt: Date.now(),
      props: {
        mode: parsed.data.mode,
        phase: parsed.data.phase,
        reasoning: parsed.data.reasoning,
        extraData: parsed.data.extraData,
        status: 'completed',
        success: parsed.success,
        error: parsed.error,
      },
    });
  });
}
