import { appSlice } from '../store/appSlice';
import type { AppDispatch } from '../store/store';
import { type TimelineEntity, timelineSlice } from '../store/timelineSlice';
import { asRecord, asString, type CanonicalFrame, type SnapshotEntityFrame, unwrapAnyPayload } from './protocol';
import { recordSnapshotDebug } from './streamDebug';

export function messageEntity(id: string, props: Record<string, unknown>): TimelineEntity {
  return {
    id,
    kind: 'message',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    props,
  };
}

export function agentModeEntity(id: string, kind: 'agent_mode' | 'agent_mode_preview', props: Record<string, unknown>): TimelineEntity {
  return {
    id,
    kind,
    createdAt: Date.now(),
    updatedAt: Date.now(),
    props,
  };
}

export function agentModePreviewEntityId(messageId: string): string {
  return `agent-mode-preview:${messageId}`;
}

export function timelineEntityFromSnapshotEntity(entity: SnapshotEntityFrame): TimelineEntity | null {
  const kind = asString(entity?.kind);
  const id = asString(entity?.id);
  const payload = unwrapAnyPayload(entity?.payload);
  if (!id) return null;

  if (kind === 'ChatMessage') {
    const messageId = asString(payload.messageId) || id;
    return messageEntity(messageId, {
      role: asString(payload.role) || 'assistant',
      prompt: asString(payload.prompt),
      content: asString(payload.content) || asString(payload.text),
      status: asString(payload.status) || 'idle',
      streaming: payload.streaming === true,
      error: asString(payload.error),
    });
  }

  if (kind === 'AgentMode') {
    const data = asRecord(payload.data);
    const flattenedData = Object.keys(data).length > 0
      ? data
      : {
          from: asString(payload.from),
          to: asString(payload.to),
          analysis: asString(payload.analysis),
        };
    return agentModeEntity(id || 'agent-mode', 'agent_mode', {
      title: asString(payload.title) || 'Agent mode switch',
      data: flattenedData,
      preview: payload.preview === true,
      messageId: asString(payload.messageId),
    });
  }

  return {
    id,
    kind: kind || 'system',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    props: payload,
  };
}

export function applySnapshot(frame: CanonicalFrame, dispatch: AppDispatch, sessionId = '') {
  dispatch(timelineSlice.actions.clear());
  const entities = Array.isArray(frame.entities) ? (frame.entities as SnapshotEntityFrame[]) : [];
  const mappedEntities: Array<{ raw: SnapshotEntityFrame; mapped: TimelineEntity | null }> = [];
  let status = 'idle';
  for (const entity of entities) {
    const mapped = timelineEntityFromSnapshotEntity(entity);
    mappedEntities.push({ raw: entity, mapped });
    if (!mapped) continue;
    dispatch(timelineSlice.actions.upsertEntity(mapped));
    if (mapped.kind === 'message') {
      const nextStatus = asString(mapped.props?.status);
      if (nextStatus) status = nextStatus;
    }
  }
  dispatch(appSlice.actions.setStatus(status));
  recordSnapshotDebug(sessionId, frame.ordinal, mappedEntities);
}
