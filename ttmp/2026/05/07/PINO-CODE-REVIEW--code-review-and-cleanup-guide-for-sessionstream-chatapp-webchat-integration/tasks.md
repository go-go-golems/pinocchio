# Tasks

## TODO

- [x] Create PINO-CODE-REVIEW ticket in `pinocchio/ttmp`.
- [x] Inventory package layout, large files, generated artifacts, and recent ticket diaries.
- [x] Map the current sessionstream -> chatapp -> web-chat -> browser runtime flow.
- [x] Review recent debug/reconcile/provider-observability integration for cleanup risks.
- [x] Identify deprecated, legacy, unclear, overgrown, or overengineered code paths.
- [x] Write intern-facing architecture/code-review/cleanup guide with diagrams, API references, file references, and cleanup sketches.
- [x] Maintain investigation diary.
- [x] Upload guide bundle to reMarkable.
- [ ] Validate doc metadata with docmgr doctor if this workspace has docmgr vocabulary for the new ticket.

## Chatapp split execution

- [x] Add a behavior-preserving `pkg/chatapp/chat.go` split plan to the diary before editing code.
- [x] Extract message helpers, demo inference, and base projections out of `pkg/chatapp/chat.go`.
- [ ] Extract `runtimeEventSink` and text segment state helpers out of `pkg/chatapp/chat.go`.
- [ ] Extract runtime inference / Geppetto session orchestration out of `pkg/chatapp/chat.go`.
- [ ] Re-run focused chatapp and web-chat tests after each split slice.
- [ ] Commit each behavior-preserving slice with diary updates.

## Follow-up cleanup candidates

- [ ] Split `pkg/chatapp/chat.go` into engine, runtime sink, projections, IDs, and demo inference files.
- [x] Split `cmd/web-chat/app/debug_recorder.go` by pipeline, transport, and Geppetto record domains.
- [ ] Extract `cmd/web-chat/app/debug_reconcile_db.go` into schema, inserts, frontend parsing, views, and provider adapters.
- [ ] Split `cmd/web-chat/web/src/ws/wsManager.ts` into transport client, hydration coordinator, event mapper, and debug hooks.
- [ ] Add a generated or table-driven frontend mapping for typed `ReasoningUpdate`, `ToolCallUpdate`, and `ChatMessageUpdate` payloads.
- [ ] Move app-local agent-mode plugin code out of `cmd/web-chat` or explicitly document it as example-only.
- [ ] Resolve cross-repo Sessionstream/Geppetto/Pinocchio dependency alignment so Pinocchio passes `GOWORK=off` validation.
