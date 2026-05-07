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
- [x] Extract `runtimeEventSink` and text segment state helpers out of `pkg/chatapp/chat.go`.
- [x] Extract runtime inference / Geppetto session orchestration out of `pkg/chatapp/chat.go`.
- [x] Re-run focused chatapp and web-chat tests after each split slice.
- [x] Commit each behavior-preserving slice with diary updates.

## Frontend wsManager split execution

- [x] Add a behavior-preserving `cmd/web-chat/web/src/ws/wsManager.ts` split plan to the diary before editing code.
- [x] Extract snapshot entity mapping and snapshot application out of `wsManager.ts`.
- [x] Extract UI event mutation mapping and application out of `wsManager.ts`.
- [ ] Extract WebSocket connection lifecycle / message handler helpers if the manager remains large after mapper splits.
- [x] Re-run frontend unit/type checks after each split slice.
- [x] Commit each behavior-preserving frontend slice with diary updates.
- [x] Assess post-mapper `wsManager.ts` size and defer lifecycle extraction unless connection logic grows again.

## Follow-up cleanup candidates

- [ ] Split `pkg/chatapp/chat.go` into engine, runtime sink, projections, IDs, and demo inference files.
- [x] Split `cmd/web-chat/app/debug_recorder.go` by pipeline, transport, and Geppetto record domains.
- [x] Extract `cmd/web-chat/app/debug_reconcile_db.go` into schema, inserts, frontend parsing, views, and provider adapters.
- [ ] Split `cmd/web-chat/web/src/ws/wsManager.ts` into transport client, hydration coordinator, event mapper, and debug hooks.
- [x] Add a generated or table-driven frontend mapping for typed `ReasoningUpdate`, `ToolCallUpdate`, and `ChatMessageUpdate` payloads.

## Frontend typed payload decoding phase 3

- [x] Add a phase-3 typed payload decoding plan to the diary before editing code.
- [x] Generate chatapp TypeScript protobuf descriptors for the web-chat frontend.
- [x] Add a typed `decodeKnownUIEvent` utility for `ChatMessageUpdate`, `ReasoningUpdate`, `ToolCallUpdate`, `ToolResultUpdate`, and agent-mode updates.
- [x] Refactor `timelineEvents.ts` to map known typed UI events instead of reading all known payloads as generic records.
- [x] Add tests for typed lowerCamel protobuf JSON decoding, optional reasoning provider IDs, and tool updates.
- [x] Re-run frontend typecheck and targeted Vitest tests.
- [x] Commit each behavior-preserving frontend decoding slice with diary updates.
- [x] Keep agent-mode plugin code in `cmd/web-chat` and document it as intentionally app-local/example-specific.
- [ ] Resolve cross-repo Sessionstream/Geppetto/Pinocchio dependency alignment so Pinocchio passes `GOWORK=off` validation.

## State-machine hardening

- [x] Add a state-machine hardening plan to the diary before editing code.
- [x] Key reasoning segment state by parent message plus provider item identity when available.
- [x] Preserve fallback parent-only behavior for providers that do not emit item IDs.
- [x] Route summaries for completed provider items back to their original segment even when another segment is active.
- [x] Parse reasoning provider IDs from event metadata extras as well as info-event data.
- [x] Add focused tests for interleaved provider items, completed-segment summaries, and fallback behavior.
- [x] Re-run focused chatapp/plugin/web-chat tests.
- [x] Commit state-machine hardening with diary updates.
