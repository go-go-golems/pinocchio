### Backend-centric semantic event mapping for a rich, store-driven UI

This document proposes moving semantic extraction of LLM/tool events to the backend so the web client receives semantically rich, stable events over WebSocket. The goal is a clean Zustand action log with domain-oriented actions rather than low-level lifecycle diffs.

---

### 1) Current state (analysis)

- **Frontend timeline components** (`web/src/timeline/components.js`):
  - Presentational Preact components that render normalized entities of kinds: `llm_text`, `tool_call`, `tool_call_result`, `calc_result`, `agent_mode`, `log_event`.
  - They rely on an `entity` prop with `kind`, `props`, and, for `llm_text`, flags like `streaming`, `metadata`, role, and final text.

- **Frontend store actions** (`web/src/store.js`):
  - Low-level lifecycle: `tlCreated`, `tlUpdated`, `tlCompleted`, `tlDeleted`, with a dedupe rule for generic tool results.
  - High-level actions: `llmTextStart`, `llmTextAppend`, `llmTextFinal`, `toolCallStart`, `toolCallDelta`, `toolCallDone`, `toolCallResult`, plus `sendPrompt` and `startChat`.
  - Ingestion (`handleIncoming`) maps `{ tl: true, event }` to lifecycle and tries to infer high-level actions by inspecting patches (e.g., delta, text growth).

- **Frontend WS mapping** (`web/src/store.js`):
  - Receives messages `{ tl: true, event }`; maps to lifecycle or inferred high-level actions.
  - Dedupes locally created user messages vs server echoes.

- **Backend mapping** (`cmd/web-chat/pkg/backend/forwarder.go`):
  - Receives Geppetto `events.Event` and emits TL lifecycle messages (`TimelineEvent`) over WS wrapped as `{ tl: true, event }`.
  - Mappings:
    - Logs → `created(log_event)` + `completed`
    - LLM start → `created(llm_text, streaming=true, role=assistant)`
    - LLM partial → `updated` with `patch: { text: ev.Completion, metadata, streaming: true }`
    - LLM final → `completed { text }` + `updated { streaming: false }`
    - Tool call → `created(tool_call, { name, input })`
    - Tool execute → `updated { exec: true, input }`
    - Tool result → either custom `calc_result` or generic `tool_call_result` (and clear exec)
    - Agent mode → `created(agent_mode)` + `completed`

Problems:
- Frontend infers semantics from generic TL lifecycle messages, duplicating logic and adding brittleness.
- Some semantics (like deltas) are easier to compute at the source (backend) where raw events are present.

---

### 2) Proposed architecture: backend emits semantic events

Shift semantic extraction to the backend. The backend continues to own the lifecycle-to-UI mapping but now emits a richer, stable semantic event envelope alongside or instead of the TL lifecycle:

- Message envelope over WS:
```json
{ "sem": true, "event": { /* SemanticEvent */ } }
```

- SemanticEvent variants (examples):
  - `{ type: "llm.start", id, role, metadata }`
  - `{ type: "llm.delta", id, delta, cumulative, metadata }`  // cumulative optional for redundancy
  - `{ type: "llm.final", id, text, metadata }`
  - `{ type: "tool.start", id, name, input }`
  - `{ type: "tool.delta", id, patch }`
  - `{ type: "tool.done", id }`
  - `{ type: "tool.result", id, result, customKind? }`
  - `{ type: "agent.mode", id, title, from?, to?, analysis? }`
  - `{ type: "log", id, level, message, fields? }`

Notes:
- We will NOT emit legacy `{ tl: true, event }` messages. SEM is the only protocol.
- Semantic payloads are derived from Geppetto events in `forwarder.go` where source fields are available without guesswork.

---

### 3) Frontend store with a rich action log

- The store ingests `{ sem: true, event }` and directly routes to high-level actions without attempting inference:
  - `llm.start` → `llmTextStart(id, role, metadata)`
  - `llm.delta` → `llmTextAppend(id, delta)` (optionally verify cumulative)
  - `llm.final` → `llmTextFinal(id, text, metadata)`
  - `tool.start` → `toolCallStart(id, name, input)`
  - `tool.delta` → `toolCallDelta(id, patch)`
  - `tool.done` → `toolCallDone(id)`
  - `tool.result` → if `customKind` present, create that kind; else `toolCallResult`
  - `agent.mode` → create `agent_mode` entity then complete
  - `log` → create `log_event` then complete

- For a period, keep TL ingestion as fallback; prefer SEM ingestion for action log clarity.

---

### 4) Control flow / data flow

