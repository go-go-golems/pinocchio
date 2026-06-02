import type { RenderEntity, RenderEntityProps } from '../types';

export function toRenderEntity(value: unknown): RenderEntity {
  const e = asRecord(value);
  const createdAt = Number(e.createdAt ?? 0);
  return {
    id: String(e.id ?? ''),
    kind: String(e.kind ?? ''),
    props: asRecord(e.props) as RenderEntityProps,
    createdAt: Number.isFinite(createdAt) ? createdAt : 0,
    updatedAt: e.updatedAt ? Number(e.updatedAt) : undefined,
  };
}

export function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) return value as Record<string, unknown>;
  return {};
}
