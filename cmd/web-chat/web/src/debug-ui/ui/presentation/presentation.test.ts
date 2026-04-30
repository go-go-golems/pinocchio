import { describe, expect, it } from 'vitest';
import { getBlockPresentation } from './blocks';
import { getEventPresentation } from './events';
import { getTimelineKindPresentation } from './timeline';

describe('presentation helpers', () => {
  it('maps known event types', () => {
    expect(getEventPresentation('llm.start')).toEqual({
      icon: '▶️',
      color: 'var(--accent-green)',
      badgeClass: 'badge-green',
    });

    expect(getEventPresentation('tool.result')).toEqual({
      icon: '📤',
      color: 'var(--accent-cyan)',
      badgeClass: 'badge-cyan',
    });
  });


  it('falls back safely for unknown event types', () => {
    expect(getEventPresentation('custom.unknown')).toEqual({
      icon: '📦',
      color: 'var(--border-color)',
      badgeClass: 'badge-blue',
    });
  });

  it('maps known and unknown block kinds', () => {
    expect(getBlockPresentation('tool_call')).toEqual({
      icon: '🔧',
      badgeClass: 'badge-yellow',
    });

    expect(getBlockPresentation('mystery_kind')).toEqual({
      icon: '📦',
      badgeClass: 'badge-blue',
    });
  });

  it('maps known and unknown timeline kinds', () => {
    expect(getTimelineKindPresentation('message')).toEqual({
      icon: '💬',
      color: 'var(--accent-blue)',
      badgeClass: 'badge-blue',
    });

    expect(getTimelineKindPresentation('unknown_kind')).toEqual({
      icon: '📦',
      color: 'var(--border-color)',
      badgeClass: 'badge-blue',
    });
  });
});
