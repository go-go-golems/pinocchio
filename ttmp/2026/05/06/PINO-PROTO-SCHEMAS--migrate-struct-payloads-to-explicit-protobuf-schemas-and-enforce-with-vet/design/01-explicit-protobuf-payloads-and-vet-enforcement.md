---
Title: Explicit Protobuf Payloads and Vet Enforcement Guide
Ticket: PINO-PROTO-SCHEMAS
Status: active
Topics:
  - protobuf
  - sessionstream
  - webchat
  - linting
  - coinvault
DocType: design
Intent: intern-onboarding implementation guide
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Design and implementation guide for replacing generic Struct payload registrations with explicit protobuf schemas and enforcing the rule with a real Go analyzer."
LastUpdated: 2026-05-06T15:45:00-04:00
WhatFor: "Use when implementing the migration away from google.protobuf.Struct top-level sessionstream payloads in Pinocchio/CoinVault or building the schema-policy vet analyzer."
WhenToUse: "Before editing chatapp plugins, CoinVault widget schemas, sessionstream schema registrations, or lint/vet tooling."
---

# Explicit Protobuf Payloads and Vet Enforcement Guide

## 1. Executive summary

Pinocchio and CoinVault use `sessionstream` as the durable chat/event/timeline substrate. `sessionstream` is protobuf-first: commands, backend events, UI events, and timeline entities all carry `proto.Message` payloads. The browser receives those protobuf payloads as protobuf JSON over WebSocket, and hydration reloads the same payloads from persisted timeline snapshots.

The current system still has leftover top-level `google.protobuf.Struct` payloads. A `Struct` is protobuf, but it is the protobuf equivalent of an arbitrary JSON object. It is useful for intentionally open-ended metadata, but it is a poor durable contract for UI events and timeline entities. The recent `AgentMode` hydration bug showed the practical problem: live UI events and hydrated snapshots carried the same conceptual payload with different JSON shapes, and the frontend rendered `No analysis` after reload.

This ticket migrates all remaining top-level `Struct` payload registrations in Pinocchio and CoinVault to explicit, domain-specific protobuf messages, and replaces the temporary source-scanning `_test.go` policy check with a real Go vet-style analyzer.

The target rule is:

> Every sessionstream command, backend event, UI event, and timeline entity payload must be a concrete, named, feature-owned protobuf message. `google.protobuf.Struct` may appear only inside a typed message field for intentionally open-ended sub-data.

## 2. Why this matters

A sessionstream payload is not just a Go value. It is the contract among five layers:

- runtime producers that publish events;
- projection code that turns backend events into UI events and timeline entities;
- hydration stores that persist and reload timeline state;
- WebSocket transport that emits protobuf JSON;
- frontend renderers that map JSON payloads into React entities.

When the top-level payload is `Struct`, the compiler cannot tell whether a field exists, generated TypeScript cannot model the payload, and schema drift hides until a browser reload or a provider-specific edge case. When the top-level payload is a concrete protobuf message, each layer gets a stable field list, generated Go methods, generated frontend types, and a clear migration path.

## 3. System map

### 3.1 Runtime flow

```text
User/browser
  |
  | POST /api/chat/sessions/{id}/messages
  v
Pinocchio/CoinVault web server
  |
  | chatapp.Engine starts runtime inference
  v
Geppetto runtime events
  |
  | ChatPlugin.HandleRuntimeEvent(...)
  v
sessionstream backend Event{Name, Payload proto.Message}
  |
  | ProjectUI / ProjectTimeline
  v
UIEvent{Name, Payload proto.Message}       TimelineEntity{Kind, Id, Payload proto.Message}
  |                                        |
  | WebSocket protobuf JSON                | Hydration SQLite snapshot
  v                                        v
Browser live timeline                 Browser reload/hydration timeline
```

The live path and hydration path must agree on the same payload schema. If they do not, the UI can work while streaming and fail after reload.

### 3.2 Key API references

#### sessionstream schema registry

