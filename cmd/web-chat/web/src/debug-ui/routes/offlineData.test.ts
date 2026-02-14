import { describe, expect, it } from 'vitest';
import type { RunDetailResponse } from '../types';
import { buildTurnDetail, parseOfflineInspectorData } from './offlineData';

describe('offlineData', () => {
  it('parses turns sqlite detail into turn snapshots', () => {
    const run: RunDetailResponse = {
      run_id: 'turns|conv-1|session-1',
      kind: 'turns',
      detail: {
        conv_id: 'conv-1',
        session_id: 'session-1',
        items: [
          {
            conv_id: 'conv-1',
            session_id: 'session-1',
            turn_id: 'turn-1',
            phase: 'draft',
            created_at_ms: 100,
            parsed: {
              id: 'turn-1',
              blocks: [{ kind: 'user', payload: { text: 'hi' }, metadata: {} }],
              metadata: { 'geppetto.session_id@v1': 'session-1' },
            },
          },
          {
            conv_id: 'conv-1',
            session_id: 'session-1',
            turn_id: 'turn-1',
            phase: 'final',
            created_at_ms: 200,
            payload: 'id: turn-1\\nblocks:\\n  - kind: llm_text\\n    payload:\\n      text: hello\\n',
          },
        ],
      },
    };

    const parsed = parseOfflineInspectorData(run);
    expect(parsed.turns).toHaveLength(2);
    expect(parsed.turns[0].phase).toBe('draft');
    expect(parsed.turns[1].phase).toBe('final');

    const turnDetail = buildTurnDetail(parsed.turns, parsed.convID, parsed.sessionID, 'turn-1');
    expect(turnDetail?.phases.draft).toBeTruthy();
    expect(turnDetail?.phases.final).toBeTruthy();
  });

  it('parses artifact detail events and turns', () => {
    const run: RunDetailResponse = {
      run_id: 'artifact|run-1',
      kind: 'artifact',
      detail: {
        input_turn: {
          parsed: {
            id: 'input',
            blocks: [{ kind: 'user', payload: { text: 'q' }, metadata: {} }],
            metadata: {},
          },
        },
        turns: [
          {
            name: 'final_turn.yaml',
            parsed: {
              id: 'turn-1',
              blocks: [{ kind: 'llm_text', payload: { text: 'a' }, metadata: {} }],
              metadata: {},
            },
          },
        ],
        events: [
          {
            name: 'events.ndjson',
            items: [{ event: { type: 'chat.message', id: 'evt-1', data: { text: 'a' } } }],
          },
        ],
      },
    };

    const parsed = parseOfflineInspectorData(run);
    expect(parsed.turns.length).toBeGreaterThanOrEqual(2);
    expect(parsed.events).toHaveLength(1);
    expect(parsed.events[0].type).toBe('chat.message');
  });

  it('parses timeline snapshot entities and flattens props', () => {
    const run: RunDetailResponse = {
      run_id: 'timeline|conv-1',
      kind: 'timeline',
      detail: {
        snapshot: {
          entities: [
            {
              id: 'msg-1',
              kind: 'message',
              createdAtMs: 123,
              snapshot: { message: { role: 'assistant', content: 'hello' } },
            },
          ],
        },
      },
    };

    const parsed = parseOfflineInspectorData(run);
    expect(parsed.entities).toHaveLength(1);
    expect(parsed.entities[0].kind).toBe('message');
    expect(parsed.entities[0].props.role).toBe('assistant');
  });
});
