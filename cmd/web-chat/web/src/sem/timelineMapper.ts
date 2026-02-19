import type { TimelineEntityV2 } from '../sem/pb/proto/sem/timeline/transport_pb';
import type { TimelineEntity } from '../store/timelineSlice';
import { toNumber, toNumberOr } from '../utils/number';

function isObject(v: unknown): v is Record<string, unknown> {
  return !!v && typeof v === 'object' && !Array.isArray(v);
}

function asString(v: unknown): string {
  return typeof v === 'string' ? v : '';
}

function asBoolean(v: unknown): boolean | undefined {
  return typeof v === 'boolean' ? v : undefined;
}

export function propsFromTimelineEntity(e: TimelineEntityV2): Record<string, unknown> {
  const base = isObject(e?.props) ? { ...e.props } : {};

  if (e?.kind === 'tool_result') {
    const resultRaw = asString(base.resultRaw);
    return {
      ...base,
      customKind: asString(base.customKind),
      // Keep UI cards readable even when structured result object exists.
      result: resultRaw || base.result || '',
    };
  }

  if (e?.kind === 'thinking_mode') {
    const status = asString(base.status);
    const successFromStatus = status === 'completed' ? true : status === 'error' ? false : undefined;
    const success = asBoolean(base.success);
    return {
      ...base,
      status,
      success: typeof success === 'boolean' ? success : successFromStatus,
      error: asString(base.error),
    };
  }

  return base;
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
