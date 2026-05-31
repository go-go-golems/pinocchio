import type { RenderEntity } from '../../webchat/types';

export function toRenderEntity(e: any): RenderEntity {
  const createdAt = Number(e?.createdAt ?? 0);
  return {
    id: String(e?.id ?? ''),
    kind: String(e?.kind ?? ''),
    props: e?.props ?? {},
    createdAt: Number.isFinite(createdAt) ? createdAt : 0,
    updatedAt: e?.updatedAt ? Number(e.updatedAt) : undefined,
  };
}

export function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) return value as Record<string, unknown>;
  return {};
}
