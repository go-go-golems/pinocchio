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
  "overrides": {
    "system_prompt": "Be concise"
  }
}
```

Use `/chat/{runtime}` when runtime should come from the path instead of resolver defaults.

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
