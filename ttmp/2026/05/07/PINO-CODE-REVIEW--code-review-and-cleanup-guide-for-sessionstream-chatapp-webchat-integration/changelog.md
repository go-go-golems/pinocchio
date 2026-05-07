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
