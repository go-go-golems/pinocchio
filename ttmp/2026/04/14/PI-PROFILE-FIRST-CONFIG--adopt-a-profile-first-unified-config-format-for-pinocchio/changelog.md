# Changelog

## 2026-04-14

- Initial workspace created


## 2026-04-14

Created the new profile-first config ticket and wrote a detailed current-state analysis, primary design document, and intern-oriented implementation guide. The proposed direction keeps Glazed config plans, keeps external engine-profile registries as optional imported catalogs, introduces a unified Pinocchio config document with app/profile/profiles blocks, and recommends a staged migration away from top-level runtime section config and the legacy .pinocchio-profile.yml local override file.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/analysis/01-current-profile-config-and-registry-architecture-analysis.md — Current architecture and design pressure analysis for the new format
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/design-doc/01-profile-first-unified-config-format-and-migration-design.md — Primary design document for the target schema and migration plan
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/01-implementation-guide-for-the-profile-first-config-format.md — Detailed implementation guide for a future coding pass
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Chronological diary for the research and design work
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Tracks completed research work and future implementation phases


## 2026-04-14

Validated the new ticket with docmgr doctor, added the missing design topic slug to the ticket vocabulary, and uploaded the full design bundle to reMarkable as a single PDF with a table of contents.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/index.md — Ticket validated and delivered to reMarkable
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Recorded the validation and reMarkable delivery step
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/vocabulary.yaml — Added the design topic so the new ticket validates cleanly


## 2026-04-15

Refined the ticket into a detailed execution backlog and updated the design/implementation docs to reflect the latest product decisions: app.repositories should merge across layers, the new config format is a deliberate breaking change, and migration help should come from docs or an optional migration verb rather than runtime compatibility shims.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/design-doc/01-profile-first-unified-config-format-and-migration-design.md — Updated merge semantics and replaced compatibility rollout with a breaking-change rollout
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/01-implementation-guide-for-the-profile-first-config-format.md — Updated the implementation guide to remove compatibility assumptions and encode repositories merge semantics
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Recorded the implementation-kickoff planning step
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Expanded the high-level future-work list into a phased implementation backlog


## 2026-04-15

Step 4 (commit 322e375): added the first code tranche under pinocchio/pkg/configdoc with typed Document/App/Profile/InlineProfile structs, strict YAML decoding for the new format only, slug normalization/validation, explicit rejection of old top-level config shapes, and the new local override filename policy (.pinocchio.yml with legacy .pinocchio-profile.yml rejected).

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/load.go — Introduces strict YAML decode with KnownFields and NormalizeAndValidate integration
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/load_test.go — Introduces focused tests for valid decode and explicit rejection of old format inputs
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/types.go — Introduces the typed unified config document and local filename policy
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 4 for the first configdoc implementation tranche
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the first phase-1 configdoc tasks complete

