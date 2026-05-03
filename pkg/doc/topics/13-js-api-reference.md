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

The webchat frontend processes sessionstream WebSocket frames and manages timeline entity state through Redux. This reference documents the frame contracts and the public API surface for building custom widgets and extensions.

## WebSocket Frame Types

The sessionstream WebSocket transport sends these frame types:

| Frame type | Direction | Description |
|---|---|---|
| `hello` | server → client | Connection established, contains `connectionId` |
| `subscribe` | client → server | Subscribe to a session, carries `sessionId` and `sinceOrdinal` |
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
  ordinal: string;
  name: string;
  payload: Record<string, unknown>;
};
```

Key fields:

| Field | Type | Description |
|---|---|---|
| `type` | string | Always `"ui-event"` |
| `sessionId` | string | Session this event belongs to |
| `ordinal` | string | Monotonic event ordinal |
| `name` | string | UI event name (e.g., `"ChatMessageAppended"`) |
| `payload` | object | Event-specific data |

## Snapshot Frame

```typescript
type SnapshotFrame = {
  type: "snapshot";
  sessionId: string;
  ordinal: string;
  entities: Array<{
    kind: string;
    id: string;
    tombstone: boolean;
    payload: Record<string, unknown>;
  }>;
};
```

## Subscribe Frame

```typescript
type SubscribeFrame = {
  type: "subscribe";
  sessionId: string;
  sinceOrdinal?: string;
};
```

## Common UI Event Names

| Event name | Description |
|---|---|
| `ChatMessageAccepted` | User message acknowledged |
| `ChatMessageStarted` | Assistant response begins |
| `ChatMessageAppended` | Token/chunk appended |
| `ChatMessageFinished` | Response complete |
| `ChatMessageStopped` | Response stopped |
| `ChatReasoningStarted` | Thinking block begins |
| `ChatReasoningAppended` | Thinking content appended |
| `ChatReasoningFinished` | Thinking block complete |
| `ChatAgentModePreviewUpdated` | Mode switch preview |
| `ChatAgentModeCommitted` | Mode switch committed |
| `ChatAgentModePreviewCleared` | Mode preview cleared |

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

- [Webchat Frontend Integration](webchat-frontend-integration.md) — endpoint contracts and WebSocket lifecycle
- [Webchat Frontend Architecture](webchat-frontend-architecture.md) — directory structure and state flow
