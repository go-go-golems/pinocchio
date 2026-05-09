import { describe, expect, it } from 'vitest';
import { timelineSlice, type TimelineEntity } from '../store/timelineSlice';
import { timelineMutationFromUIEvent } from './timelineEvents';
import type { CanonicalFrame } from './protocol';

type TimelineState = {
  byId: Record<string, TimelineEntity>;
  order: string[];
};

function applyFrames(frames: CanonicalFrame[]): TimelineState {
  let state: TimelineState = { byId: {}, order: [] };
  for (const frame of frames) {
    const mutation = timelineMutationFromUIEvent(frame);
    if (mutation?.upsert) {
      state = timelineSlice.reducer(state, timelineSlice.actions.upsertEntity(mutation.upsert));
    }
    if (mutation?.deleteId) {
      state = timelineSlice.reducer(state, timelineSlice.actions.deleteEntity(mutation.deleteId));
    }
  }
  return state;
}

describe('frontend timeline protocol matrix', () => {
  it('FRONTEND-01 sparse tool finish preserves name input and correlation after Redux merge', () => {
    const correlation = {
      provider: 'openai-responses',
      model: 'gpt-test',
      responseId: 'resp_tool',
      itemId: 'fc_1',
      outputIndex: 0,
      streamKind: 'tool_call',
      toolCallId: 'call_1',
      toolCallIndex: 0,
      correlationKey: 'tool:call_1',
      parentCorrelationKey: 'provider-call-key',
    };

    const state = applyFrames([
      {
        name: 'ChatToolCallRequested',
        payload: {
          messageId: 'chat-msg-1',
          toolCallId: 'call_1',
          toolName: 'search',
          input: '{"query":"gold"}',
          status: 'pending',
          correlation,
        },
      },
      {
        name: 'ChatToolCallFinished',
        payload: {
          messageId: 'chat-msg-1',
          toolCallId: 'call_1',
          status: 'completed',
        },
      },
    ]);

    const entity = state.byId.call_1;
    expect(entity?.kind).toBe('tool_call');
    expect(entity?.props.name).toBe('search');
    expect(entity?.props.toolName).toBe('search');
    expect(entity?.props.input).toEqual({ query: 'gold' });
    expect(entity?.props.inputRaw).toBe('{"query":"gold"}');
    expect(entity?.props.done).toBe(true);
    expect(entity?.props.status).toBe('completed');
    expect(entity?.props.provider).toBe('openai-responses');
    expect(entity?.props.responseId).toBe('resp_tool');
    expect(entity?.props.outputIndex).toBe(0);
    expect(entity?.props.toolCallIndex).toBe(0);
    expect(entity?.props.correlationKey).toBe('tool:call_1');
  });

  it('FRONTEND-02 missing tool name does not persist display fallback as canonical state', () => {
    const mutation = timelineMutationFromUIEvent({
      name: 'ChatToolCallStarted',
      payload: {
        messageId: 'chat-msg-1',
        toolCallId: 'call_missing_name',
        status: 'pending',
      },
    });

    expect(mutation?.upsert?.id).toBe('call_missing_name');
    expect(mutation?.upsert?.props).not.toHaveProperty('name');
    expect(mutation?.upsert?.props).not.toHaveProperty('toolName');
  });

  it('FRONTEND-03 sparse text finish preserves prior content and correlation after Redux merge', () => {
    const correlation = {
      provider: 'openai-responses',
      model: 'gpt-test',
      responseId: 'resp_text',
      itemId: 'msg_1',
      outputIndex: 0,
      segmentId: 'segment-text-1',
      segmentIndex: 1,
      segmentType: 'text',
      streamKind: 'content',
      correlationKey: 'text:msg_1',
      parentCorrelationKey: 'provider-call-key',
    };

    const state = applyFrames([
      {
        name: 'ChatTextDelta',
        payload: {
          messageId: 'chat-msg-1:text:1',
          role: 'assistant',
          content: 'partial answer',
          text: 'partial answer',
          status: 'streaming',
          streaming: true,
          correlation,
        },
      },
      {
        name: 'ChatTextSegmentFinished',
        payload: {
          messageId: 'chat-msg-1:text:1',
          status: 'failed',
          streaming: false,
          final: true,
        },
      },
    ]);

    const entity = state.byId['chat-msg-1:text:1'];
    expect(entity?.kind).toBe('message');
    expect(entity?.props.content).toBe('partial answer');
    expect(entity?.props.status).toBe('failed');
    expect(entity?.props.final).toBe(true);
    expect(entity?.props.provider).toBe('openai-responses');
    expect(entity?.props.responseId).toBe('resp_text');
    expect(entity?.props.outputIndex).toBe(0);
    expect(entity?.props.segmentId).toBe('segment-text-1');
    expect(entity?.props.correlationKey).toBe('text:msg_1');
  });
});
