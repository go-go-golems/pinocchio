export interface ConversationSummary {
  conv_id: string;
  session_id: string;
  profile: string;
  active_sockets: number;
  stream_running: boolean;
  queue_depth: number;
  buffered_events: number;
  last_activity_ms: number;
  has_timeline_source: boolean;
}

export interface ConversationDetail extends ConversationSummary {
  active_request_key: string;
}

export interface TurnsEnvelope {
  conv_id: string;
  session_id: string;
  phase: string;
  since_ms: number;
  items: TurnSnapshot[];
}

export interface TurnSnapshot {
  conv_id: string;
  session_id: string;
  turn_id: string;
  phase: string;
  created_at_ms: number;
  payload: string;
}

export interface TurnDetailEnvelope {
  conv_id: string;
  session_id: string;
  turn_id: string;
  items: TurnPhaseSnapshot[];
}

export interface TurnPhaseSnapshot {
  phase: string;
  created_at_ms: number;
  payload: string;
  parsed?: Record<string, unknown>;
  parse_error?: string;
}

export interface EventsEnvelope {
  conv_id: string;
  since_seq: number;
  type: string;
  limit: number;
  items: EventItem[];
}

export interface EventItem {
  seq: number;
  type?: string;
  id?: string;
  frame?: Record<string, unknown>;
  raw?: string;
  decode_error?: string;
}

export interface RunsEnvelope {
  artifacts_root?: string;
  turns_db?: string;
  timeline_db?: string;
  limit: number;
  items: OfflineRunSummary[];
}

export interface OfflineRunSummary {
  run_id: string;
  kind: string;
  display: string;
  source_path: string;
  timestamp_ms?: number;
  conv_id?: string;
  session_id?: string;
  counts?: Record<string, unknown>;
}

export interface RunDetailEnvelope {
  run_id: string;
  kind: string;
  detail: Record<string, unknown>;
}

export interface TimelineSnapshotEnvelope {
  convId?: string;
  version?: number;
  serverTimeMs?: number;
  entities?: unknown[];
  [key: string]: unknown;
}
