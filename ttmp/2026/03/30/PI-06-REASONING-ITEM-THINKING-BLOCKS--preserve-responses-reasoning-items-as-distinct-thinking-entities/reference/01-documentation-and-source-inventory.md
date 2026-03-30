---
Title: Documentation and source inventory
Ticket: PI-06-REASONING-ITEM-THINKING-BLOCKS
Status: active
Topics:
    - pinocchio
    - geppetto
    - webchat
    - open-responses
DocType: reference
Intent: research
Summary: Inventory of documentation and source files used to understand the current single-thinking-stream architecture.
LastUpdated: 2026-03-30T16:54:10-04:00
---

# Documentation and source inventory

## Documentation I found

The relevant explicit documentation is mostly on the Pinocchio web-chat side rather than the Geppetto OpenAI Responses side.

- `pinocchio/pkg/doc/topics/webchat-sem-and-ui.md`
  - Useful for understanding which SEM event families exist in web-chat and how backend event translation is expected to feed the UI.
- `pinocchio/pkg/doc/topics/webchat-framework-guide.md`
  - Useful for understanding the broader web-chat architecture and where timeline and SEM concerns sit in the stack.
- `pinocchio/pkg/doc/topics/webchat-profile-registry.md`
  - Not directly about thinking streams, but helpful when tracing how runtime behavior is configured in web-chat sessions.

## Primary source code references

There does not appear to be a dedicated design document for Responses reasoning-item lifecycle. For this ticket, the engine source and tests are the authoritative references.

- `geppetto/pkg/steps/ai/openai_responses/engine.go`
  - Primary source of truth for Responses SSE event handling and the current `response.output_item.added` logic.
- `geppetto/pkg/steps/ai/openai/engine_openai.go`
  - Comparison point for the older OpenAI streaming engine, which also exposes a single flattened thinking stream.
- `geppetto/pkg/events/chat-events.go`
  - Defines `EventThinkingPartial`, `EventReasoningTextDelta`, `EventReasoningTextDone`, and related event contracts.
- `geppetto/pkg/steps/ai/openai_responses/engine_test.go`
  - Existing tests for Responses streaming reasoning behavior.
- `pinocchio/pkg/webchat/sem_translator.go`
  - Shows where thinking events are mapped to the fixed `baseID + ":thinking"` id.
- `pinocchio/pkg/webchat/timeline_projector.go`
  - Shows how streamed SEM events become timeline message entities.
- `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
  - Shows how the frontend consumes `llm.thinking.*` events.

## Important conclusion from the documentation search

The architecture is only partially documented. The web-chat SEM and timeline layers are documented well enough to understand their role. The OpenAI Responses reasoning-item contract is not documented in a standalone guide in this repository, so the implementation source and tests are the main references an intern should read first.
