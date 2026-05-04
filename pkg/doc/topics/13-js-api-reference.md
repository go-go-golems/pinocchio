---
Title: JavaScript API Reference
Slug: js-api-reference
Short: Contract reference for JavaScript sessionstream UI event handlers and timeline state management loaded by web-chat.
Topics:
- webchat
- frontend
- api
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: Reference
---

## Overview

The webchat frontend processes sessionstream WebSocket frames and manages timeline entity state through Redux. This reference documents the browser-facing JSON frame contracts and the public API surface for building custom widgets and extensions.

The backend contract is protobuf-first. `sessionstream` defines protobuf `ClientFrame` / `ServerFrame` transport messages, while `pinocchio/pkg/chatapp` defines protobuf payload messages for chat commands, UI events, and timeline entities. Browser code receives their JSON representation.

## WebSocket Frame Types

The sessionstream WebSocket transport sends these frame types:

| Frame type | Direction | Description |
|---|---|---|
| `hello` | server → client | Connection established, contains `connectionId` |
| `subscribe` | client → server | Subscribe to a session, carries `sessionId` and `sinceSnapshotOrdinal` |
| `snapshot` | server → client | Full current state on subscribe, contains `entities` array |
| `subscribed` | server → client | Confirms subscription |
| `ui-event` | server → client | Live update event |
| `unsubscribe` | client → server | Stop receiving events for a session |
| `ping` | client → server | Liveness check |
| `pong` | server → client | Pong response |
| `error` | server → client | Error notification |

## UI Event Frame

```typescript
type UIEventFrame = {
  type: "ui-event";
  sessionId: string;
  eventOrdinal: string;
  name: string;
  payload: Record<string, unknown>;
};
```

Key fields:

| Field | Type | Description |
|---|---|---|
| `type` | string | Always `"ui-event"` |
| `sessionId` | string | Session this event belongs to |
| `eventOrdinal` | string | Monotonic event ordinal for this live UI event |
| `name` | string | UI event name (e.g., `"ChatMessageAppended"`) |
| `payload` | object | Event-specific data |

## Snapshot Frame

```typescript
type SnapshotFrame = {
  type: "snapshot";
  sessionId: string;
  snapshotOrdinal: string;
  entities: Array<{
    kind: string;
    id: string;
    tombstone: boolean;
    createdOrdinal?: string;
    lastEventOrdinal?: string;
    payload: Record<string, unknown>;
  }>;
};
```

## Subscribe Frame

```typescript
type SubscribeFrame = {
  type: "subscribe";
  sessionId: string;
  sinceSnapshotOrdinal?: string;
};
```

## Common UI Event Names

| Event name | Description |
|---|---|
| `ChatMessageAccepted` | User message acknowledged |
| `ChatMessageStarted` | Assistant response begins |
| `ChatMessageAppended` | Assistant text segment updated |
| `ChatMessageFinished` | Assistant text segment or final response complete |
| `ChatMessageStopped` | Response stopped |
| `ChatReasoningStarted` | Shared reasoning plugin thinking block begins |
| `ChatReasoningAppended` | Shared reasoning plugin thinking content appended |
| `ChatReasoningFinished` | Shared reasoning plugin thinking block complete |
| `ChatToolCallStarted` | Shared tool-call plugin saw a model tool request |
| `ChatToolCallUpdated` | Shared tool-call plugin saw execution start/update |
| `ChatToolCallFinished` | Shared tool-call plugin saw tool call completion |
| `ChatToolResultReady` | Shared tool-call plugin produced a result row |
| `ChatAgentModePreviewUpdated` | Mode switch preview |
| `ChatAgentModeCommitted` | Mode switch committed |
| `ChatAgentModePreviewCleared` | Mode preview cleared |

## Chatapp Payloads

The canonical protobuf source for base chatapp payloads is `proto/pinocchio/chatapp/v1/chat.proto`.

Important JSON fields for `ChatMessage` payloads:

| Field | Description |
|---|---|
| `messageId` | Concrete timeline row ID. |
| `parentMessageId` | Parent assistant run ID for segmented rows. |
| `segment` | One-based segment number inside the parent run. |
| `segmentType` | Logical row kind such as `text` or `thinking`. |
| `final` | True only for the final assistant text row of a run. |

The shared reasoning plugin currently uses `google.protobuf.Struct` payloads because providers expose varied reasoning metadata. The shared tool-call plugin uses typed protobuf payloads: `ToolCallUpdate`, `ToolResultUpdate`, `ToolCallEntity`, and `ToolResultEntity`.

## Timeline State (Redux)

The `timelineSlice` manages entities:

```typescript
// Actions
timelineSlice.actions.upsertEntity(entity: TimelineEntity)
timelineSlice.actions.deleteEntity(id: string)
timelineSlice.actions.clear()

// Entity shape
type TimelineEntity = {
  id: string;
  kind: string;
  createdAt: number;
  updatedAt: number;
  props: Record<string, unknown>;
};
```

## Renderer Registry

Map entity kinds to React components:

```typescript
import { registerTimelineRenderer } from './rendererRegistry';

registerTimelineRenderer('message', MessageCard);
registerTimelineRenderer('agent_mode', AgentModeCard);
registerTimelineRenderer('tool_call', ToolCallCard);
registerTimelineRenderer('tool_result', ToolResultCard);
```

## Props Normalization

Before rendering, entity props are normalized:

```typescript
import { registerTimelinePropsNormalizer } from './webchat';

registerTimelinePropsNormalizer('tool_result', (props) => ({
  ...props,
  customKind: String(props.customKind ?? ''),
  result: String(props.resultRaw ?? props.result ?? ''),
}));
```

## See Also

- [Chatapp Protobuf Schemas and Shared Plugins](chatapp-protobuf-plugins.md) — backend protobuf payloads and reusable plugins
- [Webchat Frontend Integration](webchat-frontend-integration.md) — endpoint contracts and WebSocket lifecycle
- [Webchat Frontend Architecture](webchat-frontend-architecture.md) — directory structure and state flow
