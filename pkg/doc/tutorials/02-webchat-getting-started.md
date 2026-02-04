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

If you want to bind the chat to a specific conversation ID:

```
http://localhost:5173/?conv_id=<uuid>
```

## 3) Send a message through HTTP

```bash
curl -s http://localhost:5173/chat \
  -H "Content-Type: application/json" \
  -d '{"conv_id":"<uuid>","prompt":"hello"}'
```

If the UI is running, you can just type in the composer and hit Enter.

## 4) WebSocket connection (optional manual)

The UI connects to:

```
ws://localhost:5173/ws?conv_id=<uuid>
```

In dev, Vite proxies `/ws` to the Go backend.

## 5) Profiles and overrides

Profiles control default prompts, tools, and middlewares. You can switch profiles:

```bash
curl -s http://localhost:5173/api/chat/profile \
  -H "Content-Type: application/json" \
  -d '{"slug":"agent"}'
```

Overrides can be passed in the chat payload:

```json
{
  "conv_id": "<uuid>",
  "prompt": "use tools",
  "overrides": {
    "system_prompt": "You are an assistant",
    "tools": ["calculator"]
  }
}
```

## Troubleshooting

- **`prompt_len=0`** in logs: ensure you send `prompt`, not `text`.
- **WS not connecting**: confirm `/ws` proxy in `vite.config.ts` and backend is on `:8080`.
- **Timeline empty**: pass `--timeline-db` or `--timeline-dsn` on backend start.

## Next: User Guide

Read the user guide for API details and customization:

- `pinocchio/pkg/doc/topics/webchat-user-guide.md`
