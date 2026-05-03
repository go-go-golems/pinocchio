---
Title: Changelog
Ticket: SEM-CLEANUP
DocType: changelog
---

## Changelog

### 2026-05-03: Initial investigation and design doc

- Created ticket SEM-CLEANUP for removing dead SEM frame pipeline.
- Investigated entire codebase for SEM frame usage.
- **Finding:** Zero production consumers of SEM registry (Go and TS).
- **Finding:** Only `ChatWidget.stories.tsx` uses the TS SEM registry.
- **Finding:** `debugTimelineWsManager.ts` uses `timelineMapper.ts` (still active).
- **Finding:** `timelinePropsRegistry.ts` is exported from `webchat/index.ts` (still active).
- **Finding:** Tutorial 04 is entirely about the old SEM pipeline (1041 lines, obsolete).
- **Finding:** Four doc topics have stale SEM references.
- **Finding:** `pkg/sem/pb/` protobuf types are still widely used and must not be touched.
- Wrote design doc with architecture analysis, evidence inventory, and phased implementation plan.
- Uploaded bundle to reMarkable: `/ai/2026/05/03/SEM-CLEANUP/SEM-CLEANUP: Dead Pipeline Removal.pdf`

### 2026-05-03: Added debug-ui migration plan (Phase 8)

- Investigated debug-ui architecture. Found it is entirely broken against the current sessionstream server.
- `/ws?conv_id=` endpoint does not exist. All `/api/debug/*` REST endpoints do not exist.
- `debugTimelineWsManager.ts` connects to a non-existent route and parses SEM envelopes that are never sent.
- `debugApi.ts` defines RTK Query endpoints against non-existent routes.
- **Solution:** Rewrite debug-ui to consume the same production WS endpoint (`/api/chat/ws` with subscribe protocol) and the same REST snapshot endpoint. Zero new Go endpoints needed.
- Added Phase 8 to design doc with detailed step-by-step migration plan, code sketches, and file-level guidance.
- Added debug-ui files to DELETE and UPDATE tables in Key File Reference.
- Updated tasks from 15 to 23 items.
- Re-uploaded to reMarkable.

## 2026-05-03

Step 7: End-to-end Playwright testing of debug UI. Verified snapshot replay (3 entities), live event streaming (795 events), all 3 pages working. Go server + Vite running in tmux.

### Related Files

- /home/manuel/code/wesen/corporate-headquarters/pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugWsManager.ts — Verified working with real sessionstream WS

