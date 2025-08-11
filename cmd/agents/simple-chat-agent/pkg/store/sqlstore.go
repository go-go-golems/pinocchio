package store

import (
    "context"
    "database/sql"
    "encoding/json"
    _ "embed"
    "time"

    _ "github.com/mattn/go-sqlite3"
    "github.com/pkg/errors"
    "github.com/rs/zerolog/log"
    "github.com/google/uuid"

    "github.com/go-go-golems/geppetto/pkg/turns"
    "github.com/go-go-golems/geppetto/pkg/events"
    geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
)

type SQLiteStore struct {
    db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, errors.Wrap(err, "open sqlite")
    }
    s := &SQLiteStore{db: db}
    if err := s.init(); err != nil {
        return nil, err
    }
    return s, nil
}

func (s *SQLiteStore) init() error {
    if _, err := s.db.Exec(schemaSQL); err != nil {
        return errors.Wrap(err, "init schema")
    }
    if _, err := s.db.Exec(viewsSQL); err != nil {
        return errors.Wrap(err, "init views")
    }
    return nil
}

func (s *SQLiteStore) EnsureRun(ctx context.Context, runID string, metadata map[string]any) error {
    if runID == "" {
        return errors.New("empty run id")
    }
    var exists int
    _ = s.db.QueryRowContext(ctx, "SELECT 1 FROM runs WHERE id=?", runID).Scan(&exists)
    if exists == 1 {
        return nil
    }
    metaJSON, _ := json.Marshal(metadata)
    _, err := s.db.ExecContext(ctx, "INSERT INTO runs(id, created_at, metadata) VALUES(?,?,?)", runID, time.Now().Format(time.RFC3339Nano), string(metaJSON))
    if err != nil { return errors.Wrap(err, "insert run") }
    // store metadata as KV for easier querying
    for k, v := range metadata {
        typ, vt, vj := classifyValue(v)
        _, _ = s.db.ExecContext(ctx, "INSERT OR REPLACE INTO run_metadata_kv(run_id, key, type, value_text, value_json) VALUES(?,?,?,?,?)", runID, k, typ, vt, vj)
    }
    return nil
}

func (s *SQLiteStore) EnsureTurn(ctx context.Context, runID, turnID string, metadata map[string]any) error {
    if runID == "" || turnID == "" {
        return errors.New("empty run/turn id")
    }
    var exists int
    _ = s.db.QueryRowContext(ctx, "SELECT 1 FROM turns WHERE id=?", turnID).Scan(&exists)
    if exists == 1 {
        return nil
    }
    metaJSON, _ := json.Marshal(metadata)
    _, err := s.db.ExecContext(ctx, "INSERT INTO turns(id, run_id, created_at, metadata) VALUES(?,?,?,?)", turnID, runID, time.Now().Format(time.RFC3339Nano), string(metaJSON))
    if err != nil { return errors.Wrap(err, "insert turn") }
    for k, v := range metadata {
        typ, vt, vj := classifyValue(v)
        _, _ = s.db.ExecContext(ctx, "INSERT OR REPLACE INTO turn_kv(turn_id, section, key, type, value_text, value_json) VALUES(?,?,?,?,?,?)", turnID, "metadata", k, typ, vt, vj)
    }
    return nil
}