1. Geppetto event occurs.
2. `forwarder.go` converts it to one or more `SemanticEvent` JSON frames `{ sem: true, event }`.
3. WS forwards SEM frames to the client.
4. Frontend `handleIncoming` routes SEM frames directly to semantic actions (no inference, no fallback).
5. Zustand store updates `timeline.byId/order` via high-level actions.
6. UI (`Timeline`) re-renders from selectors; devtools shows semantic action names and payloads.

---

### 5) Backend implementation plan (forwarder.go)

- Add `SemanticEvent` struct and helpers:
  - Types: `LLMStart`, `LLMDelta`, `LLMFinal`, `ToolStart`, `ToolDelta`, `ToolDone`, `ToolResult`, `AgentMode`, `Log`.
  - Envelope: `{ "sem": true, "event": <SemanticEvent> }`.

- Emit SEM events (no TL):
  - EventPartialCompletionStart → `llm.start(id, role=assistant, metadata)`
  - EventPartialCompletion → `llm.delta(id, delta=ev.Delta, cumulative=ev.Completion, metadata)`
  - EventFinal → `llm.final(id, text=ev.Text, metadata)`
  - EventInterrupt → `llm.final(id, text=intr.Text)`
  - EventToolCall → `tool.start(id, name, inputObj)` (parse JSON input once)
  - EventToolCallExecute → `tool.delta(id, patch={ exec: true, input })`
  - EventToolResult / EventToolCallExecutionResult → `tool.result(id, result, customKind?)` and `tool.done(id)`
  - EventAgentModeSwitch → `agent.mode(id, title/message, data)`
  - EventLog → `log(id, level, message, fields)`

- Maintain tool call cache as today to enrich `tool.result` with `customKind` (e.g., `calc_result`).

No TL emission:
- Remove legacy TL emission code-paths; SEM-only output.

---

### 6) Frontend implementation plan (store.js)

- Update `handleIncoming` to route `{ sem: true, event }` only:
  - Switch on `event.type` and call corresponding high-level actions.
  - Remove TL ingestion and inference logic entirely.

- Keep user echo dedupe for locally sent prompts.

- Devtools: ensure action names include payloads for readability.

---

### 7) Data contracts

- LLM
```json
{ "sem": true, "event": { "type": "llm.start", "id": "<uuid>", "role": "assistant", "metadata": { /* usage, model, etc */ } } }
{ "sem": true, "event": { "type": "llm.delta", "id": "<uuid>", "delta": "...", "cumulative": "...", "metadata": { /* optional */ } } }
{ "sem": true, "event": { "type": "llm.final", "id": "<uuid>", "text": "...", "metadata": { /* optional */ } } }
```

- Tool
```json
{ "sem": true, "event": { "type": "tool.start", "id": "<id>", "name": "calc", "input": { /* parsed input */ } } }
{ "sem": true, "event": { "type": "tool.delta", "id": "<id>", "patch": { "exec": true } } }
{ "sem": true, "event": { "type": "tool.result", "id": "<id>", "result": 42, "customKind": "calc_result" } }
{ "sem": true, "event": { "type": "tool.done", "id": "<id>" } }
```

- Agent mode / Log
```json
{ "sem": true, "event": { "type": "agent.mode", "id": "<id>", "title": "...", "from": "...", "to": "...", "analysis": "..." } }
{ "sem": true, "event": { "type": "log", "id": "<id>", "level": "info", "message": "...", "fields": { } } }
```

---

### 8) Rollout strategy

- Single cutover to SEM-only: backend ships SEM frames; frontend only accepts SEM.
- Keep feature-flag to disable SEM briefly if critical issues arise (no TL fallback).

---

### 9) Implementation steps

1) Backend
- [ ] Define `SemanticEvent` structs and `wrapSem(te any) []byte`.
- [x] Introduce `SemanticEventsFromEvent` to produce SEM frames per event case.
- [x] Remove TL emission from the WS forwarder; SEM-only.
- [x] Enhance tool cache to set `customKind` consistently.

2) Frontend
- [x] Update `handleIncoming` to route `{ sem: true, event }` to `llmTextStart/Append/Final`, `toolCallStart/Delta/Done/Result`, etc.
- [x] Remove TL route and any text-diff inference logic.
- [x] Validate action payloads in devtools (names + payloads visible).
- [ ] Verify UI with real streaming and tool flows.

3) Tests
- [ ] Backend unit tests for semantic mapping per Geppetto event type.
- [ ] Frontend unit tests for SEM ingestion → timeline state.

---

### 10) Expected outcome

- Cleaner action history (e.g., `llm.delta`, `tool.result(calc_result)`), minimal frontend inference.
- Tighter contract between inference layer and UI with fewer edge-case bugs.
- Easier extension of new event types without touching UI mapping.