File: `sessionstream/pkg/sessionstream/schema.go`

```go
type SchemaRegistry struct { /* maps logical names to proto.Message prototypes */ }

func (r *SchemaRegistry) RegisterCommand(name string, msg proto.Message) error
func (r *SchemaRegistry) RegisterEvent(name string, msg proto.Message) error
func (r *SchemaRegistry) RegisterUIEvent(name string, msg proto.Message) error
func (r *SchemaRegistry) RegisterTimelineEntity(kind string, msg proto.Message) error
func (r *SchemaRegistry) MarshalProtoJSON(msg proto.Message) ([]byte, error)
```

The registry stores protobuf message prototypes by logical name. Hydration and transport use those prototypes to marshal/unmarshal payloads.

#### sessionstream event and projection types

Files:

- `sessionstream/pkg/sessionstream/types.go`
- `sessionstream/pkg/sessionstream/projection.go`

```go
type Event struct {
    Name      string
    Payload   proto.Message
    SessionId SessionId
    Ordinal   uint64
}

type UIEvent struct {
    Name    string
    Payload proto.Message
}

type TimelineEntity struct {
    Kind             string
    Id               string
    CreatedOrdinal   uint64
    LastEventOrdinal uint64
    Payload          proto.Message
    Tombstone        bool
}
```

These structs are intentionally generic over `proto.Message`, but the concrete `Payload` value should be a named domain message, not `*structpb.Struct`.

#### Pinocchio chat plugin interface

Files:

- `pinocchio/pkg/chatapp/chat.go`
- `pinocchio/cmd/web-chat/agentmode_chat_feature.go`
- `pinocchio/pkg/chatapp/plugins/reasoning.go`
- `pinocchio/pkg/chatapp/plugins/toolcall.go`

The app/plugin pattern is:

```go
type ChatPlugin interface {
    RegisterSchemas(reg *sessionstream.SchemaRegistry) error
    HandleRuntimeEvent(ctx context.Context, runtime RuntimeEventContext, event gepevents.Event) (bool, error)
    ProjectUI(ctx context.Context, ev sessionstream.Event, sess *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error)
    ProjectTimeline(ctx context.Context, ev sessionstream.Event, sess *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error)
}
```

The schema policy applies to every `RegisterSchemas` implementation, including app-specific ones under `cmd/web-chat` or CoinVault `internal/webchat`.

## 4. Current inventory

### 4.1 Already typed and healthy

#### Base chat messages

File: `pinocchio/proto/pinocchio/chatapp/v1/chat.proto`

- `StartInferenceCommand`
- `StopInferenceCommand`
- `ChatMessageUpdate`
- `ChatMessageEntity`

Registered in `pinocchio/pkg/chatapp/chat.go`.

#### Tool-call plugin

Files:

- `pinocchio/proto/pinocchio/chatapp/v1/chat.proto`
- `pinocchio/pkg/chatapp/plugins/toolcall.go`

Typed messages:

- `ToolCallUpdate`
- `ToolResultUpdate`
- `ToolCallEntity`
- `ToolResultEntity`

This is the model to copy: typed event payloads plus typed timeline entity payloads.

### 4.2 Leftover Pinocchio Struct payloads

#### Reasoning plugin

File: `pinocchio/pkg/chatapp/plugins/reasoning.go`

Current registrations:

```go
reg.RegisterEvent(ReasoningStartedEventName, &structpb.Struct{})
reg.RegisterEvent(ReasoningDeltaEventName, &structpb.Struct{})
reg.RegisterEvent(ReasoningFinishedEventName, &structpb.Struct{})
reg.RegisterUIEvent(ReasoningStartedUIName, &structpb.Struct{})
reg.RegisterUIEvent(ReasoningAppendedUIName, &structpb.Struct{})
reg.RegisterUIEvent(ReasoningFinishedUIName, &structpb.Struct{})
```

The payload is actually chat-message shaped:

- `messageId`
- `parentMessageId`
- `segment`
- `segmentType`
- `role`
- `chunk`
- `text`
- `content`
- `status`
- `streaming`

