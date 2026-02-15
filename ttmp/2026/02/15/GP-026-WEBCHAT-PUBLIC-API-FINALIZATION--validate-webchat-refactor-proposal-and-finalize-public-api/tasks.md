# Tasks

## TODO


- [x] StreamCoordinator: accept prebuilt SEM envelopes in addition to Geppetto event JSON, with deterministic seq patching
- [x] Timeline projector: add builtin chat.message handler so user messages can be projected via SEM stream path
- [x] ConversationService: replace direct user timeline store upsert with stream-published chat.message SEM event
- [x] Add regression tests for mixed stream ingestion and chat.message projection flow
- [x] Diary: record phase progress and decisions for implemented refactor changes
- [x] Extract StreamBackend abstraction (in-memory + redis) from Router wiring and add constructor helpers
- [x] Introduce explicit HTTP helpers wired to ChatService + StreamHub + TimelineService
- [x] Implement StreamHub to own per-conversation stream state (subscriber/coordinator/ws pool/projector/maintenance)
- [x] Split ConversationService into ChatService-focused API (queue/idempotency/inference only, no websocket attach)
- [x] Cut cmd/web-chat over to service-based wiring and remove direct Router dependency
- [ ] Finalize docs, migration notes, and API contract tests for public release readiness
- [ ] Delete legacy Router/ConvManager/old service paths once replacement services are verified
- [ ] Cut web-agent-example over to service-based wiring and verify event sink wrapper behavior
- [ ] Extract TimelineService and timeline HTTP helper independent of Router
- [ ] Reorganize package layout into stream/chat/timeline/http/bootstrap subpackages with stable public exports
