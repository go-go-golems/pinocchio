---
Title: Webchat Frontend Integration
Slug: webchat-frontend-integration
Short: How frontend clients integrate with app-owned /chat and /ws plus /api/timeline hydration.
Topics:
- webchat
- frontend
- websocket
- streaming
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Integration Model

Frontend integration uses three endpoints:

- `POST /chat` to submit user prompts
- `GET /ws?conv_id=<id>` for streaming SEM events
- `GET /api/timeline?conv_id=<id>&since_version=<n>&limit=<n>` for hydration

## Architecture Overview

```
Browser UI
  -> POST /chat
  -> WS /ws?conv_id=...
  -> GET /api/timeline

Go backend
  -> ChatService handles submit/queue/idempotency
  -> StreamHub attaches websocket and fanout
  -> TimelineService returns snapshot entities
```

## WebSocket Lifecycle

1. Determine `conv_id` from URL or state.
2. Connect websocket with `conv_id`.
3. Buffer events while hydration is running.
4. Load `/api/timeline` snapshot.
5. Replay buffered events.
6. Switch to live stream dispatch.

## Recommended Client Pattern: `wsManager` (Dedicated WebSocket Lifecycle Module)

In practice, the simplest way to implement the lifecycle above is to centralize it into a dedicated `wsManager` module/class (see `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`) instead of spreading WebSocket logic across components.

Key reasons:

- **Deterministic lifecycle**: one place owns connect/disconnect, buffering, and hydration gating.
- **Avoid “refetch resets”**: WebSocket streams are push-based. Modeling them as *pull-based query cache entries* (e.g., using RTK Query `queryFn` + invalidation/refetch) can lead to confusing edge cases where a refetch overwrites previously buffered stream frames.
- **Simpler rehydration**: reconnect flows can always run “HTTP snapshot → buffered replay → live”.

Minimal API sketch (frontend):

```ts
type WsStatus = 'disconnected' | 'connecting' | 'connected' | 'hydrating' | 'ready' | 'error';

type SemEnvelope = { sem: true; event: { type: string; id: string; seq?: number; data?: unknown } };

interface WsManager {
  connect(args: { convId: string; basePrefix: string; hydrate?: boolean }): Promise<void>;
  disconnect(): void;
  subscribe(cb: () => void): () => void;
  getState(): { status: WsStatus; convId: string; events: SemEnvelope[]; lastSeq?: number };
}
```

State integration options:

- Push events into a local store (React hook + `useSyncExternalStore`) for “monitor” views.
- Dispatch into a Redux slice (as the web-chat example does) when you want global timeline/message state.

## SEM Frame Contract

WebSocket payloads are semantic envelopes:

```json
{
  "sem": true,
  "event": {
    "type": "llm.delta",
    "id": "event-id",
    "seq": 1707053365123000000,
    "data": { "cumulative": "Hi" }
  }
}
```

Common event types:

- `llm.start`
- `llm.delta`
- `llm.final`
- `tool.start`
- `tool.delta`
- `tool.result`
- `tool.done`
- `log`

## Chat Request Example

```json
{
  "prompt": "Hello assistant",
  "conv_id": "conv-123",
  "request_overrides": {
    "system_prompt": "Be concise"
  }
}
```

Use `/chat/{runtime}` when runtime should come from the path instead of resolver defaults.

Note: use `request_overrides` in the request body (legacy `overrides` aliases are not part of the canonical contract; see [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)).

## Hydration Request Example

```text
GET /api/timeline?conv_id=conv-123&since_version=0&limit=500
```

## Base Prefix Handling

If backend runs with `--root /chat`, prefix all endpoints:

- `/chat/chat`
- `/chat/ws`
- `/chat/api/timeline`

Compute a base prefix from `window.location.pathname` and reuse it consistently for fetch + websocket URLs.

## Error Handling

- Treat non-2xx `POST /chat` responses as user-visible failures.
- Reconnect websocket on disconnect and rehydrate from `/api/timeline`.
- Keep hydration gate enabled to avoid ordering races.

## Non-Canonical Paths to Avoid

Do not call these in new frontend code:

- `/timeline`
- `/turns`
- `/hydrate`

## Key Files

- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
- `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
- `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts`

## See Also

- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
- [Webchat Frontend Architecture](webchat-frontend-architecture.md)
- [Webchat Debugging and Ops](webchat-debugging-and-ops.md)
