# Tasks

## TODO

- [ ] Add a Glazed `pinocchio profiles list` command group/verb.
- [ ] Reuse Pinocchio profile bootstrap so inline `.pinocchio.yml` profiles and `--profile-registries` sources are both visible.
- [ ] Add `--verbosity default|detailed|full` and validate accepted values.
- [ ] Emit one Glazed row per profile with explicit column names, including `registry`, `profile`, `selected`, and `default`.
- [ ] Remove or supersede the flag-based `--print-profiles`/`--print-profile-resolution`/`--profile-output` early-exit UX.
- [ ] Add tests for inline profiles, registry profiles, selected/default markers, verbosity modes, and structured output.
- [ ] Update user-facing docs to prefer `pinocchio profiles list`.
- [ ] Validate with targeted tests, `go test ./...`, and manual CLI smoke.
- [ ] Update diary/changelog and commit code/docs.
