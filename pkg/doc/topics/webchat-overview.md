---
Title: Webchat Documentation Overview
Slug: webchat-overview
Short: Index of all webchat documentation with reading order and audience guide.
Topics:
- webchat
- documentation
Commands:
- web-chat
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

The webchat framework provides composable components for building real-time chat applications with streaming LLM responses and tool execution.

## Quick Start

**New to webchat?** Start here:

1. [Webchat Getting Started](../tutorials/02-webchat-getting-started.md) — Run the backend + UI locally
2. [Webchat User Guide](webchat-user-guide.md) — Practical usage and customization
3. [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage, profiles, middlewares, and HTTP API

**Building the example app?**

- [Web-Chat Example README](../../../cmd/web-chat/README.md) — Building and running the example

## Documentation by Audience

### Backend Developers

| Doc | Description |
|-----|-------------|
| [User Guide](webchat-user-guide.md) | Practical backend usage and customization |
| [Backend Reference](webchat-backend-reference.md) | API for StreamCoordinator and ConnectionPool |
| [Backend Internals](webchat-backend-internals.md) | Implementation details, concurrency, performance |

### Frontend Developers

| Doc | Description |
|-----|-------------|
| [Getting Started](../tutorials/02-webchat-getting-started.md) | Run the UI locally and connect |
| [Frontend Integration](webchat-frontend-integration.md) | WebSocket and HTTP integration patterns |
| [SEM and UI](webchat-sem-and-ui.md) | Event format, routing, timeline entities |

### Operations / Debugging

| Doc | Description |
|-----|-------------|
| [Debugging and Ops](webchat-debugging-and-ops.md) | Log patterns, troubleshooting, decision trees |

## Architecture Summary

```
Browser                          Backend (pinocchio)
   │                                   │
   │ WebSocket /ws?conv_id=...        │
   ├──────────────────────────────────>│
   │                                   │ Router
   │ POST /chat { prompt }            │   │
   ├──────────────────────────────────>│   ├─> Conversation
   │                                   │   │      │
   │                                   │   │      ├─> Engine + Tool Loop
   │                                   │   │      │      │
   │                                   │   │      │      └─> Events
   │                                   │   │      │             │
   │                                   │   │      ├─> StreamCoordinator
   │                                   │   │      │      │
   │ SEM frames (llm.delta, etc.)     │   │      │      └─> StreamCursor (seq + stream_id)
   │<──────────────────────────────────│   │      │
   │                                   │   │      └─> ConnectionPool
   │                                   │   │             │
   │                                   │   │             └─> Broadcast
```

## Key Concepts

- **Conversation**: Per-conversation state, owns engine, stream coordinator, and connection pool
- **StreamCoordinator**: Bridges event source to WebSocket via callbacks; stamps `event.seq` for ordering
- **ConnectionPool**: Manages WebSocket connections, idle timers, broadcast
- **TimelineStore**: Durable projection store keyed by `event.seq`
- **Profile**: Named configuration with default prompt and middlewares

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/pkg/webchat/router.go` | HTTP/WS wiring, profile handling |
| `pinocchio/pkg/webchat/conversation.go` | Per-conversation lifecycle |
| `pinocchio/pkg/webchat/stream_coordinator.go` | Event consumption and broadcast |
| `pinocchio/pkg/webchat/connection_pool.go` | WebSocket connection management |
| `pinocchio/pkg/webchat/sem_translator.go` | Event to SEM translation |
| `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts` | Frontend WebSocket manager |
| `pinocchio/cmd/web-chat/web/src/sem/registry.ts` | Frontend SEM event routing |

## Geppetto Core Concepts

The webchat framework builds on geppetto primitives:

- **Events and Sinks**: See `geppetto/pkg/doc/topics/04-events.md`
- **Session Lifecycle**: See `geppetto/pkg/doc/playbooks/04-migrate-to-session-api.md`
