---
Title: Webchat Getting Started
Slug: webchat-getting-started
Short: Quick start for running the Pinocchio webchat backend and React UI locally.
Topics:
- webchat
- tutorial
- frontend
- backend
Commands:
- web-chat
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
---

# Webchat Getting Started

This guide shows how to run the webchat backend and the React UI locally, then send a message through the API.

## Prerequisites

- Go installed and working (`go version`).
- Node.js + npm for the frontend.
- Optional: Redis (only if you enable redisstream).

## 1) Start the backend

From the repo root:

```bash
go run ./cmd/web-chat web-chat --addr :8080
```

Optional: enable durable timeline storage:

```bash
go run ./cmd/web-chat web-chat --addr :8080 --timeline-db /tmp/webchat-timeline.db
```

## 2) Start the frontend

```bash
cd pinocchio/cmd/web-chat/web
npm install
npm run dev
```

Open: `http://localhost:5173/`

If you want to resume a specific session ID, pass it in the URL:

```
http://localhost:5173/?sessionId=<session-id>
```

Some older links use `conv_id`; the current API names the backend resource a session and the frontend URL parameter is `sessionId`.

## 3) Send a message through HTTP

Create a session, then submit a prompt to that session:

```bash
session_id=$(curl -s -X POST http://localhost:5173/api/chat/sessions \
  -H "Content-Type: application/json" \
  -d '{}' | jq -r '.sessionId')

curl -s -X POST "http://localhost:5173/api/chat/sessions/${session_id}/messages" \
  -H "Content-Type: application/json" \
  -d '{"prompt":"hello"}'

curl -s "http://localhost:5173/api/chat/sessions/${session_id}" | jq
```

If the UI is running, you can just type in the composer and hit Enter. The backend projects protobuf-backed chatapp events into sessionstream snapshot and UI-event frames.

## 4) WebSocket connection (optional manual)

The UI connects to:

```
ws://localhost:5173/api/chat/ws
```

After the socket opens, the client subscribes with the session ID. In dev, Vite proxies `/api/chat/ws` to the Go backend.

## 5) Profiles and request overrides

Profiles control default prompts, tools, and middlewares. You can switch profiles:

```bash
curl -s http://localhost:5173/api/chat/profile \
  -H "Content-Type: application/json" \
  -d '{"slug":"agent"}'
```

You can also inspect profiles with the shared read-only profile endpoints:

```bash
curl -s http://localhost:5173/api/chat/profiles
curl -s http://localhost:5173/api/chat/profiles/analyst
```

Chat payloads stay small and selection-oriented:

```json
{
  "prompt": "use tools",
  "profile": "analyst"
}
```

For full endpoint semantics and selection behavior, see:

- `pinocchio/pkg/doc/topics/webchat-frontend-integration.md`
- `pinocchio/pkg/doc/topics/chatapp-protobuf-plugins.md`

## Troubleshooting

- **`prompt_len=0`** in logs: ensure you send `prompt`, not `text`.
- **WS not connecting**: confirm `/api/chat/ws` proxy in `vite.config.ts` and backend is on `:8080`.
- **Timeline empty after reload**: pass `--timeline-db` or `--timeline-dsn` on backend start.
- **Reasoning/tool rows missing**: confirm `web-chat` was built with `NewReasoningPlugin()` and `NewToolCallPlugin()` in the chat plugin list.

## Next: User Guide

Read these guides for API details and customization:

- `pinocchio/pkg/doc/topics/webchat-frontend-integration.md`
- `pinocchio/pkg/doc/topics/webchat-frontend-architecture.md`
- `pinocchio/pkg/doc/topics/chatapp-protobuf-plugins.md`