Target: replace top-level Struct with `ReasoningUpdate` or reuse/extend `ChatMessageUpdate` if semantics match exactly.

Recommendation: define a dedicated `ReasoningUpdate` first. It avoids overloading `ChatMessageUpdate` and leaves room for reasoning-specific provider metadata later.

#### Agent mode plugin

File: `pinocchio/cmd/web-chat/agentmode_chat_feature.go`

Current registrations:

```go
reg.RegisterEvent(agentModePreviewEventName, &structpb.Struct{})
reg.RegisterEvent(agentModeCommittedEventName, &structpb.Struct{})
reg.RegisterUIEvent(agentModePreviewUIName, &structpb.Struct{})
reg.RegisterUIEvent(agentModeCommittedUIName, &structpb.Struct{})
reg.RegisterUIEvent(agentModePreviewClearUIName, &structpb.Struct{})
reg.RegisterTimelineEntity(agentModeTimelineEntityKind, &structpb.Struct{})
```

Target messages:

```proto
message AgentModePreviewUpdate {
  string message_id = 1;
  string candidate_mode = 2;
  string analysis = 3;
  string parse_state = 4;
  bool preview = 5;
}

message AgentModeCommittedUpdate {
  string message_id = 1;
  string title = 2;
  string from = 3;
  string to = 4;
  string analysis = 5;
  bool preview = 6;
}

message AgentModePreviewCleared {
  string message_id = 1;
}

message AgentModeEntity {
  string message_id = 1;
  string title = 2;
  string from = 3;
  string to = 4;
  string analysis = 5;
  bool preview = 6;
}
```

Important: `AgentModeEntity` should flatten the current `data` object. The UI wants `from`, `to`, and `analysis`; hiding those under an arbitrary `data` map recreated the Struct problem.

### 4.3 CoinVault Struct-like payloads

CoinVault already uses named messages, but those messages still contain top-level widget data as a generic `Struct` field.

File: `2026-03-16--gec-rag/proto/coinvault/widgets/v1/widgets.proto`

```proto
message CoinVaultWidgetUpsert {
  string id = 1;
  string type = 2;
  google.protobuf.Struct payload = 3;
}

message CoinVaultWidgetEntity {
  string id = 1;
  string type = 2;
  google.protobuf.Struct payload = 3;
}
```

This is better than registering `Struct` directly, but it is still too generic for durable UI widgets. The `type` string plus `payload` map is a second dynamic dispatch layer. It prevents generated frontend types for concrete widget shapes.

Target: use a protobuf `oneof` for known CoinVault widgets.

Sketch:

```proto
message CoinVaultWidgetUpsert {
  string id = 1;
  oneof widget {
    InventoryCards inventory_cards = 10;
    InventoryTable inventory_table = 11;
    SqlTable sql_table = 12;
    StatsRow stats_row = 13;
    StockAlert stock_alert = 14;
  }
}

message CoinVaultWidgetEntity {
  string id = 1;
  oneof widget {
    InventoryCards inventory_cards = 10;
    InventoryTable inventory_table = 11;
    SqlTable sql_table = 12;
    StatsRow stats_row = 13;
    StockAlert stock_alert = 14;
  }
}

message StatsRow {
  repeated Stat stats = 1;
}

message Stat {
  string label = 1;
  string value = 2;
  string unit = 3;
  string tone = 4;
}
```

For table-shaped data, prefer typed table messages over arbitrary rows if the columns are known. If columns are genuinely dynamic SQL result columns, use a typed dynamic table representation rather than `Struct`:

```proto
message DynamicTable {
  repeated Column columns = 1;
  repeated Row rows = 2;
}

message Column {
  string key = 1;
  string label = 2;
  string type = 3; // string, number, currency, date, boolean
}

message Row {
  repeated Cell cells = 1;
}

message Cell {
  oneof value {
    string string_value = 1;
    double number_value = 2;
    bool bool_value = 3;
    string date_value = 4;
    string null_value = 5;
  }
}
```

