# Tasks

## TODO

- [x] Fix Pinocchio compile errors in `ToolLoopBackend` after `b.turn` → `b.Turn` refactor (commit `ee8bf085a1cefea9a73eeceadf4afe9aae453668`)

- [ ] (Geppetto — separate ticket/PR) Relax `turnsdatalint` from const-only keys to typed-key enforcement:
  - [ ] Update `geppetto/pkg/analysis/turnsdatalint/analyzer.go` to accept any expression whose type is the expected named key type (instead of requiring `*types.Const`)
  - [ ] Update `geppetto/pkg/analysis/turnsdatalint/analyzer_test.go` with passing cases:
    - [ ] typed conversions (`TurnDataKey("foo")`)
    - [ ] typed variables (`key := turns.DataKeyFoo; t.Data[key]`)
    - [ ] typed parameters (`func set(key TurnDataKey) { t.Data[key] = v }`)
  - [ ] Keep failing cases for raw strings (`t.Data["foo"]`) and untyped string variables
  - [ ] Update docs in `geppetto/pkg/doc/topics/12-turnsdatalint.md`

- [ ] (Pinocchio) Once Geppetto is updated, re-run:
  - [ ] `make geppetto-lint-build && make geppetto-lint`
  - [ ] Confirm existing “typed conversion / typed variable / typed parameter” patterns become valid without Pinocchio-side code changes

