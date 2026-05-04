---
Title: Webchat Frontend Architecture
Slug: webchat-frontend-architecture
Short: Frontend architecture for the sessionstream-backed webchat application.
Topics:
- webchat
- frontend
- architecture
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Frontend Stack

The webchat frontend is a React SPA under `cmd/web-chat/web/src/`:

- React 18+ with TypeScript
- Redux Toolkit for state management
- Sessionstream WebSocket transport for live updates
- Protobuf-backed backend contracts rendered as JSON websocket frames
- Vite for build

## Directory Structure

```
cmd/web-chat/web/src/
  ws/
    wsManager.ts          WebSocket lifecycle + sessionstream protocol
  store/
    store.ts              Redux store configuration
    appSlice.ts           App-level state (status, errors)
    timelineSlice.ts      Timeline entity state (upsert, delete, clear)
  webchat/
    ChatWidget.tsx        Root chat widget component
    rendererRegistry.ts   Entity kind → React component mapping
    cards.tsx             Card renderers (message, agent mode, etc.)
    timelinePropsRegistry.ts  Props normalization before rendering
  sem/
    pb/                   Historical SEM protobuf-generated TypeScript types (not the live chatapp transport)
```

## Sessionstream Projection Pipeline

The frontend receives data through the sessionstream WebSocket transport:

1. Connect to `/api/chat/ws`.
2. Subscribe with a JSON `ClientFrame` shape such as `{ type: "subscribe", sessionId, sinceSnapshotOrdinal: "0" }`.
3. Receive a JSON `ServerFrame` snapshot with `snapshotOrdinal` and `entities` — the full current state.
4. Receive JSON `ServerFrame` UI events with `eventOrdinal`, `name`, and `payload` — live updates.

On the backend these frames and payloads are protobuf-backed: sessionstream uses registered protobuf message schemas and serializes them to JSON for browser delivery. The frontend currently treats the decoded frame as a canonical JSON object and maps it into local Redux state.

```text
snapshot frame
  -> clear store, map registered timeline entities, upsert all entities

ui-event frame
  -> derive mutation (upsert entity, delete entity, update status)
  -> dispatch to timelineSlice
  -> rendererRegistry resolves component by local entity kind
  -> React re-renders
```

The production frontend does not parse historical SEM envelopes. Chatapp backend events and timeline entities are defined by `proto/pinocchio/chatapp/v1/chat.proto` and by registered `ChatPlugin` schemas.

## State Flow

```text
WebSocket frame
  -> wsManager.ts
    -> timelineSlice.upsertEntity / deleteEntity
      -> Redux store
        -> React components (ChatWidget, cards)
```

## Renderer Registry

Entities are rendered by kind. Register a renderer for each entity kind:

```typescript
import { registerTimelineRenderer } from './rendererRegistry';
registerTimelineRenderer('message', MessageCard);
registerTimelineRenderer('agent_mode', AgentModeCard);
registerTimelineRenderer('tool_call', ToolCallCard);
registerTimelineRenderer('tool_result', ToolResultCard);
```

Props are normalized through `timelinePropsRegistry.ts` before reaching renderers, protecting against schema drift between protobuf JSON payloads and local React prop names.

The default web frontend maps registered backend entity kinds into local renderer kinds. For example, `ChatMessage` snapshot entities become local `message` entities, thinking rows are `message` entities with `role: "thinking"`, and shared tool-call plugin entities can be normalized into `tool_call` / `tool_result` renderers by downstream apps.

## Key Files

- `cmd/web-chat/web/src/ws/wsManager.ts` — WebSocket lifecycle and frame→state mapping
- `cmd/web-chat/web/src/store/timelineSlice.ts` — Redux slice for timeline entities
- `cmd/web-chat/web/src/webchat/ChatWidget.tsx` — Root component
- `cmd/web-chat/web/src/webchat/rendererRegistry.ts` — Kind → component registry