This keeps the contract explicit while still supporting dynamic SQL tables.

## 5. Desired end state

### 5.1 Repository policy

- No new `RegisterEvent(..., &structpb.Struct{})`.
- No new `RegisterUIEvent(..., &structpb.Struct{})`.
- No new `RegisterTimelineEntity(..., &structpb.Struct{})`.
- No app-specific exception: app-specific payloads are durable contracts too.
- Existing `Struct` fields inside a typed message require a comment explaining why the field is intentionally dynamic.
- CoinVault widget schemas should use typed messages or `oneof` rather than `type + Struct payload`.

### 5.2 Generated types

Each registered payload has:

- `.proto` source definition;
- generated Go code;
- generated TypeScript code where the frontend consumes it;
- tests covering protobuf JSON shape and hydration mapping.

### 5.3 Frontend mapping

Frontend code should stop treating hydrated payloads as arbitrary records where generated types are available. A reasonable sequence is:

1. Keep current record-based mapping while backend migration lands.
2. Import generated TS schemas.
3. Decode or type-narrow payloads per event/entity kind.
4. Remove generic `data`/`payload` escape hatches from renderer props where not needed.

## 6. Implementation plan

### Phase 0: Preserve current guardrail

The repository currently has a temporary architecture test:

File: `pinocchio/pkg/chatapp/schema_policy_test.go`

It scans source files for forbidden `&structpb.Struct{}` schema registrations. It has a temporary allowlist for the known debt.

Keep this test until the real analyzer is running in CI. Then either remove the test or make it call the analyzer test harness.

### Phase 1: Add Pinocchio protobuf messages

Edit:

- `pinocchio/proto/pinocchio/chatapp/v1/chat.proto`

Add:

```proto
message ReasoningUpdate { ... }
message AgentModePreviewUpdate { ... }
message AgentModeCommittedUpdate { ... }
message AgentModePreviewCleared { ... }
message AgentModeEntity { ... }
```

Run generation:

```text
cd pinocchio
go generate ./...
```

Expected generated files include:

- `pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1/chat.pb.go`
- frontend generated protobuf files under the web package, depending on the existing generation config.

### Phase 2: Migrate ReasoningPlugin

Edit:

- `pinocchio/pkg/chatapp/plugins/reasoning.go`
- `pinocchio/pkg/chatapp/plugins/reasoning_test.go`

Replace `structpb.NewStruct(...)` with typed constructors.

Before:

```go
pb, err := structpb.NewStruct(map[string]any{
    "messageId": id,
    "content": content,
    "status": "streaming",
})
runtime.Publish(ctx, ReasoningDeltaEventName, pb)
```

After:

```go
pb := &chatappv1.ReasoningUpdate{
    MessageId:       id,
    ParentMessageId: parentID,
    Segment:         int32(segment),
    SegmentType:     "thinking",
    Role:            "thinking",
    Content:         content,
    Status:          "streaming",
    Streaming:       true,
}
runtime.Publish(ctx, ReasoningDeltaEventName, pb)
```

Projection pseudocode:

```text
ProjectUI(event):
  update = event.Payload.(*ReasoningUpdate)
  return UIEvent{Name: matchingUIName, Payload: proto.Clone(update)}

ProjectTimeline(event):
  update = event.Payload.(*ReasoningUpdate)
  entity = current ChatMessageEntity for update.message_id, or empty
  merge update fields into entity
  entity.role = "thinking"
  entity.segment_type = "thinking"
  return TimelineEntity{Kind: "ChatMessage", Id: update.message_id, Payload: entity}
```

Question to resolve during implementation: if `ReasoningUpdate` is exactly the same as `ChatMessageUpdate`, reuse may be acceptable. However, a dedicated message is clearer for policy and future reasoning metadata.

### Phase 3: Migrate AgentMode plugin

Edit:

