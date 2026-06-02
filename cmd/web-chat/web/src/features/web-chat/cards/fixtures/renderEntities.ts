import type { RenderEntity } from '../../types';

const now = Date.parse('2026-05-31T12:00:00Z');

export function renderEntity(kind: string, id: string, props: Record<string, unknown> = {}): RenderEntity {
  return {
    id,
    kind,
    props,
    createdAt: now,
    updatedAt: now,
  };
}

export function messageEntity(id: string, props: Record<string, unknown> = {}): RenderEntity {
  return renderEntity('message', id, props);
}

export function toolCallEntity(id: string, props: Record<string, unknown> = {}): RenderEntity {
  return renderEntity('tool_call', id, props);
}

export function toolResultEntity(id: string, props: Record<string, unknown> = {}): RenderEntity {
  return renderEntity('tool_result', id, props);
}

export function agentModeEntity(id: string, props: Record<string, unknown> = {}): RenderEntity {
  return renderEntity('agent_mode', id, props);
}

export function widgetEntity(id: string, props: Record<string, unknown> = {}): RenderEntity {
  return renderEntity('widget_instance', id, props);
}
