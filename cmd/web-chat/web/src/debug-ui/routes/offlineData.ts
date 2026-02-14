import { parseTurnPayload, toParsedTurn } from '../api/turnParsing';
import type {
  ParsedTurn,
  RunDetailResponse,
  SemEvent,
  TimelineEntity,
  TurnDetail,
  TurnPhase,
  TurnSnapshot,
} from '../types';

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function asNumber(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string') {
    const parsed = Number(value.trim());
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

function normalizePhase(raw: unknown): TurnPhase {
  switch (asString(raw)) {
    case 'draft':
      return 'draft';
    case 'pre_inference':
      return 'pre_inference';
    case 'post_inference':
      return 'post_inference';
    case 'post_tools':
      return 'post_tools';
    case 'final':
      return 'final';
    default:
      return 'final';
  }
}

function flattenTimelineProps(entity: Record<string, unknown>): Record<string, unknown> {
  const keys = [
    'message',
    'toolCall',
    'toolResult',
    'status',
    'thinkingMode',
    'modeEvaluation',
    'innerThoughts',
    'teamAnalysis',
    'discoDialogueLine',
    'discoDialogueCheck',
    'discoDialogueState',
  ];
  for (const key of keys) {
    const direct = asRecord(entity[key]);
    if (Object.keys(direct).length > 0) {
      return direct;
    }
  }
  const snapshot = asRecord(entity.snapshot);
  for (const key of keys) {
    const nested = asRecord(snapshot[key]);
    if (Object.keys(nested).length > 0) {
      return nested;
    }
  }
  return asRecord(entity.props);
}

function toTimelineEntity(raw: unknown): TimelineEntity {
  const entity = asRecord(raw);
  return {
    id: asString(entity.id),
    kind: asString(entity.kind),
    created_at: asNumber(entity.createdAtMs ?? entity.created_at_ms ?? entity.createdAt ?? entity.created_at),
    updated_at:
      asNumber(entity.updatedAtMs ?? entity.updated_at_ms ?? entity.updatedAt ?? entity.updated_at) || undefined,
    version: asNumber(entity.version) || undefined,
    props: flattenTimelineProps(entity),
  };
}

function parseTurnFromDetail(value: unknown, fallbackID: string): ParsedTurn {
  const item = asRecord(value);
  if (item.parsed) {
    return toParsedTurn(item.parsed, fallbackID);
  }
  if (item.payload) {
    return parseTurnPayload(item.payload, fallbackID);
  }
  if (item.yaml) {
    return parseTurnPayload(item.yaml, fallbackID);
  }
  return { id: fallbackID, blocks: [], metadata: {}, data: {} };
}

function toSemEvent(raw: unknown, seq: number): SemEvent {
  const item = asRecord(raw);
  const event = asRecord(item.event);
  const eventType = asString(item.type) || asString(event.type) || 'offline.event';
  const eventID = asString(item.id) || asString(event.id) || `offline-${seq}`;
  return {
    type: eventType,
    id: eventID,
    seq,
    stream_id: asString(event.stream_id ?? event.streamId) || undefined,
    data: item.data ?? event.data ?? item,
    received_at: new Date().toISOString(),
  };
}

export interface OfflineInspectorData {
  convID: string;
  sessionID: string;
  turns: TurnSnapshot[];
  events: SemEvent[];
  entities: TimelineEntity[];
}

export function parseOfflineInspectorData(run: RunDetailResponse): OfflineInspectorData {
  const detail = asRecord(run.detail);
  const turns: TurnSnapshot[] = [];
  const events: SemEvent[] = [];
  const entities: TimelineEntity[] = [];

  const convID = asString(detail.conv_id) || `offline:${run.run_id}`;
  const sessionID = asString(detail.session_id) || run.run_id;

  if (run.kind === 'turns') {
    const items = Array.isArray(detail.items) ? detail.items : [];
    for (const itemRaw of items) {
      const item = asRecord(itemRaw);
      const turnID = asString(item.turn_id) || 'turn';
      turns.push({
        conv_id: asString(item.conv_id) || convID,
        session_id: asString(item.session_id) || sessionID,
        turn_id: turnID,
        phase: normalizePhase(item.phase),
        created_at_ms: asNumber(item.created_at_ms),
        turn: parseTurnFromDetail(item, turnID),
      });
    }
  }

  if (run.kind === 'artifact') {
    const inputTurn = asRecord(detail.input_turn);
    if (Object.keys(inputTurn).length > 0) {
      const parsed = parseTurnFromDetail(inputTurn, 'input');
      turns.push({
        conv_id: convID,
        session_id: sessionID,
        turn_id: parsed.id || 'input',
        phase: 'draft',
        created_at_ms: 0,
        turn: parsed,
      });
    }

    const turnItems = Array.isArray(detail.turns) ? detail.turns : [];
    for (const itemRaw of turnItems) {
      const item = asRecord(itemRaw);
      const name = asString(item.name) || 'turn';
      const parsed = parseTurnFromDetail(item, name);
      turns.push({
        conv_id: convID,
        session_id: sessionID,
        turn_id: parsed.id || name,
        phase: 'final',
        created_at_ms: 0,
        turn: parsed,
      });
    }

    const eventFiles = Array.isArray(detail.events) ? detail.events : [];
    let seq = 1;
    for (const fileRaw of eventFiles) {
      const file = asRecord(fileRaw);
      const fileItems = Array.isArray(file.items) ? file.items : [];
      for (const entry of fileItems) {
        events.push(toSemEvent(entry, seq));
        seq++;
      }
    }
  }

  if (run.kind === 'timeline') {
    const snapshot = asRecord(detail.snapshot);
    const rawEntities = Array.isArray(snapshot.entities) ? snapshot.entities : [];
    for (const entityRaw of rawEntities) {
      entities.push(toTimelineEntity(entityRaw));
    }
  }

  return { convID, sessionID, turns, events, entities };
}

export function buildTurnDetail(
  snapshots: TurnSnapshot[],
  convID: string,
  sessionID: string,
  turnID: string
): TurnDetail | null {
  const phases: TurnDetail['phases'] = {};

  for (const snapshot of snapshots) {
    if (snapshot.turn_id !== turnID) {
      continue;
    }
    const existing = phases[snapshot.phase];
    if (!existing || snapshot.created_at_ms >= Date.parse(existing.captured_at)) {
      phases[snapshot.phase] = {
        captured_at:
          snapshot.created_at_ms > 0
            ? new Date(snapshot.created_at_ms).toISOString()
            : new Date(0).toISOString(),
        turn: snapshot.turn,
      };
    }
  }

  if (Object.keys(phases).length === 0) {
    return null;
  }

  return {
    conv_id: convID,
    session_id: sessionID,
    turn_id: turnID,
    phases,
  };
}

