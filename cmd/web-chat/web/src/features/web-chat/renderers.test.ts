import { describe, expect, it } from 'vitest';
import { GenericCard, MessageCard } from './cards';
import { createWebChatRenderers } from './renderers';

function OverrideRenderer() {
  return null;
}

describe('createWebChatRenderers', () => {
  it('includes builtin renderers for canonical web-chat entity kinds', () => {
    const renderers = createWebChatRenderers();

    expect(renderers.message).toBe(MessageCard);
    expect(renderers.agent_mode).toBeTypeOf('function');
    expect(renderers.tool_call).toBeTypeOf('function');
    expect(renderers.widget_instance).toBeTypeOf('function');
    expect(renderers.default).toBe(GenericCard);
  });

  it('applies overrides without leaking them to later calls', () => {
    const overridden = createWebChatRenderers({ overrides: { message: OverrideRenderer } });
    const fresh = createWebChatRenderers();

    expect(overridden.message).toBe(OverrideRenderer);
    expect(fresh.message).toBe(MessageCard);
  });
});
