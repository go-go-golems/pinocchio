## Better design for storing Turns and Blocks with Entgo (porting current SQLite KV model)

This document proposes how to port the current SQLite-based KV schema used by the simple chat agent to Entgo, while preserving existing features and making `debug_commands.go` easier to maintain and evolve.

### Scope and sources

- Current write path: `pinocchio/cmd/agents/simple-chat-agent/pkg/store/sqlstore.go`
- Current read/inspect path: `pinocchio/cmd/agents/simple-chat-agent/pkg/ui/debug_commands.go`
- Conceptual model: `geppetto/pkg/doc/topics/08-turns.md`

Goals:
- Keep the provider-agnostic Run/Turn/Block model and the phase-aware KV snapshots intact.
- Introduce Entgo models for core entities and KVs, plus helper repositories to replace most hand-written SQL.
- Preserve existing SQLite views and raw queries where Ent is not a great fit (complex views, compact inspectors), then gradually reduce raw SQL.

## 1) What we have today

From `sqlstore.go` and the UI readers in `debug_commands.go`:

- **Tables (inferred)**
  - `runs(id, created_at, metadata JSON)` and `run_metadata_kv(run_id, key, type, value_text, value_json)`
  - `turns(id, run_id, created_at, metadata JSON)` and `turn_kv(turn_id, section, key, type, value_text, value_json)`
  - `blocks(id, turn_id, ord, kind, role, created_at)`
  - `block_payload_kv(block_id, turn_id, phase, key, type, value_text, value_json)`
  - `block_metadata_kv(block_id, turn_id, phase, key, type, value_text, value_json)`
  - `tool_registry_snapshots(run_id, turn_id, phase, created_at, tools_json)`
  - `turn_snapshots(turn_id, phase, created_at, data JSON)`
  - `chat_events(id, created_at, type, level, message, tool_name, tool_id, input, result, data_json, payload_json, run_id, turn_id)`
  - Plus views installed from `views.sql` (e.g., `v_turn_modes`, `v_injected_mode_prompts`).

- **Write semantics** (from `SaveTurnSnapshot`):
  - Ensure `Run` and `Turn` rows exist (ids are UUIDs if missing).
  - For each `Block` in order, upsert `blocks` and write KV rows for payload and metadata, tagged by `phase`.
  - For `Turn.Data[turns.DataKeyToolRegistry]`, persist curated JSON of tool definitions and also append a `tool_registry_snapshots` row.
  - Write `Turn.Data` and selected `Turn.Metadata` entries into `turn_kv`.
  - Append a serialized `turn_snapshots` JSON row for the whole turn.
  - `LogEvent` appends to `chat_events`.

- **Read/inspect** (from `debug_commands.go`):
  - `runs`, `turns` listing by latest `created_at`.
  - `blocks` printer: selects blocks for a turn and prints tool names/ids, args/results, text, and optional metadata, optionally per `phase`. For `tool_use` blocks, it can look up the matching `tool_call` args.
  - `toolcalls` printer lists tool calls with args/results.
  - `tools` tries to show tool registry snapshots; falls back to distinct tool names from blocks; can also read `turn_kv(section='data', key='tool_registry')`.
  - `events` lists recent `chat_events` rows.
  - Some features rely on views `v_turn_modes`, `v_injected_mode_prompts`.

## 2) Data model with Entgo

We can keep table names for backward compatibility by setting Ent annotations. The model mirrors the existing shape and keeps the `phase` dimension for KVs.

### Core entities

- Run
  - id: string (UUID), created_at: time, metadata: JSON (optional)
  - edges: turns, run_metadata (KV)

- Turn
  - id: string (UUID), run_id FK, created_at: time, metadata: JSON (optional), data: JSON (optional)
  - edges: run, blocks, turn_kv (sectioned), tool_registry_snapshots

- Block
  - id: string (UUID), turn_id FK, ord: int (unique per turn), kind: enum { user, llm_text, tool_call, tool_use, system, other }, role: string?, created_at: time
  - edges: turn, payload_kv, metadata_kv

### KV entities (phase-aware)

- RunMetadataKV
  - run_id FK, key, type, value_text, value_json
  - Unique(run_id, key)

- TurnKV
  - turn_id FK, section in { metadata, data }, key, type, value_text, value_json
  - Unique(turn_id, section, key)

- BlockPayloadKV, BlockMetadataKV
  - block_id FK, turn_id FK, phase string, key, type, value_text, value_json
  - Index(turn_id), Index(block_id, turn_id), Index(turn_id, phase), optional Unique(block_id, turn_id, phase, key)

### Snapshots and events

