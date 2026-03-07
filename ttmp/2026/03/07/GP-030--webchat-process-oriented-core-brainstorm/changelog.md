# Changelog

## 2026-03-07

- Initial workspace created
- Added a brainstorm document capturing the current discussion about Values separation, app-owned runners, Conversation vs Process abstractions, and websocket multiplexing tradeoffs
- Added the latest runner-input guidance, including constructor-time vs `StartRequest` vs `context.Context` responsibilities
- Added a detailed design and implementation guide describing how to keep `Conversation` as the transport identity while introducing an app-owned `Runner` abstraction
- Updated the ticket index and task list to reflect the new design work
- Expanded the task list with detailed phase 2 and phase 3 implementation tasks covering runner extraction, service boundaries, HTTP helper adaptation, examples, and regression coverage
- Converted GP-030 from design-only notes into an implementation ticket with a diary, explicit phase ordering, and the standing decisions to keep `Conversation`, keep runner instantiation app-owned, and extract the current LLM startup path first
- Extracted the current LLM startup sequence behind `pkg/webchat.LLMLoopRunner`, added core runner types, and introduced `ConversationService.PrepareRunnerStart(...)` as the first embedding-oriented helper
- Added the runner-first embedding path through `PrepareRunnerStart(...) + NewLLMLoopRunner(...)`, clarified the HTTP helper layer as convenience over the runner path, and fixed `TimelineEmitter` so runner-owned upserts persist to the timeline store before fanout
- Closed the first GP-030 implementation slice after passing focused Go validation and `docmgr doctor`
