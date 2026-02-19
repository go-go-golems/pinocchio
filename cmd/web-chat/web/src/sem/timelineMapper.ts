import type { TimelineEntityV2 } from '../sem/pb/proto/sem/timeline/transport_pb';
import type { TimelineEntity } from '../store/timelineSlice';
import { toNumber, toNumberOr } from '../utils/number';
import { normalizeTimelineProps } from './timelinePropsRegistry';

function isObject(v: unknown): v is Record<string, unknown> {
  return !!v && typeof v === 'object' && !Array.isArray(v);
}

export function propsFromTimelineEntity(e: TimelineEntityV2): Record<string, unknown> {
  const base = isObject(e?.props) ? { ...e.props } : {};
  return normalizeTimelineProps(e?.kind ?? '', base);
}

export function timelineEntityFromProto(e: TimelineEntityV2, version?: unknown): TimelineEntity | null {
  if (!e?.id || !e?.kind) return null;
  const createdAt = toNumberOr((e as any).createdAtMs, Date.now());
  const updatedAt = toNumber((e as any).updatedAtMs) || undefined;
  const versionNum = toNumber(version);
  return {
    id: e.id,
    kind: e.kind,
    createdAt,
    updatedAt,
    version: typeof versionNum === 'number' ? versionNum : undefined,
    props: propsFromTimelineEntity(e),
  };
}
