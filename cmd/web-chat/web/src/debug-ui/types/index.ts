export interface ConversationSummary {
  id: string;
  profile_slug: string;
  session_id: string;
  engine_config_sig: string;
  is_running: boolean;
  ws_connections: number;
  last_activity: string;
  turn_count: number;
  has_timeline: boolean;
}

export interface ConversationDetail extends ConversationSummary {
  engine_config: EngineConfig;
}

export interface EngineConfig {
  profile_slug: string;
  system_prompt: string;
  middlewares: { name: string; config?: Record<string, unknown> }[];
  tools: string[];
}

export interface SessionSummary {
  session_id: string;
  turn_count: number;
  first_turn_at: string;
  last_turn_at: string;
}

export interface TurnSnapshot {
  conv_id: string;
  session_id: string;
  turn_id: string;
  phase: TurnPhase;
  created_at_ms: number;
  turn: ParsedTurn;
}

export type TurnPhase =
  | 'draft'
  | 'pre_inference'
  | 'post_inference'
  | 'post_tools'
  | 'final';

export interface ParsedTurn {
  id: string;
  blocks: ParsedBlock[];
  metadata: Record<string, unknown>;
  data: Record<string, unknown>;
}

export interface ParsedBlock {
  index: number;
  id?: string;
  kind: BlockKind;
  role?: string;
  payload: Record<string, unknown>;
  metadata: Record<string, unknown>;
}

export type BlockKind =
  | 'system'
  | 'user'
  | 'llm_text'
  | 'tool_call'
  | 'tool_use'
  | 'reasoning'
  | 'other';

export interface SemEvent {
  type: string;
  id: string;
  seq: number;
  stream_id?: string;
  data: unknown;
  received_at: string;
}

export interface EventsResponse {
  events: SemEvent[];
  total: number;
  buffer_capacity: number;
}

export interface TurnDetail {
  conv_id: string;
  session_id: string;
  turn_id: string;
  phases: Partial<Record<TurnPhase, PhaseSnapshot>>;
}

export interface PhaseSnapshot {
  captured_at: string;
  turn: ParsedTurn;
}

export interface MwTrace {
  conv_id: string;
  inference_id: string;
  chain: MwTraceLayer[];
  engine: EngineStats;
}

export interface MwTraceLayer {
  layer: number;
  name: string;
  pre_blocks: number;
  post_blocks: number;
  blocks_added: number;
  blocks_removed: number;
  blocks_changed: number;
  changed_blocks?: { index: number; kind: string; change: string }[];
  metadata_changes: string[];
  duration_ms: number;
}

export interface EngineStats {
  model: string;
  input_blocks: number;
  output_blocks: number;
  latency_ms: number;
  tokens_in: number;
  tokens_out: number;
  stop_reason: string;
}

export interface TimelineEntity {
  id: string;
  kind: string;
  created_at: number;
  updated_at?: number;
  version?: number;
  props: Record<string, unknown>;
}

export interface TimelineSnapshot {
  entities: TimelineEntity[];
  version: number;
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

export interface RunsResponse {
  artifacts_root?: string;
  turns_db?: string;
  timeline_db?: string;
  limit: number;
  items: OfflineRunSummary[];
}

export interface RunDetailResponse {
  run_id: string;
  kind: string;
  detail: Record<string, unknown>;
}

export interface Anomaly {
  id: string;
  severity: 'warning' | 'info' | 'error';
  title: string;
  description: string;
  location: {
    convId: string;
    turnId?: string;
    seq?: number;
    blockIndex?: number;
  };
}
