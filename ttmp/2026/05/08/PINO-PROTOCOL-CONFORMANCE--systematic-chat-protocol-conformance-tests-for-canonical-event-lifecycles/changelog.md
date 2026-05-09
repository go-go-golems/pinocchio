# Changelog

## 2026-05-08

- Created `PINO-PROTOCOL-CONFORMANCE` workspace for systematic chat protocol conformance testing.
- Added primary design guide: `design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md`.
- Added chronological investigation diary: `reference/01-investigation-diary.md`.
- Documented source-backed protocol invariants for run, text, reasoning, tool, correlation, projection, persistence, and frontend sparse-patch behavior.
- Documented phased implementation plan for Go runtime tests, plugin projection tests, frontend reducer tests, persistence tests, trace replay, and later fuzz/property tests.
- Completed source-backed protocol conformance design guide and investigation diary for canonical chat lifecycle testing.
- `docmgr doctor --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE --stale-after 30` passed.
- Uploaded the design/diary/tasks/changelog bundle to reMarkable: `/ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE/PINO_PROTOCOL_CONFORMANCE_chat_protocol_guide.pdf`.
- Reuploaded the same bundle with the default PDF layout after removing layout-option guidance from the local reMarkable upload skill.
- Added textbook-style static analysis guide: `design-doc/02-static-analysis-for-protocol-conformance.md`.
- Added textbook-style finite-state model guide: `design-doc/03-finite-state-model-for-protocol-conformance.md`.
- Updated all three design guides to cover the lowest provider-native normalization layer across OpenAI Responses, OpenAI-compatible Chat Completions, Claude, and Gemini.
- Expanded tasks to make Geppetto provider-normalization matrices Phase 1 before Pinocchio runtime/projection/frontend conformance tests.
- Uploaded a new non-overwriting reMarkable bundle with all three updated guides: `/ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE/PINO_PROTOCOL_CONFORMANCE_provider_normalization_guides.pdf`.
- Added reducer refactor design: `design-doc/04-openai-chat-stream-reducer-refactor.md`.
- Updated tasks to focus immediate implementation on OpenAI Chat Completions reducer refactoring with table-driven tests.
- Marked static-analysis and model-checking implementation out of scope for this ticket.
- Implemented OpenAI Chat Completions stream reducer in Geppetto: `4262075 Add OpenAI chat stream reducer tests`.
- Wired `engine_openai.go` to use the reducer and apply effects: `12d58dc Wire OpenAI chat stream reducer`.
- Recorded validation: targeted OpenAI package tests and Geppetto pre-commit `go test ./...` plus lint passed.

### Related Files

- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md — Primary intern-oriented design and implementation guide.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/design-doc/02-static-analysis-for-protocol-conformance.md — Static analysis guide including provider-adapter checks.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/design-doc/03-finite-state-model-for-protocol-conformance.md — Finite-state/model-based testing guide including provider-normalization model.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/design-doc/04-openai-chat-stream-reducer-refactor.md — Current reducer refactor implementation plan.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/geppetto/pkg/steps/ai/openai/chat_stream_reducer.go — Implemented reducer state, inputs, terminal handling, effects, and correlation helper.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/geppetto/pkg/steps/ai/openai/chat_stream_reducer_test.go — Table-driven reducer protocol tests.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/geppetto/pkg/steps/ai/openai/engine_openai.go — Engine stream loop now delegates protocol transitions to the reducer.
- /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/reference/01-investigation-diary.md — Chronological research diary for the ticket.
