# Changelog

## 2026-02-17

- Initial workspace created
- Added broad inventory document for debug persistence data domains and correlation model
- Added restricted implementation plan focused on conversation index persistence in current timeline/turn architecture
- Uploaded restricted implementation plan to reMarkable at `/ai/2026/02/17/GP-027-DEBUG-PERSISTENCE` and verified remote listing
- Added implementation diary and execution-step task breakdown for sequential delivery and commit tracking
- Added `ConversationRecord` and extended `TimelineStore` contract with conversation index methods; patched stores/stubs for compile-safe incremental rollout
- Implemented `timeline_conversations` persistence in SQLite and in-memory timeline stores, with new conversation index tests
- Wired conversation index write-through in `ConvManager` lifecycle (`GetOrCreate`, connection attach/detach, idle eviction)
- Updated debug conversation endpoints to merge live in-memory and persisted timeline conversation index data
- Added regression tests for persisted-only and merged debug conversation responses; deferred turn-summary enrichment helper to phase-2
