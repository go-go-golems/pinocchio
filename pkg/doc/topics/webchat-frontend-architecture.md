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
    pb/                   Protobuf-generated TypeScript types (data types only)
```

## Sessionstream Projection Pipeline

The frontend receives data through the sessionstream WebSocket transport:

1. Connect to `/api/chat/ws`.
2. Subscribe with `{ type: "subscribe", sessionId }`.
3. Receive `{ type: "snapshot", entities: [...] }` — full current state.
4. Receive `{ type: "ui-event", name, payload }` — live updates.

The `wsManager.ts` maps incoming frames to Redux store mutations:

```text
snapshot frame
  -> clear store, upsert all entities

ui-event frame
  -> derive mutation (upsert entity, delete entity, update status)
  -> dispatch to timelineSlice
  -> rendererRegistry resolves component by entity kind
  -> React re-renders
```

No SEM envelope parsing or protobuf decoding is involved in the production frontend. All frame shapes are plain JSON.

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
```

Props are normalized through `timelinePropsRegistry.ts` before reaching renderers, protecting against schema drift.

## Key Files

- `cmd/web-chat/web/src/ws/wsManager.ts` — WebSocket lifecycle and frame→state mapping
- `cmd/web-chat/web/src/store/timelineSlice.ts` — Redux slice for timeline entities
- `cmd/web-chat/web/src/webchat/ChatWidget.tsx` — Root component
- `cmd/web-chat/web/src/webchat/rendererRegistry.ts` — Kind → component registry
