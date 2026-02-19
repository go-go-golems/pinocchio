import type React from 'react';
import {
  GenericCard,
  LogCard,
  MessageCard,
  ThinkingModeCard,
  ToolCallCard,
  ToolResultCard,
} from './cards';
import type { ChatWidgetRenderers, RenderEntity } from './types';

type Renderer = React.ComponentType<{ e: RenderEntity }>;

const builtinRenderers: Record<string, Renderer> = {
  message: MessageCard,
  tool_call: ToolCallCard,
  tool_result: ToolResultCard,
  log: LogCard,
  thinking_mode: ThinkingModeCard,
};

const extensionRenderers = new Map<string, Renderer>();

export function registerTimelineRenderer(kind: string, renderer: Renderer) {
  const key = String(kind || '').trim();
  if (!key) return;
  extensionRenderers.set(key, renderer);
}

export function unregisterTimelineRenderer(kind: string) {
  const key = String(kind || '').trim();
  if (!key) return;
  extensionRenderers.delete(key);
}

export function clearRegisteredTimelineRenderers() {
  extensionRenderers.clear();
}

export function resolveTimelineRenderers(overrides?: Partial<ChatWidgetRenderers>): ChatWidgetRenderers {
  const resolved: ChatWidgetRenderers = {
    ...builtinRenderers,
    ...Object.fromEntries(extensionRenderers.entries()),
    ...(overrides ?? {}),
    default: GenericCard,
  };
  if (overrides?.default) {
    resolved.default = overrides.default;
  }
  return resolved;
}
