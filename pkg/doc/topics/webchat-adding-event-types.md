---
Title: Adding a New Event Type End-to-End
Slug: webchat-adding-event-types
Short: Step-by-step guide for adding a new event type from geppetto backend through SEM translation to frontend widget.
Topics:
- webchat
- sem
- events
- frontend
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: Tutorial
---

Adding a new event type to the webchat system touches multiple layers. This tutorial walks through the complete pipeline — from defining a geppetto event to rendering a React widget.

## Overview

The full pipeline for a new event type:

```
1. Define geppetto event type       (geppetto/pkg/events/)
2. Define protobuf SEM message      (pinocchio/proto/sem/)
3. Register backend SEM handler     (pinocchio/pkg/webchat/sem_translator.go)
4. Add timeline projector case      (pinocchio/pkg/webchat/timeline_projector.go)
5. Create frontend SEM handler      (pinocchio/cmd/web-chat/web/src/sem/)
6. Create React widget              (widget component + registerWidgetRenderer)
7. Wire imports                     (ensure handler and widget are imported)
```

Each step is independent enough that you can skip steps that don't apply. For example, if you're adding a transient UI event that doesn't need persistence, skip step 4 (projector). If you're reusing an existing widget kind, skip step 6.

---

## Step 1: Define the Geppetto Event

Create a new event type in `geppetto/pkg/events/`. All events embed `EventImpl` and are registered via `init()`.

```go
// geppetto/pkg/events/my_feature_events.go
package events

import "encoding/json"

type EventMyFeatureProgress struct {
    EventImpl
    Phase    string  `json:"phase"`
    Progress float64 `json:"progress"`
    Detail   string  `json:"detail,omitempty"`
}

func NewMyFeatureProgressEvent(meta EventMetadata, phase string, progress float64) *EventMyFeatureProgress {
    return &EventMyFeatureProgress{
        EventImpl: EventImpl{
            Type_:     "my-feature-progress",
            Metadata_: meta,
        },
        Phase:    phase,
        Progress: progress,
    }
}

func init() {
    _ = RegisterEventCodec("my-feature-progress", func(b []byte) (Event, error) {
        var ev EventMyFeatureProgress
        if err := json.Unmarshal(b, &ev); err != nil {
            return nil, err
        }
        ev.SetPayload(b)
        return &ev, nil
    })
}
```

**Key points:**
- The `Type_` string must be unique across all event types
- Embed `EventImpl` to satisfy the `Event` interface
- Register in `init()` so the event can be deserialized from JSON
- Populate `EventMetadata` with correlation IDs (SessionID, InferenceID, TurnID) when publishing

**Publishing the event:**

```go
events.PublishEventToContext(ctx, events.NewMyFeatureProgressEvent(meta, "analyzing", 0.5))
```

---

## Step 2: Define the Protobuf Message

Protobuf messages define the contract between backend and frontend. There are two locations:

- **SEM base messages** (`pinocchio/proto/sem/base/`) — for the SEM frame payload (what goes over WebSocket)
- **Timeline snapshot messages** (`pinocchio/proto/sem/timeline/`) — for persistent timeline entities (what gets stored in the DB)

You may need one or both depending on whether the event is transient or persistent.

**SEM base message** (for WebSocket transport):

```proto
// pinocchio/proto/sem/base/myfeature/myfeature.proto
syntax = "proto3";
package sem.base.myfeature;

message MyFeatureProgress {
  string phase = 1;
  double progress = 2;
  string detail = 3;
}
```

**Timeline snapshot message** (for persistence):

```proto
// pinocchio/proto/sem/timeline/myfeature/myfeature.proto
syntax = "proto3";
package sem.timeline.myfeature;

message MyFeatureSnapshotV1 {
  uint32 schema_version = 1;
  string phase = 2;
  double progress = 3;
  string detail = 4;
  string status = 5;  // "active", "completed", "error"
}
```

If adding a persistent entity, also add the snapshot to the `TimelineEntityV1` oneof in `pinocchio/proto/sem/timeline/transport/transport.proto`:

```proto
oneof snapshot {
    // ... existing entries ...
    myfeature.MyFeatureSnapshotV1 my_feature = N;  // next available number
}
```

After editing protos, regenerate the Go and TypeScript code:

```bash
# From the pinocchio directory
make proto-gen  # or whatever the project's proto generation command is
```

---

## Step 3: Register the Backend SEM Handler

In `pinocchio/pkg/webchat/sem_translator.go`, register a handler in the `RegisterDefaultHandlers()` method that converts your geppetto event into a SEM frame.

```go
// Inside RegisterDefaultHandlers()
semregistry.RegisterByType[*events.EventMyFeatureProgress](func(ev *events.EventMyFeatureProgress) ([][]byte, error) {
    md := ev.Metadata()

    data, err := protoToRaw(&sempb.MyFeatureProgress{
        Phase:    ev.Phase,
        Progress: ev.Progress,
        Detail:   ev.Detail,
    })
    if err != nil {
        return nil, err
    }

    return [][]byte{wrapSem(map[string]any{
        "type": "my-feature.progress",
        "id":   md.ID.String(),
        "data": data,
    })}, nil
})
```

