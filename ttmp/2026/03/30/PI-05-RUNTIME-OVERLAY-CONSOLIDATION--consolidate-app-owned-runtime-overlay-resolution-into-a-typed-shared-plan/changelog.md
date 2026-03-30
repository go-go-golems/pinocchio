# Changelog

## 2026-03-30

- Initial workspace created
- Added ticket overview describing the repeated runtime-overlay pattern across Pinocchio web-chat apps and the need for a shared typed plan
- Added a concrete task list covering typed runtime plans, typed metadata, shared merge helpers, and migrations for `pinocchio`, `go-go-os-chat`, and `gec-rag`
- Implemented typed resolved stack lineage in `geppetto` and committed it in the main sanitize workspace as `1b34ec8`
- Implemented shared runtime-plan and fingerprint helpers in `pinocchio` and committed them as `65260a8`
- Migrated `pinocchio/cmd/web-chat` to the shared runtime-plan helper and committed the refactor as `9ed199c`
- Replayed the framework commits into the `wesen-os` workspace clones as `5958ed12` in `geppetto` and `6200c04` in `pinocchio`
- Migrated `go-go-os-chat` to the shared runtime-plan helper and committed it as `3858008`
- Added a typed inference-runtime-plan migration seam in `gec-rag` and committed it as `3ced495`
- Updated the diary with the full implementation sequence, validation commands, and the remaining follow-up for direct `gec-rag` helper adoption
