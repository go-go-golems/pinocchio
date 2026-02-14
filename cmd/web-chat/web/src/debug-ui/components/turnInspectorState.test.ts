import { describe, expect, it } from 'vitest';
import type { ParsedBlock, TurnDetail } from '../types';
import {
  getAvailableTurnPhases,
  resolveBlockSelectionIndex,
  resolveCompareSelection,
} from './turnInspectorState';

const baseBlock = (overrides: Partial<ParsedBlock>): ParsedBlock => ({
  index: 0,
  kind: 'llm_text',
  payload: {},
  metadata: {},
  ...overrides,
});

const detail: TurnDetail = {
  conv_id: 'conv-1',
  session_id: 'session-1',
  turn_id: 'turn-1',
  phases: {
    draft: {
      captured_at: new Date(100).toISOString(),
      turn: {
        id: 'turn-1',
        blocks: [
          baseBlock({ index: 0, id: 'a', payload: { text: 'draft' }, metadata: { x: 1 } }),
          baseBlock({ index: 1, id: 'b', payload: { text: 'same' }, metadata: { y: 1 } }),
        ],
        metadata: {},
        data: {},
      },
    },
    final: {
      captured_at: new Date(200).toISOString(),
      turn: {
        id: 'turn-1',
        blocks: [
          baseBlock({ index: 0, id: 'a', payload: { text: 'final' }, metadata: { x: 2 } }),
          baseBlock({ index: 1, id: 'c', payload: { text: 'new' }, metadata: {} }),
        ],
        metadata: {},
        data: {},
      },
    },
  },
};

describe('turnInspectorState', () => {
  it('resolves compare defaults to first and last available phases', () => {
    const available = getAvailableTurnPhases(detail);
    const resolved = resolveCompareSelection(available, { a: null, b: null });
    expect(resolved).toEqual({ a: 'draft', b: 'final' });
  });

  it('normalizes compare phases when invalid or duplicated', () => {
    const available = getAvailableTurnPhases(detail);
    expect(resolveCompareSelection(available, { a: 'final', b: 'final' })).toEqual({
      a: 'final',
      b: 'draft',
    });
    expect(resolveCompareSelection(available, { a: 'post_tools', b: 'draft' })).toEqual({
      a: 'draft',
      b: 'final',
    });
  });

  it('resolves block selection by id, then index, then payload/metadata signature', () => {
    const byID = resolveBlockSelectionIndex(
      detail,
      'final',
      baseBlock({ index: 3, id: 'c', payload: { text: 'ignored' } })
    );
    expect(byID).toBe(1);

    const byIndex = resolveBlockSelectionIndex(
      detail,
      'draft',
      baseBlock({ index: 1, payload: { text: 'whatever' } })
    );
    expect(byIndex).toBe(1);

    const bySignature = resolveBlockSelectionIndex(
      detail,
      'final',
      baseBlock({ index: 9, payload: { text: 'final' }, metadata: { x: 2 } })
    );
    expect(bySignature).toBe(0);
  });
});
