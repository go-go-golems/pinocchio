import { beforeEach, describe, expect, it } from 'vitest';
import { timelineSlice } from '../store/timelineSlice';
import { handleSem, registerDefaultSemHandlers, type SemEvent } from './registry';

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

function emitSem(dispatch: (action: unknown) => unknown, event: SemEvent) {
  handleSem(
    {
      sem: true,
      event,
    },
    dispatch as any
  );
}

describe('SEM registry llm message projection', () => {
  beforeEach(() => {
    registerDefaultSemHandlers();
  });

  it('does not create a timeline message on llm.start without content', () => {
    const store = dispatchThroughTimelineReducer();

    emitSem(store.dispatch, {
      type: 'llm.start',
      id: 'msg-1',
      data: { id: 'msg-1', role: 'assistant' },
    });

    expect(store.getState().order).toEqual([]);
  });

  it('creates a message only when non-empty llm.delta data appears', () => {
    const store = dispatchThroughTimelineReducer();

    emitSem(store.dispatch, {
      type: 'llm.start',
      id: 'msg-2',
      data: { id: 'msg-2', role: 'assistant' },
    });
    emitSem(store.dispatch, {
      type: 'llm.delta',
      id: 'msg-2',
      data: { id: 'msg-2', cumulative: '' },
    });
    expect(store.getState().order).toEqual([]);

    emitSem(store.dispatch, {
      type: 'llm.delta',
      id: 'msg-2',
      data: { id: 'msg-2', cumulative: 'Hello world' },
    });

    const msg = store.getState().byId['msg-2'];
    expect(msg).toBeTruthy();
    expect(msg.props.role).toBe('assistant');
    expect(msg.props.content).toBe('Hello world');
    expect(msg.props.streaming).toBe(true);
  });

  it('does not create a thinking message when no thinking text was ever emitted', () => {
    const store = dispatchThroughTimelineReducer();

    emitSem(store.dispatch, {
      type: 'llm.thinking.start',
      id: 'thinking-1',
      data: { id: 'thinking-1', role: 'thinking' },
    });
    emitSem(store.dispatch, {
      type: 'llm.thinking.final',
      id: 'thinking-1',
      data: { id: 'thinking-1' },
    });

    expect(store.getState().order).toEqual([]);
  });

  it('preserves prior content when llm.final has empty text and only toggles streaming off', () => {
    const store = dispatchThroughTimelineReducer();

    emitSem(store.dispatch, {
      type: 'llm.delta',
      id: 'msg-3',
      data: { id: 'msg-3', cumulative: 'partial response' },
    });
    emitSem(store.dispatch, {
      type: 'llm.final',
      id: 'msg-3',
      data: { id: 'msg-3', text: '' },
    });

    const msg = store.getState().byId['msg-3'];
    expect(msg).toBeTruthy();
    expect(msg.props.content).toBe('partial response');
    expect(msg.props.streaming).toBe(false);
  });

  it('does not leak emitted state after terminal message updates with text', () => {
    const store = dispatchThroughTimelineReducer();

    emitSem(store.dispatch, {
      type: 'llm.final',
      id: 'msg-reuse',
      data: { id: 'msg-reuse', text: 'done' },
    });
    expect(store.getState().order).toEqual(['msg-reuse']);

    store.dispatch(timelineSlice.actions.clear());
    expect(store.getState().order).toEqual([]);

    emitSem(store.dispatch, {
      type: 'llm.final',
      id: 'msg-reuse',
      data: { id: 'msg-reuse', text: '' },
    });

    expect(store.getState().order).toEqual([]);
  });
});
