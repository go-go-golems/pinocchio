# Changelog

## 2026-03-18

- Initial workspace created


## 2026-03-18

Created the Pinocchio downstream engine-profile migration ticket, documented the CLI and JS breakage, and wrote a phased implementation plan focused on repository-loaded commands and pinocchio js.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/cmds/js.go — Current JS bootstrap shape analyzed for migration
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime.go — Current shared helper bug identified in analysis

## 2026-03-18

Implemented the first downstream migration slice for the Pinocchio CLI and JS command. The shared helper now returns final merged `InferenceSettings`, direct CLI callers were updated, the legacy `profiles_migrate_legacy` command was deleted, the `pinocchio js` examples/tests were rewritten to the engine-profile model, and the command help/docs now teach `gp.profiles.resolve({})` plus `gp.engines.fromResolvedProfile(...)` instead of the removed mixed runtime path.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime.go — canonical final-settings helper
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime_test.go — focused merge-path tests
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/main_profile_registries_test.go — JS command regression coverage
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/runner-profile-demo.js — real inference example now uses engine profiles directly
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/runner-profile-smoke.js — deterministic smoke script now validates profile-selected engine settings
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/profiles/basic.yaml — fixture rewritten to engine-profile YAML
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md — help page updated to the new split

## 2026-03-18

Implemented the repository-loaded command follow-up slice. `PinocchioCommand.RunIntoWriter(...)` now resolves the selected engine profile for blocking runs as well, so YAML-loaded commands no longer ignore profile-selected engine settings. Added a fake-factory regression test that loads a command from YAML and proves the merged engine-profile settings reach `CreateEngine(...)`.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/cmd.go — blocking command path now resolves engine-profile-selected settings
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/cmd_profile_registry_test.go — regression test for loaded YAML commands

## 2026-03-18

Implemented the profile-registry migration slice. Added a standalone migration script for old `~/.config/pinocchio/profiles.yaml`, added helper tests that convert mixed runtime profiles and older flat profile maps into engine-only `inference_settings`, added a regression test for the default auto-discovered profiles file path, and rewrote the public docs to describe the real engine-profile shape and precedence order.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/engine_profile_migration.go — core migration helper for mixed and legacy profile YAML
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/engine_profile_migration_test.go — focused conversion tests
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/scripts/migrate-engine-profiles-yaml/main.go — standalone user-facing migration script
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/main_profile_registries_test.go — default `~/.config/pinocchio/profiles.yaml` regression test
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/README.md — updated engine-profile docs and migration instructions
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/README.md — updated JS example docs
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md — updated help page

## 2026-03-18

Finished the web-chat follow-up planning slice inside GP-50. Documented the remaining app-owned runtime concerns in web chat, recommended a separate narrow Pinocchio-local app-profile format instead of reusing Geppetto engine profiles, and recorded the future migration steps needed after the CLI/JS cutover.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/ttmp/2026/03/18/GP-50-PINOCCHIO-ENGINE-PROFILE-MIGRATION--migrate-pinocchio-cli-and-js-to-engine-only-profiles/design-doc/02-web-chat-follow-up-plan.md — explicit web-chat migration handoff plan
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/web-chat/profile_policy.go — mixed app/runtime concerns inventoried
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/web-chat/runtime_composer.go — mixed app/runtime concerns inventoried
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/inference/runtime/composer.go — shared runtime contract that still needs a follow-up hard cut

## 2026-03-18

Closed the CLI/JS portion of GP-50 after verifying the default `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` fallback with a built binary. The command:

```bash
XDG_CONFIG_HOME=<tmp> HOME=<tmp> /tmp/pinocchio-gp50 js ./examples/js/runner-profile-smoke.js --profile gpt-5-mini
```

produced:

```text
"profile=gpt-5-mini model=gpt-5-mini prompt=hello from pinocchio js"
```

This completes the user-facing Pinocchio CLI and JS migration to engine-only profiles. Web chat remains explicitly deferred to the follow-up plan.

## 2026-03-18

Implemented the shared Pinocchio web-chat hard cut that both CoinVault and Temporal were blocked on. Pinocchio now owns its app runtime payload in `pkg/inference/runtime/profile_runtime.go`, shared web-chat/runtime contracts no longer reference the deleted Geppetto mixed runtime type, and `cmd/web-chat` now reads prompt/tool/middleware policy from the `pinocchio.webchat_runtime@v1` profile extension while engine profiles continue to provide only final `InferenceSettings`.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/inference/runtime/profile_runtime.go — new Pinocchio-owned runtime payload and extension helper
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/webchat/conversation.go — shared conversation state now stores Pinocchio runtime payload, not Geppetto runtime spec
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/webchat/http/api.go — HTTP resolution/request types now carry local runtime payload plus resolved inference settings
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/web-chat/profile_policy.go — request resolver now merges engine profiles with Pinocchio runtime extensions
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/web-chat/runtime_composer.go — runtime composer now resolves Pinocchio middleware/tool/runtime policy without Geppetto runtime types
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/webchat/http/profile_api.go — profile API now surfaces Pinocchio runtime extension data instead of deleted Geppetto runtime fields

## 2026-03-18

Made `cmd/web-chat` local-first like CoinVault and Temporal. The resolver now builds a Pinocchio-local conversation/runtime plan first and only converts into `webhttp.ResolvedConversationRequest` at the final transport boundary. This removes the remaining place where the shared transport object doubled as the primary local domain model in the request-resolution path.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/web-chat/profile_policy.go — local-first resolved plan and explicit transport conversion
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/web-chat/profile_policy_test.go — focused local-plan regression coverage