// SaveTurnSnapshot stores the full turn, including blocks, payloads and metadata, as a JSON snapshot per phase (pre/post).
func (s *SQLiteStore) SaveTurnSnapshot(ctx context.Context, t *turns.Turn, phase string) error {
    if t == nil {
        return nil
    }
    if t.RunID == "" {
        t.RunID = uuid.NewString()
    }
    if t.ID == "" {
        t.ID = uuid.NewString()
    }
    if err := s.EnsureRun(ctx, t.RunID, t.Metadata); err != nil {
        log.Warn().Err(err).Str("run_id", t.RunID).Msg("EnsureRun failed")
    }
    if err := s.EnsureTurn(ctx, t.RunID, t.ID, t.Metadata); err != nil {
        log.Warn().Err(err).Str("turn_id", t.ID).Msg("EnsureTurn failed")
    }
    // Serialize turn as JSON
    type block struct {
        ID       string                 `json:"id"`
        Order    int                    `json:"order"`
        Kind     turns.BlockKind        `json:"kind"`
        Role     string                 `json:"role"`
        Payload  map[string]any         `json:"payload"`
        Metadata map[string]interface{} `json:"metadata,omitempty"`
    }
    snap := struct {
        RunID    string                 `json:"run_id"`
        TurnID   string                 `json:"turn_id"`
        Metadata map[string]interface{} `json:"metadata,omitempty"`
        Data     map[string]interface{} `json:"data,omitempty"`
        Blocks   []block                `json:"blocks"`
    }{
        RunID:    t.RunID,
        TurnID:   t.ID,
        Metadata: t.Metadata,
        Data:     t.Data,
        Blocks:   make([]block, 0, len(t.Blocks)),
    }
    for i, b := range t.Blocks {
        bid := b.ID
        if bid == "" {
            log.Warn().
                Str("run_id", t.RunID).
                Str("turn_id", t.ID).
                Int("index", i).
                Int("kind", int(b.Kind)).
                Str("role", b.Role).
                Msg("SaveTurnSnapshot: block without ID; generating new one")
            bid = uuid.NewString()
            // Persist the generated ID back onto the Turn to keep it stable across phases
            t.Blocks[i].ID = bid
        }
        snap.Blocks = append(snap.Blocks, block{
            ID:       bid,
            Order:    i,
            Kind:     b.Kind,
            Role:     b.Role,
            Payload:  b.Payload,
            Metadata: b.Metadata,
        })
        // upsert block row
        // ord is derived from position in slice
        _, _ = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO blocks(id, turn_id, ord, kind, role, created_at) VALUES(?,?,?,?,?,?)", bid, t.ID, i, int(b.Kind), b.Role, time.Now().Format(time.RFC3339Nano))
        // payload kv
        for pk, pv := range b.Payload {
            typ, vt, vj := classifyValue(pv)
            _, _ = s.db.ExecContext(ctx, "INSERT OR REPLACE INTO block_payload_kv(block_id, turn_id, phase, key, type, value_text, value_json) VALUES(?,?,?,?,?,?,?)", bid, t.ID, phase, pk, typ, vt, vj)
        }
        // metadata kv
        for mk, mv := range b.Metadata {
            typ, vt, vj := classifyValue(mv)
            _, _ = s.db.ExecContext(ctx, "INSERT OR REPLACE INTO block_metadata_kv(block_id, turn_id, phase, key, type, value_text, value_json) VALUES(?,?,?,?,?,?,?)", bid, t.ID, phase, mk, typ, vt, vj)
        }
    }
    // Persist tool registry definitions as JSON, if present
    if regAny, ok := t.Data[turns.DataKeyToolRegistry]; ok && regAny != nil {
        if reg, ok := regAny.(geptools.ToolRegistry); ok && reg != nil {
            defs := reg.ListTools()
            if b, err := json.Marshal(defs); err == nil {
                _, _ = s.db.ExecContext(ctx, "INSERT OR REPLACE INTO turn_kv(turn_id, section, key, type, value_text, value_json) VALUES(?,?,?,?,?,?)", t.ID, "data", "tool_registry", "object", "", string(b))
                // Also record a registry snapshot row for easier retrieval
                _, _ = s.db.ExecContext(ctx, "INSERT INTO tool_registry_snapshots(run_id, turn_id, phase, created_at, tools_json) VALUES(?,?,?,?,?)", t.RunID, t.ID, phase, time.Now().Format(time.RFC3339Nano), string(b))
            }
        }
    }
    // turn data KV (skip raw registry object to avoid overwriting curated JSON)
    for k, v := range t.Data {
        if k == turns.DataKeyToolRegistry {
            continue
        }
        typ, vt, vj := classifyValue(v)
        _, _ = s.db.ExecContext(ctx, "INSERT OR REPLACE INTO turn_kv(turn_id, section, key, type, value_text, value_json) VALUES(?,?,?,?,?,?)", t.ID, "data", k, typ, vt, vj)
    }
    js, _ := json.Marshal(snap)
    _, err := s.db.ExecContext(ctx, "INSERT INTO turn_snapshots(turn_id, phase, created_at, data) VALUES(?,?,?,?)", t.ID, phase, time.Now().Format(time.RFC3339Nano), string(js))
    return errors.Wrap(err, "insert turn snapshot")
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

// classifyValue separates a generic interface{} into a type label and storage-friendly string fields.
func classifyValue(v any) (typ string, valueText string, valueJSON string) {
    switch x := v.(type) {
    case nil:
        return "null", "", "null"
    case string:
        return "string", x, ""
    case bool:
        if x {
            return "bool", "true", "true"
        }
        return "bool", "false", "false"
    case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
        b, _ := json.Marshal(x)
        return "number", string(b), string(b)
    default:
        b, _ := json.Marshal(v)
        return "object", "", string(b)
    }
}

//go:embed schema.sql
var schemaSQL string

//go:embed views.sql
var viewsSQL string

// LogEvent persists a chat event (tool/log/info) to sqlite for debugging.
func (s *SQLiteStore) LogEvent(ctx context.Context, ev events.Event) {
    if s == nil || ev == nil { return }
    kind := string(ev.Type())
    now := time.Now().Format(time.RFC3339Nano)
    var message, level, toolName, toolID, input, result, runID, turnID string
    var dataJSON, payloadJSON string

    // keep original payload
    if p := ev.Payload(); len(p) > 0 { payloadJSON = string(p) }

    switch e := ev.(type) {
    case *events.EventToolCall:
        toolName = e.ToolCall.Name
        toolID = e.ToolCall.ID
        input = e.ToolCall.Input
    case *events.EventToolCallExecute:
        toolName = e.ToolCall.Name
        toolID = e.ToolCall.ID
        input = e.ToolCall.Input
    case *events.EventToolResult:
        toolID = e.ToolResult.ID
        result = e.ToolResult.Result
    case *events.EventToolCallExecutionResult:
        toolID = e.ToolResult.ID
        result = e.ToolResult.Result
    case *events.EventLog:
        level = e.Level
        message = e.Message
        if len(e.Fields) > 0 { if b, _ := json.Marshal(e.Fields); b != nil { dataJSON = string(b) } }
    case *events.EventInfo:
        message = e.Message
        if len(e.Data) > 0 { if b, _ := json.Marshal(e.Data); b != nil { dataJSON = string(b) } }
    }

    // extract run/turn id from EventMetadata
    meta := ev.Metadata()
    if meta.RunID != "" { runID = meta.RunID }
    if meta.TurnID != "" { turnID = meta.TurnID }

    _, _ = s.db.ExecContext(ctx, `INSERT INTO chat_events(
        created_at, type, message, level, tool_name, tool_id, input, result, data_json, payload_json, run_id, turn_id
    ) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
        now, kind, message, level, toolName, toolID, input, result, dataJSON, payloadJSON, runID, turnID,
    )
}


