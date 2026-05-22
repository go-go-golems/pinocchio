# Tasks

## TODO

- [x] Wire Geppetto profile introspection flags into the Pinocchio root CLI:
  - `--print-profiles`
  - `--print-profile-resolution`
  - `--profile-output text|json|yaml`
- [x] Add the same profile introspection section to dynamic Pinocchio command schemas so `run-command ... --print-profiles` works.
- [x] Add a Pinocchio-specific report builder that uses `pkg/cmds/profilebootstrap.ResolveCLIProfileRuntime` so `.pinocchio.yml` inline profiles and registry overlays are represented correctly.
- [x] Add early-exit handling before inference for command verbs and `run-command`.
- [x] Add tests for text/json profile output and inline profile coverage.
- [x] Update user-facing help/docs.
- [x] Run validation and commit code.
- [ ] Update diary/changelog and commit docs.
