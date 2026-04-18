# Changelog

## 2026-04-18

- Initial workspace created


## 2026-04-18

Created the research workspace, traced the profile-selection failure to registry discovery, and drafted the design doc plus investigation diary.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/cli/bootstrap/profile_registry.go — Validation point that rejects empty registries when profile is set
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/web-chat/main.go — Command-level guard that mirrors the same validation
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/cmds/helpers/parse-helpers.go — Secondary helper path that manually reads PINOCCHIO_PROFILE
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go — Pinocchio shared profile bootstrap wrapper
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-PROFILE-ENV-RESOLUTION--fix-pinocchio-profile-env-and-explicit-selection-resolution/design-doc/01-pinocchio-profile-env-and-explicit-profile-resolution-design.md — Primary design analysis and implementation guide
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-PROFILE-ENV-RESOLUTION--fix-pinocchio-profile-env-and-explicit-selection-resolution/reference/01-investigation-diary.md — Chronological investigation log


## 2026-04-18

Validated the ticket workspace with docmgr doctor, resolved vocabulary warnings, and uploaded the design-doc + diary bundle to reMarkable.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-PROFILE-ENV-RESOLUTION--fix-pinocchio-profile-env-and-explicit-selection-resolution/index.md — Ticket overview updated with design and diary links
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-PROFILE-ENV-RESOLUTION--fix-pinocchio-profile-env-and-explicit-selection-resolution/tasks.md — Updated checklist to reflect completed research and delivery work
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/vocabulary.yaml — Added missing topic vocabulary terms for bootstrap


## 2026-04-18

Added a second assessment document arguing for a Geppetto-first fix, traced historical shared fallback behavior, and mapped how to eliminate duplicate registry validation/loading across Pinocchio commands.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/cli/bootstrap/bootstrap_test.go — Current contradictory test captured as evidence of semantic drift
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/cli/bootstrap/profile_selection.go — Shared profile-selection path identified as the right fix point
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/glazed/pkg/config/plan.go — Generic layered config discovery used to frame where Glazed is and is not responsible
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-PROFILE-ENV-RESOLUTION--fix-pinocchio-profile-env-and-explicit-selection-resolution/design-doc/02-shared-assessment-centralize-profile-registry-discovery-and-loading-in-geppetto-bootstrap.md — Second design assessment with Geppetto-first recommendation


## 2026-04-18

Refreshed the reMarkable bundle after adding the second Geppetto-first assessment doc and verified the remote listing.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-PROFILE-ENV-RESOLUTION--fix-pinocchio-profile-env-and-explicit-selection-resolution/design-doc/02-shared-assessment-centralize-profile-registry-discovery-and-loading-in-geppetto-bootstrap.md — New second assessment document included in the updated bundle
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-PROFILE-ENV-RESOLUTION--fix-pinocchio-profile-env-and-explicit-selection-resolution/reference/01-investigation-diary.md — Diary updated with the second architecture reassessment step


## 2026-04-18

Implemented the shared fix in sequence: restored implicit app-owned XDG  fallback plus shared profile-runtime resolution in Geppetto, removed duplicated registry validation/loading from Pinocchio callers, and aligned the Geppetto migration tutorial with the restored behavior.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/cli/bootstrap/profile_registry_defaults.go — Shared app-name-based XDG profiles.yaml fallback
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/cli/bootstrap/profile_runtime.go — New shared helper returns profile selection plus registry chain together
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/cli/bootstrap/profile_selection.go — Shared profile selection now injects implicit default registry sources
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/doc/tutorials/09-migrating-cli-commands-to-glazed-bootstrap-profile-resolution.md — Tutorial updated to describe implicit app-owned profiles.yaml fallback
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/cmds/js.go — JS runtime bootstrap now reuses shared registry-chain resolution
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/web-chat/main.go — Web-chat now consumes shared profile-runtime resolution instead of local validation/loading
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/cmds/helpers/parse-helpers.go — Helper path now uses shared profile selection instead of manually re-reading env state


## 2026-04-18

Re-ran the original PINOCCHIO_PROFILE smoke path safely with a built binary plus temporary XDG-only profiles.yaml, confirmed the selected profile overlay reaches final inference settings, and clarified repository/config merge semantics in the Pinocchio docs.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/README.md — README now documents the separate repositories loading path and exact merge behavior
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md — Topic doc now explains why repositories are Pinocchio-local app metadata outside shared bootstrap sections
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/main.go — Runtime source of repository harvesting semantics described in the docs
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-PROFILE-ENV-RESOLUTION--fix-pinocchio-profile-env-and-explicit-selection-resolution/reference/01-investigation-diary.md — Diary updated with the safe smoke validation details and the aborted unsafe JS-path attempt


## 2026-04-18

Fixed Gemini custom-http-client auth propagation, hard-cut the outer profile API YAML key from `inference_settings.api_keys` to `inference_settings.api`, migrated the local operator profiles file to the new shape, and verified real Gemini execution succeeds instead of failing with ADC lookup errors.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/steps/ai/gemini/engine_gemini.go — Gemini client options now keep `WithAPIKey(...)` even when a custom HTTP client is used
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/steps/ai/gemini/engine_gemini_test.go — Added coverage for default-client and custom-client Gemini auth option construction
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/steps/ai/settings/settings-inference.go — Outer inference YAML key renamed to `api` and legacy `api_keys` wrapper rejected explicitly
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/steps/ai/settings/settings-inference_test.go — Added schema tests for the new `api` root and legacy-wrapper rejection
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/geppetto/pkg/cli/bootstrap/inference_debug.go — Debug trace paths now point at `api.api_keys.*` and `api.base_urls.*`
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/README.md — Updated operator-facing profile YAML example to the new `inference_settings.api` layout
- /home/manuel/.config/pinocchio/profiles.yaml — Local operator profiles migrated in place to the hard-cut schema

