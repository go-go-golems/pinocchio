# Tasks

## TODO


- [x] StreamCoordinator: accept prebuilt SEM envelopes in addition to Geppetto event JSON, with deterministic seq patching
- [x] Timeline projector: add builtin chat.message handler so user messages can be projected via SEM stream path
- [x] ConversationService: replace direct user timeline store upsert with stream-published chat.message SEM event
- [x] Add regression tests for mixed stream ingestion and chat.message projection flow
- [ ] Diary: record phase progress and decisions for implemented refactor changes