- ToolRegistrySnapshot
  - run_id, turn_id, phase, created_at, tools_json

- TurnSnapshot
  - turn_id, phase, created_at, data JSON

- ChatEvent
  - id, created_at, type, level, message, tool_name, tool_id, input, result, data_json, payload_json, run_id, turn_id

### Views

- Keep existing `views.sql` for `v_turn_modes`, `v_injected_mode_prompts`. With Ent, we can run these SQL statements after `client.Schema.Create()`.

## 3) Ent schema snippets (pseudocode)

Note: Ent supports JSON fields (as strings for SQLite) and naming via annotations to match existing tables.

```go
// ent/schema/run.go
type Run struct{ ent.Schema }
func (Run) Fields() []ent.Field {
  return []ent.Field{
    field.String("id").Immutable().NotEmpty().DefaultFunc(uuidNew),
    field.Time("created_at").Default(time.Now),
    field.JSON("metadata", map[string]any{}).Optional().Nillable(),
  }
}
func (Run) Edges() []ent.Edge {
  return []ent.Edge{
    edge.To("turns", Turn.Type),
    edge.To("metadata", RunMetadataKV.Type),
  }
}
```

```go
// ent/schema/turn.go
type Turn struct{ ent.Schema }
func (Turn) Fields() []ent.Field {
  return []ent.Field{
    field.String("id").Immutable().DefaultFunc(uuidNew),
    field.String("run_id"),
    field.Time("created_at").Default(time.Now),
    field.JSON("metadata", map[string]any{}).Optional().Nillable(),
    field.JSON("data", map[string]any{}).Optional().Nillable(),
  }
}
func (Turn) Edges() []ent.Edge {
  return []ent.Edge{
    edge.From("run", Run.Type).Ref("turns").Field("run_id").Unique().Required(),
    edge.To("blocks", Block.Type),
    edge.To("kv", TurnKV.Type),
    edge.To("tool_registry_snapshots", ToolRegistrySnapshot.Type),
  }
}
```

```go
// ent/schema/block.go
type BlockKind string
const (
  BKUser BlockKind = "user"
  BKLLM  BlockKind = "llm_text"
  BKCall BlockKind = "tool_call"
  BKUse  BlockKind = "tool_use"
  BKSys  BlockKind = "system"
  BKOther BlockKind = "other"
)
type Block struct{ ent.Schema }
func (Block) Fields() []ent.Field {
  return []ent.Field{
    field.String("id").Immutable().DefaultFunc(uuidNew),
    field.String("turn_id"),
    field.Int("ord"),
    field.Enum("kind").Values(string(BKUser), string(BKLLM), string(BKCall), string(BKUse), string(BKSys), string(BKOther)),
    field.String("role").Optional().Nillable(),
    field.Time("created_at").Default(time.Now),
  }
}
func (Block) Edges() []ent.Edge {
  return []ent.Edge{
    edge.From("turn", Turn.Type).Ref("blocks").Field("turn_id").Unique().Required(),
    edge.To("payload_kv", BlockPayloadKV.Type),
    edge.To("metadata_kv", BlockMetadataKV.Type),
  }
}
func (Block) Indexes() []ent.Index {
  return []ent.Index{
    index.Fields("turn_id", "ord").Unique(),
  }
}
```

```go
// ent/schema/kv.go (abbrev.)
type BlockPayloadKV struct{ ent.Schema }
func (BlockPayloadKV) Fields() []ent.Field {
  return []ent.Field{
    field.String("block_id"), field.String("turn_id"), field.String("phase"),
    field.String("key"), field.String("type"),
    field.String("value_text").Optional().Nillable(),
    field.String("value_json").Optional().Nillable(),
  }
}
func (BlockPayloadKV) Indexes() []ent.Index {
  return []ent.Index{
    index.Fields("block_id", "turn_id", "phase", "key").Unique(),
    index.Fields("turn_id"), index.Fields("turn_id", "phase"),
  }
}
```

## 4) Repository methods (replace `sqlstore.go`)

We add a small repository that wraps Ent operations and preserves today’s behavior.

```go
// pkg/store/entstore.go (pseudocode)
type EntStore struct { c *ent.Client }

func (s *EntStore) EnsureRun(ctx context.Context, runID string, metadata map[string]any) error {
  if runID == "" { return errors.New("empty run id") }
  return withTx(ctx, s.c, func(tx *ent.Tx) error {
    // Upsert Run
    _, err := tx.Run.Create().SetID(runID).SetCreatedAt(time.Now()).SetNillableMetadata(metadata).
      OnConflict(sql.ConflictColumns("id")).UpdateNewValues().Exec(ctx)
    if err != nil { return err }
    // Upsert kvs
    for k, v := range metadata {
      typ, vt, vj := classifyValue(v)
      err := tx.RunMetadataKV.Create().SetRunID(runID).SetKey(k).SetType(typ).SetNillableValueText(&vt).SetNillableValueJSON(&vj).
        OnConflict(sql.ConflictColumns("run_id","key")).UpdateNewValues().Exec(ctx)
      if err != nil { return err }
    }
    return nil
  })
}
```