**Key patterns:**

- Use `semregistry.RegisterByType[T]` for type-safe handler registration
- Use `protoToRaw()` to convert protobuf messages to `json.RawMessage`
- Use `wrapSem()` to wrap in the standard `{"sem": true, "event": {...}}` envelope
- The `type` string is what the frontend will use to route the event
- For events that need stable IDs across streaming, use `et.resolveMessageID(md)` instead of `md.ID.String()`

---

## Step 4: Add the Timeline Projector Case

If the event should be persisted as a timeline entity, add a case in `TimelineProjector.ApplySemFrame()` in `pinocchio/pkg/webchat/timeline_projector.go`.

```go
// Inside ApplySemFrame(), in the switch on env.Event.Type
case "my-feature.progress":
    var pb sempb.MyFeatureProgress
    if err := protojson.Unmarshal(env.Event.Data, &pb); err != nil {
        return nil
    }

    status := "active"
    if pb.Progress >= 1.0 {
        status = "completed"
    }

    err := p.upsert(ctx, seq, &timelinepb.TimelineEntityV1{
        Id:   env.Event.ID,
        Kind: "my_feature",
        Snapshot: &timelinepb.TimelineEntityV1_MyFeature{
            MyFeature: &timelinepb.MyFeatureSnapshotV1{
                SchemaVersion: 1,
                Phase:         pb.Phase,
                Progress:      pb.Progress,
                Detail:        pb.Detail,
                Status:        status,
            },
        },
    })
    return err
```

**Key behaviors to be aware of:**

- The projector throttles `llm.delta` writes to 250ms minimum. If your event is high-frequency, consider similar throttling.
- Entity IDs must be stable across updates — use the same ID for the same logical entity.
- The `version` (passed via `seq`) is the SEM frame's monotonic sequence number.
- For aggregation patterns (like planning events), see the `applyPlanning()` method as an example.

**Skip this step** if the event is transient (only needs to appear in the live stream, not on page reload).

---

## Step 5: Create the Frontend SEM Handler

Create a handler file in `pinocchio/cmd/web-chat/web/src/sem/` that registers for your event type.

```typescript
// sem/handlers/myFeature.ts (or add to an existing handler file)
import { registerSem } from '../registry';
import { timelineSlice } from '../../store/timelineSlice';

registerSem('my-feature.progress', (ev, dispatch) => {
  const data = ev.data as any;

  dispatch(
    timelineSlice.actions.upsertEntity({
      id: ev.id,
      kind: 'my_feature',
      createdAt: Date.now(),
      updatedAt: Date.now(),
      props: {
        phase: data?.phase ?? '',
        progress: data?.progress ?? 0,
        detail: data?.detail ?? '',
        status: (data?.progress ?? 0) >= 1.0 ? 'completed' : 'active',
      },
    }),
  );
});
```

**Handler patterns:**

| Pattern | When to use | Example |
|---------|-------------|---------|
| `addEntity` | Event always creates a new entity | One-shot notifications |
| `upsertEntity` | Event updates an existing entity by ID | Progress bars, streaming text |

**Key points:**
- Use `upsertEntity` when the same entity ID will be updated multiple times
- Use `addEntity` for one-shot events that should never be merged
- Keep handlers idempotent — they may run multiple times during hydration replay
- Derive entity IDs from `ev.id` to ensure consistency

If using protobuf decoding (recommended for complex payloads):

```typescript
import { fromJson } from '@bufbuild/protobuf';
import { MyFeatureProgressSchema } from '../pb/sem/base/myfeature/myfeature_pb';

registerSem('my-feature.progress', (ev, dispatch) => {
  const pb = fromJson(MyFeatureProgressSchema, ev.data as any, {
    ignoreUnknownFields: true,
  });

  dispatch(
    timelineSlice.actions.upsertEntity({
      id: ev.id,
      kind: 'my_feature',
      createdAt: Date.now(),
      props: {
        phase: pb.phase,
        progress: pb.progress,
        detail: pb.detail,
      },
    }),
  );
});
```

---

## Step 6: Create the Widget

Create a React component that renders the timeline entity.

**For pinocchio's built-in webchat** (`pinocchio/cmd/web-chat/web/src/webchat/`):

Add a rendering case in the existing card renderer, or create a standalone component:

