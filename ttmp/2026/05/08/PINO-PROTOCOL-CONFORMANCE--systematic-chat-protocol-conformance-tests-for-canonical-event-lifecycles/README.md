# PINO-PROTOCOL-CONFORMANCE

Systematic provider-to-browser chat protocol conformance tests for canonical event lifecycles.

Start here:

1. [Design guide](./design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md)
2. [OpenAI Chat Completions stream reducer refactor](./design-doc/04-openai-chat-stream-reducer-refactor.md)
3. [OpenAI Responses stream refactor](./design-doc/05-openai-responses-stream-refactor.md)
4. [Static analysis guide](./design-doc/02-static-analysis-for-protocol-conformance.md) — reference only for this ticket.
5. [Finite-state model guide](./design-doc/03-finite-state-model-for-protocol-conformance.md) — reference only for this ticket.
6. [Investigation diary](./reference/01-investigation-diary.md)
7. [Tasks](./tasks.md)
8. [Changelog](./changelog.md)

This ticket is a planning and implementation guide for replacing reactive Geppetto provider-adapter, Pinocchio runtime, and web-chat protocol edge-case fixes with explicit invariants and deterministic table-driven tests. Current implementation focus is a reducer refactor for OpenAI-compatible Chat Completions streaming; static-analysis and model-checking implementation are out of scope.