- `pinocchio/cmd/web-chat/agentmode_chat_feature.go`
- `pinocchio/cmd/web-chat/agentmode_chat_feature_test.go`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.test.ts`
- `pinocchio/cmd/web-chat/web/src/webchat/cards.tsx` if props are flattened.

Registration after migration:

```go
reg.RegisterEvent(agentModePreviewEventName, &chatappv1.AgentModePreviewUpdate{})
reg.RegisterEvent(agentModeCommittedEventName, &chatappv1.AgentModeCommittedUpdate{})
reg.RegisterUIEvent(agentModePreviewUIName, &chatappv1.AgentModePreviewUpdate{})
reg.RegisterUIEvent(agentModeCommittedUIName, &chatappv1.AgentModeCommittedUpdate{})
reg.RegisterUIEvent(agentModePreviewClearUIName, &chatappv1.AgentModePreviewCleared{})
reg.RegisterTimelineEntity(agentModeTimelineEntityKind, &chatappv1.AgentModeEntity{})
```

Runtime event conversion:

```text
EventModeSwitchPreview -> AgentModePreviewUpdate
EventAgentModeSwitch   -> AgentModeCommittedUpdate
```

Timeline projection:

```text
ProjectTimeline(committed event):
  update = payload.(*AgentModeCommittedUpdate)
  entity = AgentModeEntity{
    MessageId: update.MessageId,
    Title: update.Title,
    From: update.From,
    To: update.To,
    Analysis: update.Analysis,
    Preview: false,
  }
  return TimelineEntity{Kind: "AgentMode", Id: "session", Payload: entity}
```

Frontend mapping should no longer need to unwrap `payload.value` for AgentMode after all old data is gone. If historical local snapshots remain, keep `unwrapAnyPayload` as a frontend compatibility helper until the local DB is reset or migration is explicitly out of scope.

### Phase 4: Migrate CoinVault widgets

Edit:

- `2026-03-16--gec-rag/proto/coinvault/widgets/v1/widgets.proto`
- generated CoinVault Go protobuf code under `internal/pb/...`
- `2026-03-16--gec-rag/internal/webchat/coinvault_projection_feature.go`
- `2026-03-16--gec-rag/internal/webchat/coinvault_projection_feature_test.go`
- CoinVault frontend widget mapping code, if generated TS is consumed there.

Current CoinVault registration is already typed at the top level:

```go
reg.RegisterEvent(coinVaultWidgetProjectedEvent, &coinvaultwidgetsv1.CoinVaultWidgetUpsert{})
reg.RegisterUIEvent(coinVaultWidgetUpsertUI, &coinvaultwidgetsv1.CoinVaultWidgetUpsert{})
reg.RegisterTimelineEntity(coinVaultWidgetEntityKind, &coinvaultwidgetsv1.CoinVaultWidgetEntity{})
```

But the message internals are generic:

```proto
string type = 2;
google.protobuf.Struct payload = 3;
```

Migration strategy:

1. Inventory all current `type` values from code and fixture data.
2. Define concrete messages for each stable widget shape.
3. Replace `type + payload` with a `oneof`.
4. Update projection code to construct the right `oneof` branch.
5. Update frontend mapping to switch on the protobuf oneof case instead of a string type.

Pseudocode:

```go
func widgetUpsertFromProjection(block projectionblocks.Block) (*CoinVaultWidgetUpsert, error) {
    switch block.Type {
    case "stats_row":
        stats, err := parseStats(block.Payload)
        if err != nil { return nil, err }
        return &CoinVaultWidgetUpsert{
            Id: block.ID,
            Widget: &CoinVaultWidgetUpsert_StatsRow{StatsRow: stats},
        }, nil
    case "sql_table":
        table, err := parseDynamicTable(block.Payload)
        if err != nil { return nil, err }
        return &CoinVaultWidgetUpsert{
            Id: block.ID,
            Widget: &CoinVaultWidgetUpsert_SqlTable{SqlTable: table},
        }, nil
    default:
        return nil, fmt.Errorf("unknown widget type %q", block.Type)
    }
}
```

### Phase 5: Build real Go vet analyzer

The test-based guardrail is useful but not robust. A real analyzer should inspect the Go AST and type information.

Candidate location:

- Option A: `pinocchio/cmd/tools/pinocchio-lint`
- Option B: extend the existing custom lint approach used via Geppetto tooling.
- Option C: create reusable analyzer package under `pinocchio/pkg/analysis/sessionstreamschema` and wire a small command around it.

Recommended structure:

```text
pinocchio/
  pkg/analysis/sessionstreamschema/
    analyzer.go
    analyzer_test.go
    testdata/src/...
  cmd/tools/pinocchio-lint/
    main.go
