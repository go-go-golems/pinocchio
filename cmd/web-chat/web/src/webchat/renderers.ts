import type React from 'react';
import {
  AgentModeCard,
  GenericCard,
  LogCard,
  MessageCard,
  ToolCallCard,
  ToolResultCard,
  WidgetInstanceCard,
} from '../features/web-chat/cards';
import type { ChatWidgetRenderers, RenderEntity } from './types';

type Renderer = React.ComponentType<{ e: RenderEntity }>;

const defaultRenderers: Record<string, Renderer> = {
  message: MessageCard,
  tool_call: ToolCallCard,
  ChatFrontendToolCall: ToolCallCard,
  tool_result: ToolResultCard,
  log: LogCard,
  agent_mode: AgentModeCard,
  agent_mode_preview: AgentModeCard,
  ChatWidgetInstance: WidgetInstanceCard,
  widget_instance: WidgetInstanceCard,
};

export type WebChatRendererConfig = {
  overrides?: Partial<ChatWidgetRenderers>;
};

export function createWebChatRenderers(config: WebChatRendererConfig = {}): ChatWidgetRenderers {
  const resolved: ChatWidgetRenderers = {
    ...defaultRenderers,
    ...(config.overrides ?? {}),
    default: config.overrides?.default ?? GenericCard,
  };
  return resolved;
}
