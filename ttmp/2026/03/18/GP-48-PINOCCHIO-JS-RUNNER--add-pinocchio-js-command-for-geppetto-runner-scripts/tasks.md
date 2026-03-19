# Tasks

## TODO

- [x] Add `pinocchio js` Cobra command wiring and root registration.
- [x] Implement shared command bootstrap for script path resolution, `console`/`ENV` helpers, and JS runtime initialization.
- [x] Implement Pinocchio-aware profile registry loading for the JS command, using inherited root config and default registry discovery.
- [x] Implement Pinocchio hidden base `StepSettings` bootstrap for the JS command.
- [x] Add a small native `require("pinocchio")` module with `engines.fromDefaults()` backed by hidden base settings.
- [x] Expose at least one real Go tool registry to scripts and wire it into the runtime.
- [x] Decide and implement middleware-definition support for Pinocchio profile middleware usage in JS scripts.
- [x] Add smoke-test scripts or command-level execution coverage for local runner and profile-backed runner flows.
- [x] Update Pinocchio docs/help text so the command is discoverable and the intended script authoring model is clear.
- [x] Update diary, changelog, and ticket index as each slice lands.