```typescript
// webchat/MyFeatureCard.tsx
import React from 'react';

interface MyFeatureProps {
  entity: {
    id: string;
    kind: 'my_feature';
    props: {
      phase: string;
      progress: number;
      detail?: string;
      status?: string;
    };
  };
}

export function MyFeatureCard({ entity }: MyFeatureProps) {
  const { phase, progress, detail, status } = entity.props;

  return (
    <div style={{
      padding: '8px 12px',
      borderLeft: `3px solid ${status === 'completed' ? '#4caf50' : '#2196f3'}`,
      background: '#f5f5f5',
      borderRadius: '4px',
      margin: '4px 0',
    }}>
      <div style={{ fontWeight: 600, fontSize: '0.85em' }}>
        {phase}
      </div>
      {progress < 1.0 && (
        <div style={{
          height: 4,
          background: '#e0e0e0',
          borderRadius: 2,
          marginTop: 4,
        }}>
          <div style={{
            height: '100%',
            width: `${Math.min(progress * 100, 100)}%`,
            background: '#2196f3',
            borderRadius: 2,
          }} />
        </div>
      )}
      {detail && (
        <div style={{ fontSize: '0.8em', color: '#666', marginTop: 4 }}>
          {detail}
        </div>
      )}
    </div>
  );
}
```

**For the moments platform layer** (if using `registerWidgetRenderer`):

```typescript
// widgets/MyFeatureWidget.tsx
import { registerWidgetRenderer } from '../registry';
import type { TimelineWidgetProps } from '../types';

// Define entity type
interface MyFeatureEntity {
  id: string;
  kind: 'my_feature';
  timestamp: number;
  props: {
    phase: string;
    progress: number;
    detail?: string;
    status?: string;
  };
}

function MyFeatureWidget({ entity }: TimelineWidgetProps<MyFeatureEntity>) {
  const { phase, progress, detail, status } = entity.props;
  // ... render logic
}

// Register at module level
registerWidgetRenderer(
  'my_feature',
  ({ entity }) => <MyFeatureWidget key={entity.id} entity={entity as any} />,
  { visibility: { normal: true, debug: true } }
);
```

**Visibility options:**
- `{ normal: true, debug: true }` — always visible (messages, tool calls)
- `{ normal: false, debug: true }` — debug-only (logs, internal state)
- `{ normal: true, debug: false }` — production-only (rare)

---

## Step 7: Wire the Imports

Ensure your handler and widget are actually loaded by the application.

**Frontend SEM handler** — import in the SEM index or entry point:

```typescript
// sem/index.ts or wherever handlers are collected
import './handlers/myFeature';
```

**Widget** (moments platform) — import in `registerAll.ts`:

```typescript
// registerAll.ts
import './widgets/MyFeatureWidget';
```

Without these imports, the modules never execute and the handlers/widgets never register.

---

## Verification Checklist

After implementing all steps, verify the pipeline works:

1. **Backend SEM frame**: Enable WS debug logging and confirm the frame appears:
   ```
   [ws.mgr] message:forward type=my-feature.progress id=...
   ```

2. **Frontend handler**: Check for routing confirmation:
   ```
   [ws.hook] event:routed kind=upsert id=...
   ```

3. **Redux state**: Open Redux DevTools and inspect `timeline.byId` for your entity.

4. **Widget rendering**: Confirm the widget appears in the chat timeline.

5. **Hydration** (if using projector): Reload the page and confirm the entity reappears from the `/timeline` endpoint.

6. **Storybook** (if applicable): Write a story with synthetic entity data to test widget rendering in isolation.

**Debug tips:**
- Use `?ws_debug=1` query parameter to enable verbose WebSocket logging
- Check browser console for `[sem]` prefixed messages
- If the handler doesn't fire, verify the import is present (the module must be loaded)
- If the entity appears but the widget is blank, check that the entity `kind` matches the registered widget kind exactly

---

## Quick Reference: Existing Event Types

For reference, these are the currently registered event-to-entity mappings:

| SEM Event Type | Entity Kind | Widget | Notes |
|---------------|-------------|--------|-------|
| `llm.start/delta/final` | `message` | MessageWidget | Streaming text with cumulative content |
| `llm.thinking.start/delta/final` | `message` | MessageWidget | Thinking text (role=thinking) |
| `tool.start` | `tool_call` | ToolCallWidget | Tool invocation with status/progress |
| `tool.result` | `tool_result` | ToolWidget | Tool execution output |
| `tool.done` | `tool_call` | ToolCallWidget | Updates tool_call to completed |
| `log` | `log` | StatusWidget | Backend log messages |
| `agent.mode` | `agent_mode` | GenericCard | Agent mode switch |
| `debugger.pause` | `debugger_pause` | DebugPauseWidget | Step-controller pause |
| `thinking.mode.*` | `thinking_mode` | ThinkingModeWidget | Thinking mode selection |
| `planning.*` | `planning` | PlanningWidget | Planning run aggregation |
| `execution.*` | (updates planning) | PlanningWidget | Execution phase of planning |

---

## See Also

- [SEM and UI](webchat-sem-and-ui.md) — SEM event format and handler registration
- [Backend Internals](webchat-backend-internals.md) — Timeline projector reference
- [Frontend Integration](webchat-frontend-integration.md) — WebSocket and state management
- [Events (geppetto)](../../../../geppetto/pkg/doc/topics/04-events.md) — Geppetto event system and custom event types
- [Structured Sinks (geppetto)](../../../../geppetto/pkg/doc/topics/11-structured-sinks.md) — FilteringSink for structured data extraction