```go
func (s *EntStore) SaveTurnSnapshot(ctx context.Context, t *turns.Turn, phase string) error {
  // mirror current defaults for IDs
  if t.RunID == "" { t.RunID = uuid.NewString() }
  if t.ID == "" { t.ID = uuid.NewString() }
  return withTx(ctx, s.c, func(tx *ent.Tx) error {
    if err := s.EnsureRun(ctx, t.RunID, t.Metadata); err != nil { log.Warn().Err(err).Msg("EnsureRun failed") }
    // Upsert Turn
    _, err := tx.Turn.Create().SetID(t.ID).SetRunID(t.RunID).SetCreatedAt(time.Now()).
      SetNillableMetadata(t.Metadata).SetNillableData(t.Data).
      OnConflict(sql.ConflictColumns("id")).UpdateNewValues().Exec(ctx)
    if err != nil { return err }
    // Blocks and KVs (ordered)
    for i := range t.Blocks {
      b := &t.Blocks[i]
      if b.ID == "" { b.ID = uuid.NewString() }
      _, err := tx.Block.Create().SetID(b.ID).SetTurnID(t.ID).SetOrd(i).
        SetKind(kindToEnum(b.Kind)).SetNillableRole(&b.Role).SetCreatedAt(time.Now()).
        OnConflict(sql.ConflictColumns("id")).UpdateNewValues().Exec(ctx)
      if err != nil { return err }
      for pk, pv := range b.Payload { typ, vt, vj := classifyValue(pv); err = tx.BlockPayloadKV.Create().
        SetBlockID(b.ID).SetTurnID(t.ID).SetPhase(phase).SetKey(pk).SetType(typ).
        SetNillableValueText(&vt).SetNillableValueJSON(&vj).
        OnConflict(sql.ConflictColumns("block_id","turn_id","phase","key")).UpdateNewValues().Exec(ctx); if err != nil { return err } }
      for mk, mv := range b.Metadata { typ, vt, vj := classifyValue(mv); err = tx.BlockMetadataKV.Create().
        SetBlockID(b.ID).SetTurnID(t.ID).SetPhase(phase).SetKey(mk).SetType(typ).
        SetNillableValueText(&vt).SetNillableValueJSON(&vj).
        OnConflict(sql.ConflictColumns("block_id","turn_id","phase","key")).UpdateNewValues().Exec(ctx); if err != nil { return err } }
    }
    // Tool registry snapshot if present
    if regAny, ok := t.Data[turns.DataKeyToolRegistry]; ok && regAny != nil {
      if reg, ok := regAny.(geptools.ToolRegistry); ok && reg != nil {
        defs := reg.ListTools(); b, _ := json.Marshal(defs)
        if _, err := tx.TurnKV.Create().SetTurnID(t.ID).SetSection("data").SetKey("tool_registry").SetType("object").
          SetValueJSON(string(b)).OnConflict(sql.ConflictColumns("turn_id","section","key")).UpdateNewValues().Exec(ctx); err != nil { return err }
        if _, err := tx.ToolRegistrySnapshot.Create().SetRunID(t.RunID).SetTurnID(t.ID).SetPhase(phase).
          SetCreatedAt(time.Now()).SetToolsJSON(string(b)).Exec(ctx); err != nil { return err }
      }
    }
    // Turn.Data KVs (skip raw registry object key to avoid overwriting curated JSON)
    for k, v := range t.Data { if k == turns.DataKeyToolRegistry { continue }; typ, vt, vj := classifyValue(v)
      if _, err := tx.TurnKV.Create().SetTurnID(t.ID).SetSection("data").SetKey(k).SetType(typ).SetNillableValueText(&vt).SetNillableValueJSON(&vj).
        OnConflict(sql.ConflictColumns("turn_id","section","key")).UpdateNewValues().Exec(ctx); err != nil { return err } }
    // Snapshot row
    snap := buildSnapshotJSON(t) // same struct as today
    if _, err := tx.TurnSnapshot.Create().SetTurnID(t.ID).SetPhase(phase).SetCreatedAt(time.Now()).SetData(snap).Exec(ctx); err != nil { return err }
    return nil
  })
}
```

