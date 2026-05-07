# Changelog

## 2026-05-07

Created PINO-CODE-REVIEW and wrote the initial intern-facing code review guide for Pinocchio's sessionstream/chatapp/web-chat integration. The guide maps runtime flow, explains key packages and APIs, reviews large files and recent debug/observability work, and lists cleanup sketches with concrete file references.

### Related Files

- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/chat.go — Main chatapp engine/runtime sink/projection implementation reviewed
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/features.go — ChatPlugin extension seam reviewed
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/plugins/reasoning.go — Reasoning provider-ID propagation reviewed
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/plugins/toolcall.go — Tool-call plugin projection reviewed
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/server.go — App server/hub/ws composition reviewed
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_recorder.go — Debug recorder reviewed
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_db.go — SQLite reconcile exporter reviewed
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/main.go — CLI/mux/static runtime config integration reviewed
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/ws/wsManager.ts — Frontend websocket/hydration/event mapping reviewed

## 2026-05-07

Uploaded the PINO-CODE-REVIEW bundle to reMarkable at `/ai/2026/05/07/PINO-CODE-REVIEW/PINO-CODE-REVIEW Sessionstream Chatapp Webchat Code Review.pdf`. The first Pandoc upload failed because the diary prompt used literal `\n` sequences in an inline quoted paragraph; the diary now uses a fenced text block for the verbatim prompt and the upload succeeds.

### Related Files

- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/07/PINO-CODE-REVIEW--code-review-and-cleanup-guide-for-sessionstream-chatapp-webchat-integration/reference/01-diary.md — Upload notes and safe prompt formatting
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/07/PINO-CODE-REVIEW--code-review-and-cleanup-guide-for-sessionstream-chatapp-webchat-integration/tasks.md — Marked reMarkable upload done

## 2026-05-07

Started implementation of the cleanup guide. Split `pkg/chatapp/chat.go` leaf helpers into focused files, then completed guide items 5.4 and 5.3 by splitting the web-chat debug recorder and SQLite reconcile exporter. Commits: `c6e7229` split chatapp projections/helpers, `f2d8fef` split debug record encoders, `6ba757d` split reconcile DB exporter.

### Related Files

- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/demo.go — Demo inference helpers extracted from chat.go
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/messages.go — Chat message/protobuf helper functions extracted from chat.go
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/projections.go — Base UI and timeline projections extracted from chat.go
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_record_pipeline.go — Pipeline debug DTOs and encoders
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_record_transport.go — Transport debug DTOs and encoders
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_record_geppetto.go — Geppetto debug DTO and encoder
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_db.go — Reconcile DB orchestration only after split
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_schema.go — Reconcile DB schema DDL
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_views.go — Reconcile DB SQL views
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_backend.go — Backend pipeline/transport inserts
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_frontend.go — Frontend upload parsing/inserts
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_geppetto.go — Geppetto table inserts
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_provider.go — Export data provider adapters
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_snapshots.go — Timeline/turn snapshot inserts
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/app/debug_reconcile_values.go — Reconcile DB scalar/JSON conversion helpers

## 2026-05-07 Continued

Completed the remaining requested chatapp splits and started the frontend WebSocket manager split. `pkg/chatapp/chat.go` now primarily contains engine setup and run bookkeeping; runtime execution and the Geppetto event sink live in focused files. `wsManager.ts` now delegates snapshot mapping/application and UI-event mutation/application to focused modules while preserving legacy mapper re-exports used by existing tests. Commits: `e4503e2` extract runtime sink, `5a62fb5` extract runtime inference flow, `2fce349` extract snapshot mapping, `d2a80f8` extract UI event mapping.

### Related Files

- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/runtime_sink.go — Geppetto runtime event sink and text segment state helpers
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/pkg/chatapp/runtime_inference.go — Chatapp command handling and runtime inference orchestration
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/ws/timelineSnapshot.ts — Snapshot entity mapping/application helpers
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/ws/timelineEvents.ts — UI-event mutation mapping/application helpers
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/ws/wsManager.ts — WebSocket lifecycle and hydration coordinator after mapper extraction

## 2026-05-07 Typed Frontend Payload Decoding

Completed phase 3 of the frontend payload cleanup. Added a dedicated Buf template and generated TypeScript chatapp protobuf descriptors for the web-chat frontend, then introduced typed UI-event decoding for chat message, reasoning, tool, and agent-mode payloads. The live timeline mapper now switches on typed decoded payloads, preserves reasoning provider correlation fields, and creates frontend `tool_call` / `tool_result` entities from tool UI events. Commits: `0cfbe08` generate chatapp protobuf types, `891bf09` decode web chat UI payloads with typed protos.

### Related Files

- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/buf.chatapp.web.gen.yaml — Frontend chatapp protobuf generation template
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb.ts — Generated chatapp TypeScript protobuf descriptors
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/ws/chatappPayloads.ts — Typed decoder for known chatapp UI events
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/ws/timelineEvents.ts — Timeline mutation mapper using typed decoded payloads
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/src/ws/wsManager.test.ts — Coverage for typed reasoning provider IDs and tool event mapping
