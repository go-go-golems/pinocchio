import { describe, expect, it } from 'vitest';
import { resolveTimelineRenderers } from './rendererRegistry';

describe('rendererRegistry', () => {
  it('includes a builtin renderer for agent_mode entities', () => {
    const renderers = resolveTimelineRenderers();
    expect(renderers.agent_mode).toBeTypeOf('function');
  });
});