```go
func (s *EntStore) LogEvent(ctx context.Context, ev events.Event) {
  // same extraction of fields as today, then tx.ChatEvent.Create() ... Exec(ctx)
}
```

## 5) Porting `debug_commands.go` to Ent

Short term, we can mix Ent queries for simple listings with raw SQL for the complex inspectors. Medium term, we can build small helper queries to encapsulate joins.

- `runs` and `turns`: use `.Query().Order(ent.Desc("created_at")).Limit(n)`.
- `last-turn`: read latest turn via block payload KV rowid or simply latest `blocks` row by created_at; Ent: `client.Block.Query().Order(ent.Desc(block.FieldCreatedAt)).First(ctx)` and read `TurnID`.
- `blocks` inspector:
  - Option A: keep the current SELECT for now. It’s efficient and already implements “phase selection” and matching `tool_call` args.
  - Option B: reimplement with Ent queries + small raw subqueries. Ent can query `Block` rows ordered by `ord` and prefetch KVs; the “lookup tool_call args by id” is easiest with one raw query (as today) scoped by `turn_id` and `phase`.
- `toolcalls` and `tools`:
  - Use Ent to fetch `Block` kind/tool-related KVs and list distinct names.
  - For registry snapshots, add an Ent query to `ToolRegistrySnapshot` ordered by created_at.
- `events`: use Ent to list the last N `ChatEvent` rows.
- `mode` and `injected-mode-prompts`: keep raw SELECTs against views.

Minimal change route: introduce an Ent client and repository, wire it where writes happen; progressively update readers.

## 6) Migrations and compatibility

- Keep table names and columns stable. Use Ent annotations to set `Table` names and field names so Ent targets the existing schema.
- Run `client.Schema.Create(ctx)` for tables Ent owns, then execute the embedded `views.sql` (as done today) to create views.
- For existing apps with current tables, Ent will be able to operate if definitions match. Add guards for missing columns only if we anticipate drift.

## 7) Indexing and constraints

- `blocks`: Unique `(turn_id, ord)`; index `turn_id`.
- `block_*_kv`: Unique `(block_id, turn_id, phase, key)`; indexes on `(turn_id)` and `(turn_id, phase)` for inspector speed.
- `turn_kv`: Unique `(turn_id, section, key)`.
- Add FKs with `OnDelete(Cascade)` for child rows; enable SQLite `_fk=1` as already done in other parts of the repo.

## 8) Implementation plan (high level)

1. Add Ent schema under `pinocchio/ent/...` matching current tables (names/fields) and generate code.
2. Implement `pkg/store/entstore.go` that mirrors `sqlstore.go` behavior using Ent transactions.
3. Initialize Ent client alongside the existing store where the agent starts up; migrate tables and install views (reuse `views.sql`).
4. Switch write path: replace `SQLiteStore` with `EntStore` in the simple-chat agent wiring.
5. Update `debug_commands.go`:
   - Keep raw SQL for complex inspectors initially.
   - Replace simple listings (`runs`, `turns`, `events`) with Ent queries.
   - Add helper functions to fetch registry snapshots via Ent.
6. Regression verify: run the agent, produce runs/turns/blocks, ensure `/dbg` commands output matches prior behavior.

## 9) Notes on Geppetto Turns integration

From `08-turns.md`:
- Keep `Turn.Data[turns.DataKeyToolRegistry]` and `Turn.Data[turns.DataKeyToolConfig]` conventions.
- Engines and middleware behavior is unchanged; the Ent store is purely a persistence detail.
- The Ent repository should continue to serialize curated tool definitions JSON for portability between providers.

## 10) Future improvements

- Replace KV tables with compact JSON columns for certain paths if performance permits; keep KVs for queryable keys.
- Materialize additional views to speed up inspectors (e.g., last-phase per key).
- Add typed helpers for common queries (e.g., “pending tool calls”, “assistant text blocks”), to simplify UI code.

## 11) Risks

- Ent schema mismatches can lead to duplicate tables if names/fields differ; mitigate with explicit table/column names.
- SQLite views are not directly modeled in Ent; keep installing them with raw SQL.
- Performance: Ent adds overhead vs direct SQL; mitigate with indexes and selective raw queries for hot paths.

## 12) Quick wiring example (create client and run views)

```go
db, _ := sql.Open("sqlite3", "file:simple-agent.db?_fk=1")
drv := entsql.OpenDB(dialect.SQLite, db)
client := ent.NewClient(ent.Driver(drv))
_ = client.Schema.Create(ctx)
// install views as today using the embedded SQL (viewsSQL)
_, _ = db.Exec(viewsSQL)
```

This preserves current behavior while opening a path to cleaner queries and evolution.


