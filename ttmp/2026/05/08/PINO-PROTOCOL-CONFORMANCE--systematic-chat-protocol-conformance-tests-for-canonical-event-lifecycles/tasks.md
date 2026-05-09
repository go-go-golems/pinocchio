# Tasks

## TODO

- [ ] Split the extracted Responses provider-event handler into smaller semantic handlers only if review finds the remaining switch too hard to follow.
- [ ] Validate `go test ./pkg/steps/ai/openai_responses -count=1` after each Responses code checkpoint.
- [ ] Implement remaining Phase 1 Geppetto provider-normalization table tests in provider adapter packages using `docs/design/implementation/01-provider-event-testing.md` as the scenario source.
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
- [x] Wrote provider event table-driven testing guide with shared scenarios and provider-specific fixture shapes.
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
- [x] Designed the OpenAI Responses refactor to adopt the Chat Completions consume/complete/state pattern.
- [x] Extracted Responses stream terminal types and explicit `responsesStreamState`.
- [x] Moved Responses provider-call, segment, and tool correlation helpers onto state methods.
- [x] Extracted final Responses metadata update into a named helper.
- [x] Extracted final Responses assistant/tool turn-block appending into a named helper.
- [x] Extracted Responses provider-call finish classification/persistence helpers.
- [x] Extracted Responses SSE consume loop into `consumeResponsesSSE` while preserving event behavior.
- [x] Removed the Responses non-streaming inference path so Responses normalization has one streaming lifecycle.
- [x] Updated the former non-streaming usage test to verify usage through the forced streaming path.
- [x] Extracted Responses HTTP stream opening into `openResponsesStream`.
- [x] Extracted Responses provider-call correlation and terminal completion helpers.
- [x] Extracted small Responses stream helpers for provider suffix backfill and JSON/string chunk conversion.
- [x] Added table-driven helper tests for Responses provider suffix backfill and chunk conversion.
- [x] Moved Responses assistant text/message, response id, tool-call accumulation, and terminal/usage/error state into `responsesStreamState`.
- [x] Moved remaining Responses reasoning scratch state into `responsesStreamState`.
- [x] Extracted Responses provider event handling out of `runStreamingInference` and into `stream_events.go`.
- [x] Validated Responses package tests and full Geppetto pre-commit tests/lint for the committed Responses checkpoints.
