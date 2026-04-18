# Changelog

## 2026-04-18

- Created ticket `PIN-20260418-CANONICAL-PROFILE-RUNTIME-API` to replace split profile selection/runtime APIs with a canonical runtime API.
- Added a detailed design document describing the new contract, explicit API removals, and the implementation/validation plan.
- Added an implementation diary and detailed task list before changing code.

## 2026-04-18

- Removed the split Geppetto selection/runtime contract and made `ResolveCLIProfileRuntime(...)` the sole public profile-runtime resolver.
- Removed Pinocchio’s public selection/unified-config split and adopted a canonical runtime-first bootstrap API plus Pinocchio-owned engine-settings results.
- Migrated the main Pinocchio command runner, web-chat command, and JS bootstrap path to the runtime-first API.
- Revalidated the refactor with focused Geppetto tests, focused Pinocchio tests, full Pinocchio tests, a fresh CLI build, an isolated fallback smoke run using only `profiles.yaml`, and a real runtime smoke run.
