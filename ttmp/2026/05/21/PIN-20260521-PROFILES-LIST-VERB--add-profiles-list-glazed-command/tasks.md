# Tasks

## TODO

- [x] Add a Glazed `pinocchio profiles list` command group/verb.
- [x] Reuse Pinocchio profile bootstrap so inline `.pinocchio.yml` profiles and `--profile-registries` sources are both visible.
- [x] Add `--verbosity default|detailed|full` and validate accepted values.
- [x] Emit one Glazed row per profile with explicit column names, including `registry`, `profile`, `selected`, and `default`.
- [x] Add raw override and effective setting fields for important inference settings such as `chat.engine` and `inference.reasoning_effort`.
- [x] Remove or supersede the flag-based `--print-profiles`/`--print-profile-resolution`/`--profile-output` early-exit UX.
- [x] Add tests for registry profiles, selected/default markers, verbosity modes, raw overrides, and effective inherited settings.
- [x] Update user-facing docs to prefer `pinocchio profiles list`.
- [x] Validate with targeted tests and manual CLI smoke.
- [x] Validate with `go test ./...`.
- [ ] Update diary/changelog and commit code/docs.
