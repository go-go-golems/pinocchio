import { fromJson, type Message } from '@bufbuild/protobuf';
import type { GenMessage } from '@bufbuild/protobuf/codegenv2';
import {
  type TimelineSnapshotV1,
  TimelineSnapshotV1Schema,
  type TimelineUpsertV1,
  TimelineUpsertV1Schema,
} from '../../sem/pb/proto/sem/timeline/transport_pb';
import { timelineEntityFromProto } from '../../sem/timelineMapper';
import { toNumber } from '../../utils/number';
import { debugApi } from '../api/debugApi';
import type { AppDispatch } from '../store/store';
import { setFollowError, setFollowStatus } from '../store/uiSlice';
import type { EventsResponse, SemEvent, TimelineEntity, TimelineSnapshot } from '../types';

type ConnectArgs = {
  convId: string;
  basePrefix: string;
  dispatch: AppDispatch;
};

type RawSemEnvelope = {
  sem?: boolean;
  event?: Record<string, unknown>;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function decodeProto<T extends Message>(schema: GenMessage<T>, raw: unknown): T | null {
  if (!isRecord(raw)) {
    return null;
  }
  try {
    return fromJson(schema as any, raw as any, { ignoreUnknownFields: true }) as T;
  } catch {
    return null;
  }
}

function seqFromEnvelope(envelope: RawSemEnvelope): number | undefined {
  const raw = envelope?.event?.seq;
  return toNumber(raw);
}

function toDebugTimelineEntity(raw: TimelineUpsertV1['entity'], version: unknown): TimelineEntity | null {
  if (!raw) {
    return null;
  }
  const mapped = timelineEntityFromProto(raw, version);
  if (!mapped) {
    return null;
  }
  return {
    id: mapped.id,
    kind: mapped.kind,
    created_at: mapped.createdAt,
    updated_at: mapped.updatedAt,
    version: mapped.version,
    props: mapped.props as Record<string, unknown>,
  };
}

function toDebugTimelineSnapshot(snapshot: TimelineSnapshotV1): TimelineSnapshot {
  const entities: TimelineEntity[] = [];
  for (const entity of snapshot.entities ?? []) {
    const mapped = toDebugTimelineEntity(entity, snapshot.version);
    if (!mapped) {
      continue;
    }
    entities.push(mapped);
  }
  return {
    entities,
    version: toNumber(snapshot.version) ?? 0,
  };
}

function maxEntityVersion(snapshot: TimelineSnapshot): number {
  let maxVersion = toNumber(snapshot.version) ?? 0;
  for (const entity of snapshot.entities) {
    const entityVersion = toNumber(entity.version) ?? 0;
    if (entityVersion > maxVersion) {
      maxVersion = entityVersion;
    }
  }
  return maxVersion;
}

function toSemEventFromEnvelope(envelope: RawSemEnvelope, fallbackSeq: number): SemEvent {
  const ev = envelope.event ?? {};
  const seq = toNumber(ev.seq) ?? fallbackSeq;
  const type = typeof ev.type === 'string' ? ev.type : 'unknown';
  const id = typeof ev.id === 'string' ? ev.id : `${type}:${seq}`;
  const streamID =
    typeof ev.stream_id === 'string'
      ? ev.stream_id
      : typeof ev.streamId === 'string'
        ? ev.streamId
        : undefined;
  return {
    type,
    id,
    seq,
    stream_id: streamID,
    data: ev.data ?? {},
    received_at: new Date().toISOString(),
  };
}

export class DebugTimelineWsManager {
  private ws: WebSocket | null = null;
  private convId = '';
  private connectNonce = 0;
  private bootstrapped = false;
  private buffered: RawSemEnvelope[] = [];
  private highWaterVersion = 0;
  private syntheticSeq = 1;

  disconnect() {
    this.connectNonce++;
    try {
      this.ws?.close();
    } catch {
      // no-op
    }
    this.ws = null;
    this.convId = '';
    this.bootstrapped = false;
    this.buffered = [];
    this.highWaterVersion = 0;
    this.syntheticSeq = 1;
  }

  async connect(args: ConnectArgs) {
    if (!args.convId) {
      return;
    }

    if (this.ws && this.convId === args.convId && this.bootstrapped) {
      return;
    }

    this.disconnect();
    this.connectNonce++;
    const nonce = this.connectNonce;

    this.convId = args.convId;
    args.dispatch(setFollowError(null));
    args.dispatch(setFollowStatus('connecting'));
    this.ensureCacheEntries(args.dispatch, args.convId);

    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${window.location.host}${args.basePrefix}/ws?conv_id=${encodeURIComponent(args.convId)}`;
    const ws = new WebSocket(url);
    this.ws = ws;

    let settleOpen: (() => void) | null = null;
    const openPromise = new Promise<void>((resolve) => {
      let settled = false;
      settleOpen = () => {
        if (settled) {
          return;
        }
        settled = true;
        resolve();
      };
      setTimeout(() => settleOpen?.(), 2000);
    });

    ws.onopen = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) {
        return;
      }
      args.dispatch(setFollowStatus('connecting'));
    };
    // Read-only follow mode: no outbound websocket messages are ever sent.
    ws.onclose = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) {
        return;
      }
      args.dispatch(setFollowStatus('closed'));
    };
    ws.onerror = () => {
      settleOpen?.();
      if (nonce !== this.connectNonce) {
        return;
      }
      args.dispatch(setFollowError('websocket error'));
    };
    ws.onmessage = (message) => {
      if (nonce !== this.connectNonce) {
        return;
      }
      let envelope: RawSemEnvelope;
      try {
        envelope = JSON.parse(String(message.data)) as RawSemEnvelope;
      } catch {
        return;
      }
      if (!this.bootstrapped) {
        this.buffered.push(envelope);
        return;
      }
      this.applyEnvelope(envelope, args.dispatch, args.convId);
    };

    await openPromise;
    if (nonce !== this.connectNonce) {
      return;
    }

    args.dispatch(setFollowStatus('bootstrapping'));
    const bootstrap = await this.bootstrapFromTimeline(args);
    if (!bootstrap.ok) {
      if (nonce !== this.connectNonce) {
        return;
      }
      args.dispatch(setFollowError(bootstrap.error));
      this.disconnect();
      return;
    }
    if (nonce !== this.connectNonce) {
      return;
    }

    this.highWaterVersion = bootstrap.highWaterVersion;
    this.bootstrapped = true;
    args.dispatch(setFollowStatus('connected'));

    const buffered = this.buffered;
    this.buffered = [];
    buffered.sort((a, b) => (seqFromEnvelope(a) ?? 0) - (seqFromEnvelope(b) ?? 0));
    for (const envelope of buffered) {
      this.applyEnvelope(envelope, args.dispatch, args.convId);
    }
  }

  private ensureCacheEntries(dispatch: AppDispatch, convId: string) {
    dispatch(debugApi.util.upsertQueryData('getTimeline', { convId }, { entities: [], version: 0 }));
    dispatch(
      debugApi.util.upsertQueryData('getEvents', { convId }, {
        events: [],
        total: 0,
        buffer_capacity: 0,
      } as EventsResponse)
    );
  }

  private async bootstrapFromTimeline(
    args: ConnectArgs
  ): Promise<{ ok: true; highWaterVersion: number } | { ok: false; error: string }> {
    let response: Response;
    try {
      response = await fetch(`${args.basePrefix}/api/timeline?conv_id=${encodeURIComponent(args.convId)}`);
    } catch {
      return { ok: false, error: 'timeline bootstrap failed' };
    }
    if (!response.ok) {
      return { ok: false, error: `timeline bootstrap http ${response.status}` };
    }

    let payload: unknown;
    try {
      payload = await response.json();
    } catch {
      return { ok: false, error: 'timeline bootstrap json parse failed' };
    }
    if (!isRecord(payload)) {
      return { ok: false, error: 'timeline bootstrap payload invalid' };
    }
    const proto = decodeProto<TimelineSnapshotV1>(TimelineSnapshotV1Schema, payload);
    if (!proto) {
      return { ok: false, error: 'timeline bootstrap proto decode failed' };
    }

    const snapshot = toDebugTimelineSnapshot(proto);
    this.syntheticSeq = maxEntityVersion(snapshot) + 1;
    args.dispatch(debugApi.util.upsertQueryData('getTimeline', { convId: args.convId }, snapshot));
    return { ok: true, highWaterVersion: maxEntityVersion(snapshot) };
  }

  private applyEnvelope(envelope: RawSemEnvelope, dispatch: AppDispatch, convId: string) {
    if (!envelope.sem || !isRecord(envelope.event)) {
      return;
    }

    const eventType = typeof envelope.event.type === 'string' ? envelope.event.type : '';
    if (eventType !== 'timeline.upsert') {
      return;
    }

    const seq = seqFromEnvelope(envelope);
    if (typeof seq === 'number' && seq <= this.highWaterVersion) {
      return;
    }

    const upsert = decodeProto<TimelineUpsertV1>(TimelineUpsertV1Schema, envelope.event.data);
    const version = toNumber(upsert?.version) ?? seq ?? 0;
    const entity = toDebugTimelineEntity(upsert?.entity, version);
    if (!entity) {
      return;
    }

    dispatch(
      debugApi.util.updateQueryData('getTimeline', { convId }, (draft) => {
        const idx = draft.entities.findIndex((e) => e.id === entity.id);
        if (idx < 0) {
          draft.entities.push(entity);
        } else {
          const existing = draft.entities[idx];
          const incomingVersion = toNumber(entity.version) ?? 0;
          const existingVersion = toNumber(existing.version) ?? 0;
          if (incomingVersion < existingVersion) {
            return;
          }
          draft.entities[idx] = {
            ...existing,
            ...entity,
            props: { ...(existing.props ?? {}), ...(entity.props ?? {}) },
          };
        }

        const draftVersion = toNumber(draft.version) ?? 0;
        const entityVersion = toNumber(entity.version) ?? 0;
        if (entityVersion > draftVersion) {
          draft.version = entityVersion;
        }
      })
    );

    const semEvent = toSemEventFromEnvelope(envelope, this.syntheticSeq++);
    dispatch(
      debugApi.util.updateQueryData('getEvents', { convId }, (draft) => {
        const exists = draft.events.some((event) => event.seq === semEvent.seq && event.id === semEvent.id);
        if (exists) {
          return;
        }
        draft.events.push(semEvent);
        draft.total = draft.events.length;
        if (draft.buffer_capacity < draft.events.length) {
          draft.buffer_capacity = draft.events.length;
        }
      })
    );

    const nextHighWater = Math.max(
      this.highWaterVersion,
      toNumber(entity.version) ?? 0,
      toNumber(semEvent.seq) ?? 0
    );
    this.highWaterVersion = nextHighWater;
  }
}

export const debugTimelineWsManager = new DebugTimelineWsManager();
