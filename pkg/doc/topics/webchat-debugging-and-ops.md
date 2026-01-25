---
Title: Webchat Debugging and Operations
Slug: webchat-debugging-and-ops
Short: Operational procedures for diagnosing and troubleshooting webchat issues.
Topics:
- webchat
- debugging
- operations
- websocket
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

This guide provides operational procedures for diagnosing webchat issues. It covers WebSocket debugging, log pattern interpretation, sequence diagrams for common flows, and troubleshooting decision trees.

## WebSocket Debugging

### Enabling Debug Logs

The `wsManager` frontend component emits detailed logs when enabled:

| Trigger | How to set | Notes |
|---------|-----------|-------|
| Query flag | Append `?ws_debug=1` to URL | Per-session toggle |
| Global flag | `window.__WS_DEBUG__ = true` in console | Immediate effect |
| LocalStorage | `localStorage.setItem('__WS_DEBUG__','true')` then reload | Persists |

On startup you'll see `[ws.mgr] debug-config { ... enabled: true }` confirming the toggle.

### Log Patterns

**Connection Lifecycle:**

- `[ws.mgr] connect:begin` — manager received connect request
- `[ws.mgr] socket:open:start/open` — WebSocket handshake
- `[ws.mgr] message:forward { type, id }` — backend delivered event

**SEM Routing:**

- `[ws.mgr] message:forward { type, id }` — event received
- Parsed and routed to timeline state handlers
- Unhandled events logged for debugging

### Common Issues

**"Double connection" warnings**

- StrictMode double-mount (expected in dev)
- Check for multiple components opening connections
- Use `enabled` flag to guard mounting

**"No SEM events after open"**

1. Confirm backend is emitting frames (check backend logs)
2. Verify WebSocket messages arrive in browser DevTools
3. Check SEM registry has handlers for event types
4. Look for console errors during event processing

**"Logs not showing despite flag"**

- Use `__WS_DEBUG__` (two underscores)
- Verify via `localStorage.getItem('__WS_DEBUG__')`

## Sequence Diagrams

### Connection Flow

```
┌──────┐    ┌────────┐    ┌────────────┐
│Client│    │Router  │    │Conversation│
└──┬───┘    └───┬────┘    └─────┬──────┘
   │            │               │
   │ WS /ws?conv_id=abc&profile=default
   ├───────────>│
   │            │ Upgrade to WebSocket
   │<───────────┤
   │   [WS OK]  │
   │            │ GetOrCreate("abc", "default")
   │            ├──────────────>│
   │            │               │ [Build engine if needed]
   │            │               │ [Create StreamCoordinator]
   │            │<──────────────┤
   │            │   *Conversation
   │            │
   │            │ AddConnection(conn)
   │            ├──────────────>│
   │            │               │ [Add to ConnectionPool]
   │            │<──────────────┤
   │            │
   │            │ [Read loop starts]
```

### Message Flow

```
┌──────┐    ┌────────┐    ┌────────────┐    ┌─────────────┐
│Client│    │Router  │    │Conversation│    │StreamCoord. │
└──┬───┘    └───┬────┘    └─────┬──────┘    └──────┬──────┘
   │            │               │                  │
   │ POST /chat { prompt, conv_id }
   ├───────────>│
   │            │ StartRun(prompt)
   │            ├──────────────>│
   │            │               │ [Tool loop starts]
   │            │               │ [Events emitted]
   │            │               │
   │            │               │ [StreamCoord reads]
   │            │               ├─────────────────>│
   │            │               │                  │ [Translate]
   │            │               │                  │ [Broadcast]
   │            │               │<─────────────────┤
   │            │               │
   │ [WebSocket receives SEM frames]
   │<───────────┤
   │            │
```

## Troubleshooting Decision Trees

### Connection Issues

```
Problem: WebSocket won't connect
  ↓
Check browser console for errors
  ↓
[Network error] → Verify backend is running
               → Check URL is correct
               → Verify CORS/proxy settings
  ↓
[Upgrade failed] → Check backend logs
               → Verify conversation ID format
               → Check Redis connectivity (if used)
  ↓
[Opens but closes] → Check backend logs
                  → Verify StreamCoordinator started
```

### Missing Events

```
Problem: No SEM events reaching frontend
  ↓
Enable debug logs (?ws_debug=1)
  ↓
Check for message:forward logs
  ↓
[No message:forward] → Backend not emitting events
                    → Check tool loop errors
                    → Verify event publishing
  ↓
[message:forward exists] → Check for processing errors
                        → Verify handler registration
```

### Performance Issues

```
Problem: Slow message delivery or UI lag
  ↓
Check backend logs for timing
  ↓
[Slow engine build] → Profile engine creation
                   → Check profile loading
  ↓
[Slow broadcast] → Check connection count
               → Profile WriteMessage calls
  ↓
[Slow projection] → Check SQLite performance
                 → Verify table indexes
```

## Operational Procedures

### Verifying System Health

**Backend Health Checks:**

1. **Redis connectivity** (if used):
   ```bash
   redis-cli PING
   redis-cli XINFO GROUPS chat:test-conv-id
   ```

2. **StreamCoordinator status:**
   - Check logs for `stream coordinator: started`
   - Verify no `subscribe failed` errors
   - Monitor goroutine count

3. **ConnectionPool status:**
   - Check logs for connection add/remove
   - Verify idle timers firing
   - Monitor WebSocket count

**Frontend Health Checks:**

1. **WebSocket connection:**
   - DevTools → Network → WS
   - Verify "101 Switching Protocols"
   - Check for unexpected closes

2. **State updates:**
   - Use browser DevTools to inspect timeline state
   - Verify entities added/updated
   - Check for stale data

### Common Maintenance Tasks

**Clearing Stale Conversations:**

- Check eviction loop running (backend logs)
- Verify `idleTimeout` configured
- Restart backend if manual eviction needed

**Investigating Duplicate Events:**

1. Check for multiple StreamCoordinator instances
2. Verify one consumer per conversation (Redis)
3. Check for multiple WebSocket connections

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/pkg/webchat/router.go` | HTTP/WS wiring |
| `pinocchio/pkg/webchat/conversation.go` | Conversation lifecycle |
| `pinocchio/pkg/webchat/stream_coordinator.go` | Event consumption |
| `pinocchio/pkg/webchat/connection_pool.go` | WebSocket management |
| `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts` | Frontend WS manager |

## See Also

- [Backend Reference](webchat-backend-reference.md) — API contracts
- [Backend Internals](webchat-backend-internals.md) — Implementation details
- [Frontend Integration](webchat-frontend-integration.md) — Frontend patterns
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage
