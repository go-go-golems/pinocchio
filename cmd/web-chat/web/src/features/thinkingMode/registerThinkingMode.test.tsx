import { describe, expect, it } from 'vitest';
import { handleSem, registerDefaultSemHandlers } from '../../sem/registry';
import { timelineEntityFromProto } from '../../sem/timelineMapper';
import { clearRegisteredTimelinePropsNormalizers } from '../../sem/timelinePropsRegistry';
import { timelineSlice } from '../../store/timelineSlice';
import { clearRegisteredTimelineRenderers, resolveTimelineRenderers } from '../../webchat/rendererRegistry';
import { registerThinkingModeModule } from './registerThinkingMode';

function dispatchThroughTimelineReducer() {
  let state = timelineSlice.reducer(undefined, { type: '@@INIT' });
  const dispatch = (action: any) => {
    state = timelineSlice.reducer(state, action);
    return action;
  };
  return {
    dispatch,
    getState: () => state,
  };
}

describe('registerThinkingModeModule', () => {
  it('registers thinking_mode props normalizer and renderer', () => {
    clearRegisteredTimelinePropsNormalizers();
    clearRegisteredTimelineRenderers();

    registerThinkingModeModule();

    const mapped = timelineEntityFromProto(
      {
        id: 'tm-entity-1',
        kind: 'thinking_mode',
        createdAtMs: '100',
        props: {
          mode: 'deep',
          status: 'error',
          error: 'failed',
        },
      } as any,
      1
    );

    expect(mapped).not.toBeNull();
    expect(mapped?.props.status).toBe('error');
    expect(mapped?.props.success).toBe(false);
    expect(mapped?.props.error).toBe('failed');

    const renderers = resolveTimelineRenderers();
    expect(renderers.thinking_mode).toBeTypeOf('function');
  });

  it('registers SEM thinking.mode.* projection handlers', () => {
    registerDefaultSemHandlers();
    registerThinkingModeModule();
    const store = dispatchThroughTimelineReducer();

    handleSem(
      {
        sem: true,
        event: {
          type: 'thinking.mode.completed',
          id: 'tm-evt-1',
          seq: 12,
          data: {
            itemId: 'tm-1',
            data: {
              mode: 'chain',
              phase: 'confirmed',
              reasoning: 'done',
            },
            success: true,
            error: '',
          },
        },
      },
      store.dispatch as any
    );

    const timeline = store.getState();
    expect(timeline.byId['tm-1']).toBeTruthy();
    expect(timeline.byId['tm-1'].kind).toBe('thinking_mode');
    expect(timeline.byId['tm-1'].props.mode).toBe('chain');
    expect(timeline.byId['tm-1'].props.status).toBe('completed');
  });
});