```

Analyzer goal:

- Find calls to methods/functions named:
  - `RegisterCommand`
  - `RegisterEvent`
  - `RegisterUIEvent`
  - `RegisterTimelineEntity`
- Determine whether the payload argument type is `*google.golang.org/protobuf/types/known/structpb.Struct`.
- Emit a diagnostic unless the call is in an explicit, temporary allowlist.
- Optionally detect typed messages with fields of type `google.protobuf.Struct` and require a nearby comment or allowlist entry.

Analyzer pseudocode:

```go
var Analyzer = &analysis.Analyzer{
    Name: "sessionstreamschema",
    Doc:  "reject generic Struct top-level sessionstream payload registrations",
    Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
    inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
    inspect.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
        call := n.(*ast.CallExpr)
        name := calledName(call.Fun)
        if !isSchemaRegistrationName(name) {
            return
        }
        if len(call.Args) < 2 {
            return
        }
        payload := call.Args[1]
        typ := pass.TypesInfo.TypeOf(payload)
        if isPointerToStructPBStruct(typ) && !isAllowed(pass, call.Pos()) {
            pass.Reportf(payload.Pos(), "sessionstream payload registrations must use concrete protobuf messages, not *structpb.Struct")
        }
    })
    return nil, nil
}
```

Type helper pseudocode:

```go
func isPointerToStructPBStruct(t types.Type) bool {
    ptr, ok := t.(*types.Pointer)
    if !ok { return false }
    named, ok := ptr.Elem().(*types.Named)
    if !ok { return false }
    obj := named.Obj()
    return obj.Name() == "Struct" && obj.Pkg() != nil && obj.Pkg().Path() == "google.golang.org/protobuf/types/known/structpb"
}
```

Allowlist options:

- Prefer package-level comments for temporary debt:

```go
//sessionstreamschema:allow-struct-payload TODO(PINO-PROTO-SCHEMAS): migrate reasoning payloads
```

- Or a YAML config file:

```yaml
sessionstreamschema:
  allow:
    - path: pkg/chatapp/plugins/reasoning.go
      until: PINO-PROTO-SCHEMAS
      reason: temporary migration debt
```

For intern implementation, start with an in-code allowlist in tests, then move to config only if needed.

### Phase 6: Wire analyzer into validation

Target commands:

```text
go vet -vettool=/tmp/pinocchio-lint ./...
make lint
CI lint job
pre-commit hook
```

If using `go/analysis/unitchecker`, command skeleton:

```go
package main

import (
    "golang.org/x/tools/go/analysis/unitchecker"
    "github.com/go-go-golems/pinocchio/pkg/analysis/sessionstreamschema"
)

