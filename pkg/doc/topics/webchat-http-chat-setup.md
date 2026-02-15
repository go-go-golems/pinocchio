---
Title: Webchat HTTP Chat Setup
Slug: webchat-http-chat-setup
Short: Canonical HTTP wiring for app-owned /chat and /ws with timeline hydration and debug APIs.
Topics:
- webchat
- http
- websocket
- api
- routes
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Goal

This page defines the canonical public HTTP contract for new webchat integrations.

Use this page as the source of truth when wiring server routes, frontend clients, and API docs.

## Canonical Setup Pattern

- Build server core with `webchat.NewServer(...)`.
- Register middleware/tool factories on the returned server.
- Mount app-owned handlers:
  - `webchat.NewChatHTTPHandler(srv.ChatService(), resolver)`
  - `webchat.NewWSHTTPHandler(srv.StreamHub(), resolver, upgrader)`
  - `webchat.NewTimelineHTTPHandler(srv.TimelineService(), logger)`
- Mount utility handlers:
  - `srv.APIHandler()` for core `/api/*` utilities
  - `srv.UIHandler()` for static UI

## Route Table

| Route | Method | Ownership | Notes |
|---|---|---|---|
| `/chat` | POST | App | Submit prompt |
| `/chat/{runtime}` | POST | App | Force runtime key from path |
| `/ws?conv_id=<id>` | GET (upgrade) | App | Attach websocket stream |
| `/api/timeline` | GET | App/Core | Hydration snapshot endpoint |
| `/api/debug/timeline` | GET | Core debug | Debug alias for snapshot inspection |
| `/api/debug/turns` | GET | Core debug | Requires turn store |
| `/api/debug/turn/:conv/:session/:turn` | GET | Core debug | Per-turn detail endpoint |
| `/` | GET | Core UI | Embedded UI index/assets |

## Request Resolver Contract

Handlers for `/chat` and `/ws` both depend on a `ConversationRequestResolver`.

The resolver returns `ConversationRequestPlan` with:

- `ConvID`
- `RuntimeKey`
- `Overrides`
- `Prompt`
- `IdempotencyKey`

Use resolver policy for:

- profile/runtime selection
- cookie or query-based defaults
- override allow/deny logic
- request validation and typed errors (`RequestResolutionError`)

## Request and Response Shapes

### POST /chat

Request body:

```json
{
  "prompt": "Classify these transactions",
  "conv_id": "optional-conversation-id",
  "idempotency_key": "optional-client-key",
  "overrides": {
    "system_prompt": "You are a financial analyst",
    "middlewares": [
      { "name": "agentmode", "config": { "default_mode": "financial_analyst" } }
    ],
    "tools": ["calculator"]
  }
}
```

Response shape is service-defined and may include queue metadata. Current reference apps return start/queued metadata plus conversation/session IDs.

### GET /api/timeline

Query params:

- `conv_id` (required)
- `since_version` (optional)
- `limit` (optional)

Response is a `TimelineSnapshotV1` JSON payload.

## Persistence Flags

Timeline snapshots:

- `--timeline-dsn`
- `--timeline-db`

Turn snapshots:

- `--turns-dsn`
- `--turns-db`

Without explicit SQLite config, timeline may use in-memory storage and turn debug endpoints can be unavailable.

## Root Prefix Behavior

When you mount under `--root /chat`, prepend `/chat` to every route in the table.

Examples:

- `/chat/chat`
- `/chat/ws`
- `/chat/api/timeline`
- `/chat/api/debug/turns`

## Deprecated or Non-Canonical Paths

Do not document or depend on these paths in new integrations:

- `/timeline` (top-level)
- `/turns` (top-level)
- `/hydrate`

## Migration Checklist

- Update frontend hydration fetches to `/api/timeline`.
- Update debug tooling to `/api/debug/turns`.
- Remove any `/hydrate` endpoint assumptions.
- Replace router-era setup examples (`NewRouter + NewFromRouter`) with `NewServer + handler constructors` in onboarding docs.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `missing conv_id` on websocket | Client omitted query param | Ensure `conv_id` is always included in websocket URL |
| `timeline service not enabled` | Timeline service unavailable or route not mounted | Mount `/api/timeline` with `NewTimelineHTTPHandler` and confirm service non-nil |
| `turn store not enabled` | No turn store config | Start with `--turns-db` or `--turns-dsn` |
| Policy errors from `/chat` | Resolver validation failure | Return `RequestResolutionError` with explicit status/client message |

## See Also

- [Webchat Framework Guide](webchat-framework-guide.md)
- [Webchat User Guide](webchat-user-guide.md)
- [Webchat Frontend Integration](webchat-frontend-integration.md)
