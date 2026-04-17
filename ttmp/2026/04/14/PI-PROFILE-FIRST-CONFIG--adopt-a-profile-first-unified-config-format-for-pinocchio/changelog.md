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


## 2026-04-15

Step 5 (commit c0c9604): added presence-aware layered merge semantics to pinocchio/pkg/configdoc, including app.repositories merge+dedupe, profile.active and profile.registries replacement when explicitly present, same-slug inline profile overlay behavior, and focused tests for the new merge rules.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/load.go — Annotates field presence from YAML node structure after strict decode
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/merge.go — Implements the first layered merge semantics for the new config format
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/merge_test.go — Adds focused tests for repository accumulation
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/types.go — Extended the typed document model with internal field-presence tracking for correct merge semantics
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 5 for the merge-semantics tranche
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the core merge-rule tasks complete


## 2026-04-15

Step 6 (commit 40299c6): added the synthetic inline-registry adapter under pinocchio/pkg/configdoc so merged inline profiles can be converted into an EngineProfileRegistry and resolved through Geppetto's existing StoreRegistry path.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles.go — Introduces InlineProfilesToRegistry and NewInlineStoreRegistry for the new inline profile catalog path
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles_test.go — Adds focused tests for default profile selection and stacked inline profile resolution
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 6 for the inline registry bridge tranche
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the inline-profile-to-registry adapter task complete


## 2026-04-15

Step 7 (commit ef664c1): added imported-plus-inline registry composition under pinocchio/pkg/configdoc so inline profiles win on same slug by default while imported registries remain available as fallback catalogs.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles.go — Introduces ComposeRegistry and the composed inline+imported registry wrapper
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/profiles_test.go — Adds focused tests for inline-first same-slug precedence and imported fallback behavior
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 7 for the composition tranche
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks imported-plus-inline composition and precedence tasks complete


## 2026-04-15

Step 8 (commit 07edc1c): added LoadResolvedDocuments(...) under pinocchio/pkg/configdoc so ordered ResolvedConfigFile inputs can be decoded and merged into one effective document, with a file-backed test covering user/repo/cwd/explicit layering.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved.go — Introduces the resolved-files loading seam for future document-first bootstrap integration
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved_test.go — Adds file-backed tests for real multi-layer document ordering and merge behavior
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 8 for the resolved-files loader tranche
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the real layering-merge test task complete


## 2026-04-15

Step 9 (commit c6afd24): switched Pinocchio profile selection and engine-settings bootstrap to the unified config document path, composed inline and imported profile registries at runtime, migrated JS runtime bootstrap to the same path, and validated the CLI live with valid and failing profile configurations.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js.go — Uses the unified profile registry chain for JS runtime bootstrap
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/engine_settings.go — Preserves the hidden-base contract while resolving final engine settings from the unified document path
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go — Loads and merges unified config documents
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 9 including the live go run validation matrix and caught regression
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the Phase 4 bootstrap tranche and JS migration task complete


## 2026-04-15

Step 10 (commit 6cf9b41): migrated cmd/web-chat/main.go to initialize profile state through ResolveUnifiedConfig(...) and ResolveUnifiedProfileRegistryChain(...), finishing the main web-chat runtime migration to the unified composed profile bootstrap path.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main.go — Uses unified config resolution and the composed inline/imported registry chain for web-chat startup
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main_profile_registries_test.go — Adds focused coverage for inline-profile bootstrap without external registries
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 10 for the web-chat migration tranche
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the remaining web-chat runtime migration task complete


## 2026-04-15

Step 11 (commit ad1df70): added structured document-merge provenance to configdoc, exposing path-keyed explain data for app/profile/profile merges through ResolvedDocuments and ResolveUnifiedConfig(...), with focused tests for replacement, append+dedupe, and same-slug inline profile merge provenance.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/explain.go — Introduces the path-keyed document explain model and provenance entry vocabulary
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved.go — Carries and records explain data while loading resolved unified config documents
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/configdoc/resolved_test.go — Adds focused provenance assertions for replacement
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 11 for the provenance tranche
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the provenance/explain backlog item and all of its sub-tasks complete


## 2026-04-15

Step 12 (commit 020e6de): folded repository loading fully into app.repositories in the unified config document, collapsed the old repository-only loader path, updated the top-level pinocchio consumer, and added tests proving repository merge+dedupe behavior across unified config layers.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/main.go — Top-level command discovery now relies on unified app config repositories
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories.go — Repository loading now reuses ResolveUnifiedConfig(nil) rather than a custom section-mapper path
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories_test.go — Adds focused coverage for merged app.repositories across home/XDG/repo/cwd layers
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 12 for the app-settings consolidation tranche
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks Phase 5 complete


## 2026-04-16

Step 13 (commit 1f6843c): added focused tests that lock in the rebuild-from-base invariant for runtime profile switching across imported-registry and inline unified-config paths, and revalidated active command/example code for lingering old-shape config assumptions.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/local_profile_plan_test.go — Adds inline unified-config regression coverage for omitted fields staying at the base value
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/ui/profileswitch/manager_test.go — Adds manager tests proving profile switching rebuilds from the original base rather than retaining prior profile overrides
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 13 including the initial sparse-overlay testing mistake and the active-code sweep result
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the remaining Phase 6 runtime migration hardening and active-path sweep complete


## 2026-04-16

Step 14: rewrote the user-facing Pinocchio docs and examples to teach only the unified app/profile/profiles config model, updated .pinocchio.yml local override guidance, and removed legacy profile-settings-based instructions from the main README and JS/profile help docs.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/README.md — Main README now teaches unified config documents and profile.* precedence
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md — JS runner help now documents unified profile.active/profile.registries inheritance
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/examples/js/README.md — JS examples README now matches the unified config rollout
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md — Primary profile-resolution topic now describes .pinocchio.yml and profile.active provenance examples
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 14 and the post-rewrite grep result
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the Pinocchio docs/examples rollout task complete


## 2026-04-16

Step 15 (commit 70a3f62): fixed full-repo validation fallout by migrating the remaining stale example off the removed Geppetto middleware helper and making loaded-command profile tests hermetic so make test lint no longer picks up ambient legacy config.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/examples/simple-redis-streaming-inference/main.go — Uses GetPinocchioCommandMiddlewares instead of the deleted Geppetto helper
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/cmd_profile_registry_test.go — Sandboxes config discovery for loaded-command tests under full repo validation
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records the make test lint fallout and the hermetic test fix


## 2026-04-16

Step 16 (commit 0c958fb): added a dedicated user-facing migration guide from legacy Pinocchio config to the unified app/profile/profiles model, linked it from the README and main help topics, and marked the migration-guide task complete.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/README.md — Main README now links readers directly to the migration guide
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md — JS help now references the migration guide slug
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md — Conceptual runtime page now points to the explicit migration tutorial
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/doc/tutorials/08-migrating-legacy-pinocchio-config-to-unified-profile-documents.md — New top-level migration tutorial covering legacy-key mapping
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/reference/02-investigation-diary.md — Records Step 16 including help-slug validation and the no-new-top-level-runtime-block decision
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-PROFILE-FIRST-CONFIG--adopt-a-profile-first-unified-config-format-for-pinocchio/tasks.md — Marks the user-facing migration guide task complete