func main() {
    unitchecker.Main(sessionstreamschema.Analyzer)
}
```

Then:

```text
go build -o /tmp/pinocchio-lint ./cmd/tools/pinocchio-lint
go vet -vettool=/tmp/pinocchio-lint ./...
```

## 7. Validation checklist

### Pinocchio tests

```text
cd pinocchio
go test ./pkg/chatapp ./pkg/chatapp/plugins ./cmd/web-chat -count=1
cd cmd/web-chat/web
npx vitest run src/ws/wsManager.test.ts
npm run typecheck
```

### CoinVault tests

```text
cd 2026-03-16--gec-rag
go test ./internal/webchat ./internal/projectionsem ./internal/projectionblocks -count=1
```

### Analyzer tests

Use `analysistest` fixtures:

```go
func TestAnalyzer(t *testing.T) {
    testdata := analysistest.TestData()
    analysistest.Run(t, testdata, sessionstreamschema.Analyzer, "badstruct", "goodtyped")
}
```

Bad fixture:

```go
reg.RegisterUIEvent("Bad", &structpb.Struct{}) // want "must use concrete protobuf"
```

Good fixture:

```go
reg.RegisterUIEvent("Good", &chatappv1.AgentModeEntity{})
```

### Browser smoke tests

- Open an AgentMode session with a committed mode switch.
- Reload the browser.
- Confirm the AgentMode card still shows title/from/to/analysis.
- Open a session with reasoning events.
- Confirm thinking appears live and after reload.
- Open CoinVault widget sessions.
- Confirm widgets appear live and after reload.

## 8. Risks and design decisions

### Risk: historical local snapshots use Struct JSON

Old local SQLite timeline DBs may contain `Struct` payloads. Decide explicitly per app:

- If local data can be discarded, reset local smoke DBs.
- If data must survive, write a one-off migration.
- Do not silently support two schemas forever unless product requires it.

### Risk: CoinVault widgets may be genuinely dynamic

Some SQL result widgets are naturally dynamic. The answer is not necessarily `Struct`; the answer is a typed dynamic-table schema. A typed dynamic schema still defines columns, rows, and cell value types explicitly.

### Risk: analyzer false positives

A simple name-based analyzer may catch unrelated `RegisterEvent` functions. Avoid this by checking the receiver or function package when type information is available. For method calls, inspect the receiver type and require it to be `*sessionstream.SchemaRegistry` or compatible.

Pseudocode:

```go
func isSessionstreamRegistryMethod(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
    recvType := pass.TypesInfo.TypeOf(sel.X)
    return typeString(recvType) == "*github.com/go-go-golems/sessionstream/pkg/sessionstream.SchemaRegistry"
}
```

### Risk: too many generated changes

Protobuf changes touch generated Go and TS files. Keep commits small:

1. proto + generated code;
2. backend migration;
3. frontend migration;
4. analyzer;
5. remove allowlists.

## 9. Intern implementation runbook

1. Read this guide end-to-end.
2. Open these files side-by-side:
   - `sessionstream/pkg/sessionstream/schema.go`
   - `sessionstream/pkg/sessionstream/projection.go`
   - `pinocchio/pkg/chatapp/chat.go`
   - `pinocchio/pkg/chatapp/plugins/toolcall.go`
   - `pinocchio/pkg/chatapp/plugins/reasoning.go`
   - `pinocchio/cmd/web-chat/agentmode_chat_feature.go`
   - `2026-03-16--gec-rag/internal/webchat/coinvault_projection_feature.go`
   - `2026-03-16--gec-rag/proto/coinvault/widgets/v1/widgets.proto`
3. Implement Pinocchio protobuf messages first.
4. Migrate AgentMode before Reasoning because the bug is known and the shape is small.
5. Migrate Reasoning next.
6. Build the analyzer while the temporary test still exists.
7. Replace the temporary test allowlist with analyzer tests.
8. Migrate CoinVault widget payloads.
9. Run backend, frontend, and browser smoke tests.
10. Update this ticket's diary and changelog after each phase.

## 10. Done definition

This ticket is done when:

- Pinocchio has no production `RegisterEvent`, `RegisterUIEvent`, or `RegisterTimelineEntity` calls using `&structpb.Struct{}`.
- CoinVault widget schemas no longer use `type + google.protobuf.Struct payload` for known widgets.
- A real Go analyzer rejects generic Struct top-level sessionstream registrations.
- The analyzer runs in Pinocchio validation (`make lint` or equivalent CI/pre-commit path).
- Frontend hydration and live streaming tests cover AgentMode and reasoning with the new payloads.
- Documentation and ticket changelog reflect the final schema policy.
