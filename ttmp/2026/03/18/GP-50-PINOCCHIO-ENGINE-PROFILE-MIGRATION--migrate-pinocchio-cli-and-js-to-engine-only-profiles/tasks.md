# Tasks

## Slice 1: Analyze and lock the migration boundary

- [x] Identify the two immediate migration targets: repository-loaded CLI commands and `pinocchio js`.
- [x] Confirm that `pinocchio code ...` is not a built-in Cobra subtree but a repository-loaded command path.
- [x] Confirm that the current JS example fixtures still use the removed mixed runtime profile format.
- [x] Write the initial migration analysis and implementation plan in the design doc.

## Slice 2: Shared CLI bootstrap hard cut

- [x] Change `pkg/cmds/helpers/ResolveInferenceSettings(...)` so it returns final merged `InferenceSettings`, not just the base clone.
- [x] Add a dedicated helper for "resolve base settings + optional engine profile + final merged settings" and make it the canonical CLI path.
- [x] Audit and update direct users of the old helper shape in `cmd/agents/simple-chat-agent`, example binaries, and any command middleware glue.
- [x] Add focused tests that prove the selected engine profile changes the effective engine settings.
- [ ] Commit the shared CLI bootstrap slice.

## Slice 3: Repository-loaded command path

- [ ] Trace how repository-loaded Pinocchio YAML commands receive Geppetto sections and ensure profile selection flows into final engine settings.
- [ ] Reproduce or simulate the `pinocchio code unix hello` path with a minimal fixture.
- [ ] Update the command-loading path or helper wiring so repository-loaded commands pick up engine profile settings from config and default profile registries.
- [ ] Add a regression test or smoke example for the repository-loaded command path.
- [ ] Commit the repository-loaded command slice.

## Slice 4: `pinocchio js` migration

- [x] Remove remaining reliance on `gp.runner.resolveRuntime({ profile: ... })` style behavior from the Pinocchio JS command.
- [ ] Decide whether `pinocchio.engines.fromDefaults(...)` stays "base config only" or whether Pinocchio adds a separate engine-profile-aware helper.
- [x] Update `cmd/pinocchio/cmds/js.go` so the default profile/config path uses engine profiles cleanly.
- [x] Update the Pinocchio JS module as needed to expose the right engine inspection/build helpers.
- [ ] Commit the `pinocchio js` migration slice.

## Slice 5: Profile registry format migration

- [ ] Define the Pinocchio-facing engine-profile YAML shape to replace the old mixed runtime profile files.
- [x] Reformat `examples/js/profiles/basic.yaml` and any other checked-in fixtures.
- [ ] Add a migration script for old `~/.config/pinocchio/profiles.yaml` files if the format change is not trivial.
- [ ] Document how the command resolves `--config-file`, `PINOCCHIO_PROFILE_REGISTRIES`, and default `~/.config/pinocchio/profiles.yaml`.
- [ ] Commit the profile-registry migration slice.

## Slice 6: Real inference example and docs

- [x] Restore a real `pinocchio js` example that uses a real engine and a registry-selected engine profile.
- [x] Add inspection output so users can see which engine profile and final engine settings were selected.
- [x] Update `README.md`, `examples/js/README.md`, and command help docs to teach the new flow.
- [x] Validate the documented example command lines end to end.
- [ ] Commit the docs and example slice.

## Slice 7: Web chat follow-up planning

- [ ] Inventory the app-owned multi-profile/runtime concerns that remain in web chat.
- [ ] Decide whether web chat should keep its own app profile YAML or move to a narrower local format.
- [ ] Add a follow-up plan section or separate ticket link for web-chat migration.

## Ticket bookkeeping

- [x] Keep the diary updated after each implementation slice.
- [x] Update `changelog.md` after each code commit.
- [ ] Run `docmgr doctor` before closeout.
