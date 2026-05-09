# Tasks

## TODO

- [ ] Implement remaining Phase 1 Geppetto provider-normalization table tests in provider adapter packages beyond OpenAI Chat Completions reducer coverage.
- [ ] Implement Phase 2 Go runtime protocol matrix in `pkg/chatapp`.
- [ ] Implement Phase 3 tool/reasoning plugin projection matrices in `pkg/chatapp/plugins`.
- [ ] Implement Phase 4 frontend reducer-backed conformance matrix in `cmd/web-chat/web/src/ws`.
- [ ] Implement Phase 5 timeline persistence protocol tests in `pkg/ui`.
- [ ] Add a trace extraction/replay helper and decide whether curated fixtures belong in source testdata.

## Not doing

- [ ] Static-analysis implementation for this ticket.
- [ ] Model-checking implementation for this ticket.
- [ ] Fuzz/property tests until deterministic reducer/table tests are in place and accepted.

## Done

- [x] Created `PINO-PROTOCOL-CONFORMANCE` ticket workspace.
- [x] Gathered source evidence for provider adapters, runtime, projections, plugins, persistence, protobuf, and frontend reducer behavior.
- [x] Wrote intern-oriented protocol conformance analysis/design/implementation guide.
- [x] Wrote static analysis guide.
- [x] Wrote finite-state model guide.
- [x] Updated all three guides to cover the provider-native normalization layer.
- [x] Wrote investigation diary.
- [x] Validated ticket with `docmgr doctor`.
- [x] Uploaded guide bundle to reMarkable at `/ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE`.
- [x] Uploaded new non-overwriting provider-normalization guide bundle to reMarkable.
- [x] Added OpenAI Chat Completions reducer refactor design doc.
- [x] Refactored OpenAI Chat Completions streaming around `openAIChatStreamState`, reducer inputs, terminal inputs, and reducer effects.
- [x] Moved Chat Completions correlation construction into a reducer state helper.
- [x] Added table-driven reducer tests for text delta + EOF, empty EOF, cancel after text, error after reasoning, tool argument accumulation, and no tool requests on cancel/error.
- [x] Wired `engine_openai.go` to call the reducer and apply effects while keeping provider stream I/O in the engine.
- [x] Validated OpenAI Chat Completions package and full Geppetto pre-commit tests/lint.
- [x] Shared OpenAI Chat Completions terminal completion across EOF, cancel, and error while preserving partial text/reasoning and avoiding partial tool requests.
- [x] Extracted named `engine_openai.go` helpers so the stream loop principle is visible.
