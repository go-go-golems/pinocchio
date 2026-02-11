import type { TimelineEntityV1 } from '../sem/pb/proto/sem/timeline/transport_pb';
import type { TimelineEntity } from '../store/timelineSlice';
import { toNumber, toNumberOr } from '../utils/number';

function isObject(v: unknown): v is Record<string, any> {
  return !!v && typeof v === 'object';
}

export function propsFromTimelineEntity(e: TimelineEntityV1): any {
  const kind = e.kind;
  const snap = (e as any).snapshot;
  if (!snap || !isObject(snap)) return {};

  // bufbuild/es represents oneofs as { case, value }.
  const oneof = snap as any;
  const val = oneof.value;

  if (kind === 'message' && oneof.case === 'message') {
    return { role: val?.role, content: val?.content, streaming: !!val?.streaming };
  }
  if (kind === 'tool_call' && oneof.case === 'toolCall') {
    return { name: val?.name, input: val?.input ?? {}, status: val?.status, progress: val?.progress, done: !!val?.done };
  }
  if (kind === 'tool_result' && oneof.case === 'toolResult') {
    return { result: val?.resultRaw ?? '', customKind: val?.customKind ?? '' };
  }
  if (kind === 'thinking_mode' && oneof.case === 'thinkingMode') {
    const status = val?.status ?? '';
    const success = status === 'completed' ? true : status === 'error' ? false : undefined;
    return {
      status,
      mode: val?.mode,
      phase: val?.phase,
      reasoning: val?.reasoning,
      success: typeof val?.success === 'boolean' ? val.success : success,
      error: val?.error ?? '',
    };
  }
  if (kind === 'disco_dialogue_line' && oneof.case === 'discoDialogueLine') {
    const status = val?.status ?? '';
    const success = status === 'completed' ? true : status === 'error' ? false : undefined;
    return {
      status,
      dialogueId: val?.dialogueId ?? '',
      lineId: val?.lineId ?? '',
      persona: val?.persona ?? '',
      tone: val?.tone ?? '',
      text: val?.text ?? '',
      trigger: val?.trigger ?? '',
      progress: toNumber(val?.progress),
      success: typeof val?.success === 'boolean' ? val.success : success,
      error: val?.error ?? '',
    };
  }
  if (kind === 'disco_dialogue_check' && oneof.case === 'discoDialogueCheck') {
    const status = val?.status ?? '';
    const success = status === 'completed' ? true : status === 'error' ? false : undefined;
    return {
      status,
      dialogueId: val?.dialogueId ?? '',
      lineId: val?.lineId ?? '',
      checkType: val?.checkType ?? '',
      skill: val?.skill ?? '',
      difficulty: toNumber(val?.difficulty),
      roll: toNumber(val?.roll),
      success: typeof val?.success === 'boolean' ? val.success : success,
      error: val?.error ?? '',
    };
  }
  if (kind === 'disco_dialogue_state' && oneof.case === 'discoDialogueState') {
    const status = val?.status ?? '';
    const success = status === 'completed' ? true : status === 'error' ? false : undefined;
    return {
      status,
      dialogueId: val?.dialogueId ?? '',
      summary: val?.summary ?? '',
      success: typeof val?.success === 'boolean' ? val.success : success,
      error: val?.error ?? '',
    };
  }
  if (kind === 'status' && oneof.case === 'status') {
    return { text: val?.text, type: val?.type };
  }
  if (kind === 'team_analysis' && oneof.case === 'teamAnalysis') {
    return val ?? {};
  }

  return val ?? {};
}

export function timelineEntityFromProto(e: TimelineEntityV1, version?: unknown): TimelineEntity | null {
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
