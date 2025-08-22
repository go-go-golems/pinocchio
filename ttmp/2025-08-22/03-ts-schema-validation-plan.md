### Plan: Add TypeScript types and schema validation for incoming semantic events

This plan describes how we will introduce a strongly-typed, schema-validated pipeline for incoming WebSocket events (semantic and transitional TL) to improve reliability, observability, and developer ergonomics.

---

### 1) Goals and non-goals

- Goals
  - Runtime validation of WS messages with helpful error reports.
  - Single source of truth for event contracts that can be shared across Go and TS.
  - Strong TypeScript types for handlers and store actions without manual duplication.
  - Minimal overhead for the UI action routing and devtools readability.
- Non-goals
  - Changing existing UI components or timeline rendering logic (beyond using the validated events).
  - Replacing backend event sources. We only structure the payload contract and validation.

---

### 2) Contract model overview

- Envelope
  - Semantic events: `{ sem: true, schemaVersion: "v1", event: <SemanticEvent> }`
  - Transitional TL events: `{ tl: true, event: <TimelineEvent> }` (kept as fallback during migration)
- SemanticEvent (examples)
  - `llm.start`, `llm.delta`, `llm.final`
  - `tool.start`, `tool.delta`, `tool.done`, `tool.result`
  - `agent.mode`, `log`
  - Each variant includes a `type` discriminator and required fields (e.g., `id`, `text|delta`, etc.)
- Versioning
  - `schemaVersion` required in semantic envelope; starting at `v1`.
  - Maintain compatibility windows during rollout; new versions are additive or gated.

---

### 3) Backend source of truth (Go)

- Define structs for the semantic envelope and each event variant under `cmd/web-chat/pkg/backend`:
  - `SemanticEnvelope{ Sem bool, SchemaVersion string, Event any }`
  - Event variants with a fixed `Type string` field (e.g., `jsonschema:"enum=llm.start"`).
  - Keep IDs, metadata, and fields aligned with what the UI needs.
- Generate JSON Schema from Go types using `github.com/invopop/jsonschema`:
  - Produce per-variant schemas and a top-level `SemanticEvent` defined as a `oneOf` discriminated by `type`.
  - Emit an `Envelope` schema referencing `SemanticEvent`.
  - Output path: `cmd/web-chat/static/schema/semantic-events.v1.json` (or `pinocchio/cmd/web-chat/static/...`).
- Serve schema file(s) over HTTP for visibility, but also commit them to repo to keep build reproducible.

---

### 4) TypeScript types and runtime validators

- Types from JSON Schema
  - Use `json-schema-to-typescript` in a script to convert the published schema into TS types.
  - Output: `web/src/types/semantic-events.ts` exporting `SemanticEnvelope`, `SemanticEvent`, and the per-variant types.
- Runtime validation
  - Use Ajv (2020-12) to compile the same JSON Schema into validators:
    - A validator for the envelope.
    - A map of validators keyed by `event.type` for fast routing.
  - Turn on `allErrors: true`, `strict: false` (or set strict rules as desired), and `discriminator: true` (if used).
- Developer ergonomics
  - The handler calls one compiled validator (envelope) and one per-type validator; on success, the event is fully typed.
  - Validation errors are logged with `schemaVersion`, `type`, connection id, and a payload snippet.

---

### 5) Frontend ingestion and store routing

- `handleIncoming` priority
  - Prefer `{ sem: true }` → validate → typed switch on `event.type` → call semantic store actions (`llmTextStart/Append/Final`, `toolCallStart/Delta/Done/Result`, etc.).
  - Keep `{ tl: true }` as fallback during migration; validate TL with a minimal schema to catch regressions.
- Devtools action log
  - Ensure each action includes concise payloads (ids, sizes), keeping logs readable.
- Error UX (optional)
  - Surface a user-level banner for repeated validation failures; include a `copy error details` button for bug reports.

---

### 6) Build and tooling

- Backend (Go)
  - `make gen-schema` or `go run ./cmd/gen-schema` to output `semantic-events.v1.json`.
  - Commit the schema file.
- Frontend (Node)
  - `pnpm ts:gen` (or `npm run ts:gen`) to run a script that:
    - Imports JSON Schema.
    - Generates types via `json-schema-to-typescript` into `web/src/types/semantic-events.ts`.
    - Optionally writes a small `validators.ts` that precompiles Ajv validators and exports them.
- CI
  - Validation step that ensures the generated TS types are up to date with the schema (diff fail if drift).
  - Optionally, run a quick sample validation test against fixtures.

---

### 7) Rollout strategy

- Phase 1 (dual emission)
  - Backend emits both SEM and TL frames.
  - Frontend consumes SEM first; TL used only as fallback.
  - Log and compare: count of SEM vs TL per type; track validation failures.
- Phase 2 (SEM-only routing)
  - Remove TL inference from the client (keep minimal TL handling behind a flag for rollback).
  - Harden error logging and add per-connection stats.
- Phase 3 (TL disable)
  - Backend stops emitting TL frames (behind config).
  - Remove TL code paths from frontend.

---

### 8) Observability and diagnostics

- Structured logs on both sides with event ids, run ids, types, and versions.
- Counters: number of SEM frames accepted/rejected by validator; types distribution; average payload sizes.
- Sampling: on validation failure, attach minified payload and Ajv error paths.

---

### 9) Risks and mitigations

- Drift between Go structs and the JSON Schema
  - Mitigation: generate schema from Go; CI enforcement; avoid hand-written schema edits.
- Validator performance in the browser
  - Mitigation: precompile Ajv; validate only envelope + per-type; skip TL validation in production once SEM stabilizes.
- Breaking changes to event shapes
  - Mitigation: include `schemaVersion`, support multiple versions temporarily; add feature flags in both layers.

---

### 10) Implementation steps (detailed)

1) Backend
  - Add Go structs for `SemanticEnvelope` and event variants with `Type` discriminator.
  - Implement `SemanticEventsFromEvent(e events.Event) [][]byte` returning `{ sem: true }` frames.
  - Update the forwarder to send SEM frames (and continue TL frames initially).
  - Add schema generation code using `invopop/jsonschema`; output `semantic-events.v1.json`.
  - Add config/env flag to disable TL emission later.

2) Frontend
  - Add `web/src/types/semantic-events.ts` (generated) and `web/src/validation/validators.ts` (Ajv setup).
  - Update `handleIncoming` to:
    - Detect SEM, validate envelope and event, and route to store actions using typed event.
    - Detect TL and route to lifecycle fallback (temporary) with minimal validation.
  - Add structured error logs for validation failures.

3) Tooling & CI
  - Add `make gen-schema` and a Node script `scripts/gen-types-from-schema.mjs`.
  - CI job to run both and verify no diffs (schema/types up-to-date).

4) Documentation
  - Update developer docs with the contract, versioning rules, and scripts to run.

---

### 11) Future enhancements

- Consider shipping a tiny `@pinocchio/semantic-events` package that exports TS types and precompiled validators to avoid per-app boilerplate.
- Add a `traceId`/`source` field to every semantic event to aid debugging across services.
- Add optional compression for large deltas or results.


