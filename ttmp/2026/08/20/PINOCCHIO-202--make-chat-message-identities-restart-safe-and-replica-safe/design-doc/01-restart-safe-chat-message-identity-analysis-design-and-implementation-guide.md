---
Title: Restart-Safe Chat Message Identity Analysis Design and Implementation Guide
Ticket: PINOCCHIO-202
Status: active
Topics:
    - chat
    - backend
    - sessionstream
    - persistence
    - runtime
    - design
    - debugging
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/chatapp/chat.go
      Note: Current process-local message allocator and primary integration point
    - Path: repo://pkg/chatapp/plugins/reasoning.go
      Note: Creates reasoning entity IDs beneath the root identity
    - Path: repo://pkg/chatapp/plugins/toolcall.go
      Note: Creates tool-call and result entity IDs beneath the root identity
    - Path: repo://pkg/chatapp/projections.go
      Note: Projects persisted entities keyed by identity and exposes the overwrite symptom
    - Path: repo://pkg/chatapp/runtime_inference.go
      Note: Propagates root message identity into run and segment correlation
    - Path: repo://pkg/chatapp/runtime_sink.go
      Note: Emits runtime events whose entity identities derive from the root message
    - Path: repo://pkg/chatapp/service.go
      Note: Service construction pattern and existing UUID-based request identity precedent
ExternalSources:
    - https://github.com/go-go-golems/pinocchio/issues/202
Summary: Intern-oriented design for replacing Pinocchio chatapp's process-local message counter with restart-safe and replica-safe opaque identities.
LastUpdated: 2026-08-20T11:20:00-04:00
WhatFor: Implement and review GitHub issue 202 without corrupting message grouping, timeline projections, or correlation semantics.
WhenToUse: Before changing chatapp message generation or any derived message, run, segment, tool, widget, or projection identity.
---


# Restart-Safe Chat Message Identity

## 1. Executive summary

Pinocchio's `chatapp.Engine` currently assigns a root identity to every submitted chat run by incrementing an integer stored in the Go process. The resulting values are `chat-msg-1`, `chat-msg-2`, and so on. The mutex around the counter prevents duplicate values inside one `Engine`, but the counter has no durable or distributed scope. A new process or a second replica starts again at 1.

That behavior is incorrect for a chat system whose conversations and timeline projections survive process restarts. The root message ID is not a display sequence number. It is an identity key that propagates into user-message entities, assistant text segments, reasoning segments, run correlation, tool records, warnings, widgets, diagnostics, and frontend grouping. Reusing it causes a later event to update an older persisted entity.

The proposed implementation replaces the counter with an injectable `MessageIDGenerator`. The default generator returns `chat-msg-` followed by a random UUID using the already-required `github.com/google/uuid` package. Tests inject a deterministic generator. Consumers must treat message IDs as opaque strings and use sessionstream ordinals for chronology.

The change deliberately does not restore or reserve a numeric counter in storage. A storage-backed counter would couple the engine to one persistence implementation, require cross-replica transactions, and remain an unnecessary coordination point. Random UUID identity provides the required restart and replica independence without changing protobuf schemas or storage tables.

## 2. Problem statement

GitHub issue [#202](https://github.com/go-go-golems/pinocchio/issues/202) was opened after continuing a persisted CoinVault conversation across a backend restart. Historical entities used root IDs through `chat-msg-6`. The restarted engine generated `chat-msg-1`, `chat-msg-2`, and `chat-msg-3` again.

The persisted timeline showed the consequence directly:

```text
entity_id        created_ordinal  last_event_ordinal
chat-msg-1-user  1                713
chat-msg-2-user  90               753
chat-msg-3-user  185              766
```

The payload at the newer ordinal contained the newer prompt, while the entity retained its historical creation ordinal. The projection had not created a new chat bubble. It had updated an existing `ChatMessage` because the entity key was reused.

Assistant events were persisted successfully, but their root correlation IDs were also reused. Frontend grouping therefore placed new response content under an older response. This proves that the failure was not WebSocket loss. It was identity reuse before transport.

### 2.1 Correctness invariant

For any two distinct accepted submissions, their root message IDs must differ:

```text
submission A != submission B
    implies
rootMessageID(A) != rootMessageID(B)
```

The invariant must hold across:

- goroutines;
- multiple `Engine` objects in one process;
- process restarts;
- multiple service replicas;
- different sessions and conversations;
- replay and externally submitted commands.

### 2.2 Why a mutex is insufficient

The current mutex protects only this set:

```text
{ calls made through one Engine instance during one process lifetime }
```

The required uniqueness domain is:

```text
{ all submissions whose IDs may coexist in persisted events, projections,
  diagnostics, exports, or downstream stores }
```

These domains are materially different. Synchronization provides race safety; it does not provide durable identity.

## 3. Scope

### 3.1 In scope

- Replace `Engine.nextID` and `nextMessageID()` counter allocation.
- Add an injectable generator to `chatapp.Engine` construction.
- Define and document root message ID semantics.
- Preserve the `chat-msg-` diagnostic prefix in the default format.
- Make tests deterministic without depending on production ID values.
- Audit derived identities and consumers for numeric assumptions.
- Add restart, multiple-engine, concurrency, projection, and grouping regression tests.
- Update release notes and API documentation to say IDs are opaque.

### 3.2 Out of scope

- Changing sessionstream ordinals or their ordering semantics.
- Changing session IDs, request IDs, turn IDs, provider call IDs, or tool call IDs.
- Repairing already-collided historical entities automatically.
- Adding a compatibility allocator that can emit the old sequence.
- Parsing UUID timestamps to order messages.
- Changing protobuf field types; the relevant fields are already strings.
- Migrating downstream Redux or React architecture.

### 3.3 Data-repair boundary

The code fix prevents new collisions. It cannot reconstruct overwritten historical user-message payloads if only the latest projected entity remains. Raw sessionstream event history may support a separate repair tool, but that is a distinct operational task requiring backup and idempotency design.

## 4. Terminology

**Root message ID** is the ID allocated once when `ChatStartInference` is accepted. It identifies the logical run and anchors derived records.

**User message ID** is the root plus `-user`.

**Segment message ID** identifies one assistant text or reasoning segment derived from the root and a provider correlation or local segment number.

**Entity ID** is the key used by a sessionstream timeline projection within an entity kind.

**Ordinal** is the monotonically increasing sessionstream event position. It defines event chronology and resume ordering.

**Opaque ID** is an identifier whose internal characters have no consumer-visible meaning. Consumers may compare it for equality and store it, but may not infer sequence or type by parsing it unless a documented derived-ID API says otherwise.

## 5. Current architecture

### 5.1 Submission path

`pkg/chatapp/service.go:60-89` accepts a `PromptRequest`. It already creates a UUID request ID at line 75, stores the in-process pending request, and submits `ChatStartInference` through the sessionstream hub.

The command handler in `pkg/chatapp/runtime_inference.go:21-51` retrieves the request and calls `e.nextMessageID()` at line 39. It immediately derives the user ID and publishes `ChatUserMessageAccepted` before starting inference.

```text
HTTP / application caller
        |
        v
Service.SubmitPromptRequest
        |  request_id = UUID
        v
sessionstream Hub.Submit(ChatStartInference)
        |
        v
Engine.handleStartInference
        |  root = nextMessageID()
        |  user = root + "-user"
        v
ChatUserMessageAccepted + asynchronous run
```

The existing request UUID proves that `github.com/google/uuid` is already part of the package's runtime dependency surface. The root-message allocator is the inconsistent exception.

### 5.2 Engine ownership

`pkg/chatapp/chat.go:52-63` stores:

```go
type Engine struct {
    mu     sync.Mutex
    nextID int
    active map[sessionstream.SessionId]*activeRun
    // ...
}
```

The same mutex protects the counter, active runs, and pending-request operations. `nextMessageID()` increments `nextID` and formats the number. `NewEngine()` initializes active and pending maps but has no persistence-aware initialization step.

The engine is correctly able to load conversational turns from `TurnStore` later in `runRuntimeInference`. Loading content history does not restore identity state. That split explains why language-model context survives a restart while root message identity does not.

### 5.3 Root identity propagation

The root ID flows through the system as follows:

```text
root message ID
├── root + "-user"                      user ChatMessage entity
├── activeRun.messageID                  cancellation / current run identity
├── CorrelationInfo.run_id               cross-event grouping
├── runtimeEventSink.messageID
│   ├── root + ":text:" + segment       assistant ChatMessage entities
│   ├── root + ":thinking:" + segment   reasoning ChatMessage entities
│   └── root + ":warning"               terminal warning entity
├── RuntimeEventContext.MessageID
│   ├── tool event message_id
│   ├── frontend tool bridge context
│   └── application plugins
└── downstream widget parent_message_id and product projections
```

This propagation is intentional. One stable root groups a user submission, its run lifecycle, and all generated output. The defect is not derivation; the defect is allocating a non-unique root.

### 5.4 Text identity

`pkg/chatapp/runtime_sink.go:328-357` creates text segment IDs. When provider correlation supplies a segment ID, Pinocchio sanitizes it and appends it to the root. Otherwise it uses a per-run segment counter:

```text
<root>:text:<provider-segment-id>
<root>:text:<local-segment-number>
```

The local segment number is safe because its namespace is a unique root. It does not need to become globally unique independently.

`pkg/chatapp/projections.go:134-140` recovers the parent by finding the final `:text:` delimiter. A UUID root containing hyphens is compatible. The root must not itself contain the reserved substring `:text:`; the proposed format does not.

### 5.5 Reasoning identity

`pkg/chatapp/plugins/reasoning.go:125-157` derives reasoning IDs in the same manner:

```text
<root>:thinking:<provider-or-local-segment>
```

The plugin also writes `ParentMessageId` explicitly. A longer opaque root does not change its algorithm.

### 5.6 Tool identity

`pkg/chatapp/plugins/toolcall.go:66-80` copies the root into tool event `MessageId`. Timeline entity keys for tools use provider `ToolCallId`, not the root, at lines 99-129. Root uniqueness is still required for grouping, diagnostics, and correlation even where it is not the entity key.

### 5.7 Timeline projection

`pkg/chatapp/projections.go:27-49` projects `ChatUserMessageAccepted` to:

```go
sessionstream.TimelineEntity{
    Kind: "ChatMessage",
    Id:   payload.MessageId,
}
```

Sessionstream treats `(session, kind, id)` as the logical entity. Reusing `chat-msg-1-user` therefore means “update the existing entity,” not “append a new bubble.” This is why the symptom survives correct transport delivery.

Text and reasoning projections likewise use their derived message IDs as timeline entity IDs. Root collision creates whole subtrees of potentially colliding or misleading identifiers.

### 5.8 Frontend contract

Frontends receive both snapshots and live events. They use entity IDs as stable update keys and parent/run IDs for grouping. They must not generate replacement IDs because doing so would cause live and hydrated views to disagree. Identity correctness belongs at event creation in Pinocchio.

## 6. Failure sequence

The concrete restart failure can be expressed as a state transition:

```text
Process A
  nextID = 0
  submit -> chat-msg-1
  ...
  submit -> chat-msg-6
  timeline persists entities and events

Process A exits

Process B
  NewEngine() -> nextID = 0
  loads prior conversational turn content
  submit -> chat-msg-1             COLLISION
  publish -> chat-msg-1-user       COLLISION
  project -> update old entity     DATA LOSS IN PROJECTION
  stream -> correlation run=chat-msg-1
  frontend -> group under old run  PRESENTATION CORRUPTION
```

Two replicas have the same failure without a restart:

```text
Replica A: first submit -> chat-msg-1
Replica B: first submit -> chat-msg-1
```

The mutex cannot coordinate these replicas.

## 7. Requirements

### 7.1 Functional requirements

1. Every root ID generated by default must be collision-resistant without shared state.
2. The root must be a non-empty string.
3. The default root should retain `chat-msg-` for operational recognition.
4. Derived-ID functions must continue to work for arbitrary opaque roots.
5. Tests must be able to supply known IDs.
6. Empty or invalid custom-generator output must fail before publishing an event.
7. Ordering must remain ordinal-based.

### 7.2 Reliability requirements

- No database read is required merely to allocate an ID.
- No cross-replica lock is required.
- Generation is safe when submissions arrive concurrently.
- A generator failure cannot publish a partially identified run.
- A custom generator should be called exactly once per accepted start command.

### 7.3 Compatibility requirements

Existing persisted IDs remain valid opaque strings. The code continues to parse `:text:` only for documented parent derivation. No migration of existing records is required.

Consumers that compare exact test literals must update tests. Consumers that parse the numeric root suffix are unsupported and must be corrected, not accommodated with a legacy mode.

## 8. Proposed design

### 8.1 API

Add a generator function type and engine option:

```go
type MessageIDGenerator func() (string, error)

func WithMessageIDGenerator(generate MessageIDGenerator) Option
```

Store the generator on `Engine`:

```go
type Engine struct {
    mu                 sync.Mutex
    messageIDGenerator MessageIDGenerator
    active             map[sessionstream.SessionId]*activeRun
    // nextID removed
}
```

Default construction installs:

```go
func defaultMessageIDGenerator() (string, error) {
    return "chat-msg-" + uuid.NewString(), nil
}
```

`uuid.NewString()` produces a random UUID and has no operational failure path. The error-returning function signature remains valuable for deterministic fault tests and future generators that may fail.

### 8.2 Allocation

Replace `nextMessageID()` with:

```go
func (e *Engine) newMessageID() (string, error) {
    if e == nil || e.messageIDGenerator == nil {
        return "", errors.New("chat message ID generator is not configured")
    }
    id, err := e.messageIDGenerator()
    if err != nil {
        return "", errors.Wrap(err, "generate chat message ID")
    }
    id = strings.TrimSpace(id)
    if id == "" {
        return "", errors.New("generated chat message ID is empty")
    }
    if strings.Contains(id, ":text:") {
        return "", errors.New("generated chat message ID contains reserved text delimiter")
    }
    return id, nil
}
```

The command handler allocates before any event publication:

```go
messageID, err := e.newMessageID()
if err != nil {
    return err
}
userMessageID := messageID + "-user"
publish(ChatUserMessageAccepted{MessageId: userMessageID})
```

### 8.3 Why return an error

An injected function returning only `string` is simpler, but it cannot represent unavailable entropy, remote generation, or deliberate fault injection. Returning an error keeps allocation failure inside the normal command failure path and ensures no event is published with an empty identity.

### 8.4 Why UUIDv4 rather than UUIDv7 or ULID

UUIDv7 and ULID are time-sortable. Pinocchio must not order chat entities by ID; sessionstream ordinals already provide authoritative ordering. Time-sortability might encourage consumers to establish a second ordering system.

UUIDv4 is already available through a direct dependency and used in `Service.SubmitPromptRequest`. It has a simple no-error API and sufficient collision resistance. If project-wide standards later select UUIDv7, the generator seam permits changing the default without changing event schemas.

### 8.5 Why retain the prefix

The prefix makes logs and exports readable and avoids confusing root IDs with request, session, turn, provider, or tool IDs. It does not carry sequencing semantics.

Example:

```text
root:      chat-msg-cf2ec8d0-1fd4-4b29-bf3d-b38a4206b012
user:      chat-msg-cf2ec8d0-1fd4-4b29-bf3d-b38a4206b012-user
text:      chat-msg-cf2ec8d0-1fd4-4b29-bf3d-b38a4206b012:text:provider-segment
reasoning: chat-msg-cf2ec8d0-1fd4-4b29-bf3d-b38a4206b012:thinking:reasoning-segment
warning:   chat-msg-cf2ec8d0-1fd4-4b29-bf3d-b38a4206b012:warning
```

## 9. API behavior and invariants

### 9.1 Generator lifecycle

The option is evaluated when constructing the engine; the function is invoked per accepted start command. It must not be invoked in `SubmitPromptRequest`, because command submission and command execution may occur in different processes or replay contexts.

### 9.2 Concurrency

The default UUID function requires no engine mutex. Custom generators are responsible for their own concurrency safety. The option documentation must state this explicitly.

Test generators should use an atomic counter or a mutex:

```go
func deterministicGenerator(ids ...string) MessageIDGenerator {
    var mu sync.Mutex
    next := 0
    return func() (string, error) {
        mu.Lock()
        defer mu.Unlock()
        if next >= len(ids) {
            return "", errors.New("test IDs exhausted")
        }
        id := ids[next]
        next++
        return id, nil
    }
}
```

### 9.3 Idempotency

`PromptRequest.IdempotencyKey` is not currently the root identity. This design does not derive message IDs from it. Idempotency and identity are distinct:

- identity says which logical record this is;
- idempotency says whether repeated submission should produce another record.

If sessionstream command idempotency is strengthened later, the already-persisted command/result should determine whether the generator is invoked again.

### 9.4 Reserved delimiters

Current derived APIs use:

- `-user`;
- `:text:`;
- `:thinking:`;
- `:warning`.

The default UUID format cannot contain colons. The minimum validation should reject `:text:` because `parentMessageIDFromSegmentMessageID` parses it. A broader future `MessageID` value object could centralize all delimiter rules, but that is not required for this fix.

## 10. Decision records

### Decision: Generate globally collision-resistant root IDs

- **Context:** Persisted conversations outlive engines and may be served by multiple replicas.
- **Options considered:** In-memory counter; restore counter from timeline; database sequence; random UUID; time-sortable UUID/ULID.
- **Decision:** Use random UUID roots by default.
- **Rationale:** No coordination or persistence dependency is required, and chronology already belongs to ordinals.
- **Consequences:** IDs become longer and nondeterministic; tests must inject or capture values.
- **Status:** proposed.

### Decision: Inject generation at Engine construction

- **Context:** Exact `chat-msg-N` assertions are common in tests, and allocation may need fault testing.
- **Options considered:** Hard-code UUID calls; package-global mutable generator; `Engine` option; generate in `Service`.
- **Decision:** Add `WithMessageIDGenerator` to `Engine`.
- **Rationale:** The engine owns root allocation, options already configure it, and per-engine injection avoids global test interference.
- **Consequences:** Custom generators must be concurrency-safe and return valid opaque IDs.
- **Status:** proposed.

### Decision: Do not preserve sequential production IDs

- **Context:** A compatibility allocator would retain the same restart/replica defect or require shared coordination.
- **Options considered:** Legacy mode; sequence restoration; immediate format change.
- **Decision:** Change the production format directly.
- **Rationale:** IDs are string-typed and no numeric parsing was found in production code.
- **Consequences:** Tests and undocumented consumers assuming literal sequential IDs must change.
- **Status:** proposed.

### Decision: Keep derived-ID structure

- **Context:** Text, reasoning, warning, and user IDs already derive from the root and encode parentage.
- **Options considered:** Give every child an independent UUID; retain root-derived namespaces.
- **Decision:** Retain existing child derivation.
- **Rationale:** Once the root is unique, local child suffixes are safe and preserve efficient grouping/debugging.
- **Consequences:** Root validation must protect reserved delimiters.
- **Status:** proposed.

### Decision: Keep chronology in ordinals

- **Context:** UUIDs do not provide meaningful display order.
- **Options considered:** UUIDv7/ULID sorting; timestamps; sessionstream ordinals.
- **Decision:** Continue using sessionstream ordinals exclusively.
- **Rationale:** Ordinals already define total session order, resume position, and projection versions.
- **Consequences:** Documentation must explicitly prohibit lexical ID ordering.
- **Status:** proposed.

## 11. Implementation phases

## Phase 0: Baseline and consumer audit

Files:

- `pkg/chatapp/chat.go`
- `pkg/chatapp/runtime_inference.go`
- `pkg/chatapp/runtime_sink.go`
- `pkg/chatapp/projections.go`
- `pkg/chatapp/plugins/reasoning.go`
- `pkg/chatapp/plugins/toolcall.go`
- downstream repositories using chatapp

Tasks:

1. Run `go test ./pkg/chatapp/...` before changes.
2. Search production code for numeric parsing or lexical ordering of message IDs.
3. Record direct exact-ID assertions separately from behavior assertions.
4. Confirm no public documentation promises sequential values.

Exit criterion: all production consumers treat roots as opaque or have an explicit migration task.

## Phase 1: Generator API

Modify `pkg/chatapp/chat.go`:

1. Add `MessageIDGenerator`.
2. Add `WithMessageIDGenerator`.
3. Add default UUID implementation.
4. Remove `nextID`.
5. Add validation and contextual errors.

Add focused tests for:

- default prefix and UUID parseability;
- deterministic injection;
- empty result;
- generator error;
- reserved delimiter;
- concurrent default generation uniqueness.

Exit criterion: allocation is independent of engine state and fully unit-tested.

## Phase 2: Command-handler integration

Modify `pkg/chatapp/runtime_inference.go`:

1. Handle allocation error before publication.
2. Assert the generator is invoked once per accepted command.
3. Preserve existing derivation for the user ID and runtime root.
4. Add a test proving no event is published when generation fails.

Exit criterion: a failed allocator leaves no user, run, or derived event.

## Phase 3: Test migration

Update tests that expect `chat-msg-1` because they exercise engine allocation. Prefer injection:

```go
engine := NewEngine(WithMessageIDGenerator(
    deterministicGenerator("chat-msg-test-run"),
))
```

Do not replace meaningful assertions with weak prefix checks. Tests of parent/child propagation should use a known injected root and assert the complete derived IDs.

Fixture-only tests that construct protobuf messages directly may keep human-readable IDs such as `chat-msg-1`; they do not test production allocation.

Exit criterion: tests remain precise and no package-global generator mutation exists.

## Phase 4: Restart and replica regressions

Add a test harness using persistent sessionstream storage:

```text
create persistent hub/store
create engine A + service A
submit and wait
discard engine A
create engine B + service B over same storage
submit and wait
snapshot
assert distinct root/user/text entities
```

Add a multi-engine concurrency test:

```text
N engines × M submissions
collect ChatRunStarted root IDs
assert count(unique IDs) == N*M
```

Use the default generator in uniqueness tests. Use deterministic generators only where exact values are required.

Exit criterion: the original failure is reproduced by a counter-based test fixture and prevented by the production generator.

## Phase 5: Downstream validation

Validate at least:

- Pinocchio web-chat hydration and live streaming;
- CoinVault continuation after backend restart;
- chat-provider grouping of opaque IDs;
- reasoning, tools, widgets, and attachments;
- exports, diagnostics, and feedback targeting.

CoinVault acceptance scenario:

1. Create six messages.
2. Restart backend without deleting timeline/turn databases.
3. Continue the same conversation.
4. Confirm a new user entity is created.
5. Confirm response content groups beneath the new run.
6. Confirm existing bubbles are unchanged.

## Phase 6: Documentation and release

1. Update package documentation: message IDs are opaque strings.
2. Add a changelog entry describing restart and replica safety.
3. Link issue #202 and include regression evidence.
4. Release Pinocchio according to repository policy.
5. Bump downstream modules and repeat the CoinVault scenario.

## 12. Testing strategy

### 12.1 Unit tests

Test generator validation independently. Table cases should cover whitespace, empty output, reserved delimiters, and explicit errors.

Test default uniqueness with sufficient concurrent calls to exercise synchronization assumptions. This is not a probabilistic proof; it is a regression check that no shared counter or constant slipped into the implementation.

### 12.2 Projection tests

Publish two user events with distinct generated roots and assert two timeline entities exist. Then publish text/reasoning events and assert parent IDs point to the correct roots.

### 12.3 Restart tests

The restart test is the primary regression. It must construct a new engine, not merely reset a field on the old engine. It should reuse the same persisted session and timeline store.

### 12.4 Replica tests

Construct multiple engines concurrently. The test need not run separate OS processes because default UUID generation is process-independent by design, but a higher-level integration test may do so if the repository already has a multi-process harness.

### 12.5 Full gates

```bash
go test ./pkg/chatapp/...
go test ./cmd/web-chat/...
go test ./...
make lint
make gosec
```

### 12.6 Downstream gates

For CoinVault:

```bash
go test ./internal/webchat/...
cd web
pnpm typecheck
pnpm test:unit
pnpm build
```

Automated tests do not replace the backend-restart browser scenario because the reported defect crosses persistence, hydration, and grouping.

## 13. Security and operational analysis

UUIDs are identifiers, not authorization tokens. They must not be used as proof of access. Existing session authorization remains authoritative.

Random identifiers reduce easy enumeration compared with sequential values, but this is incidental and must not be presented as an access-control guarantee.

IDs appear in logs, database keys, JSON, protobuf strings, URLs, React keys, and telemetry. UUID strings are safe in these contexts without escaping beyond normal field encoding.

Avoid attaching prompt text, user identity, timestamps, hostnames, or replica IDs to the generated value. Such data creates privacy and topology leakage without improving correctness.

## 14. Performance analysis

UUID generation cost is negligible relative to command persistence, model inference, and WebSocket publication. Removing the engine mutex from root allocation may slightly reduce contention.

Longer IDs increase storage and index size. The increase is bounded: approximately 36 UUID characters plus the existing prefix. Chat volume and payload sizes make this cost insignificant compared with text, reasoning, and tool payloads.

Derived IDs become longer because they include the root. Existing string fields and SQLite text keys have no relevant fixed-length limit.

## 15. Risks and mitigations

### Risk: hidden consumer parses numeric suffix

Mitigation: repository-wide and downstream search, release note, and opaque-ID documentation. Do not add a legacy mode unless a concrete supported consumer is identified.

### Risk: tests become nondeterministic

Mitigation: inject a per-engine deterministic generator. Never replace exact relationship assertions with broad regex checks.

### Risk: generator emits invalid root

Mitigation: validate non-empty output and reserved delimiters before publishing any event.

### Risk: idempotent retry generates a second ID

Mitigation: retain existing command/idempotency semantics and add a dedicated test if command replay can re-enter the handler. Do not derive IDs from human-provided idempotency keys without a separate security/design review.

### Risk: old corrupted data remains

Mitigation: clearly separate prevention from repair. Preserve raw event databases before attempting any repair.

### Risk: ordering accidentally changes

Mitigation: assert ordinal-based ordering in projection and frontend tests; document IDs as opaque.

## 16. Alternatives considered

### Restore the maximum numeric suffix from the timeline

Rejected. It requires scanning or metadata persistence during engine initialization and cannot coordinate multiple replicas without a transactionally reserved range.

### Use a database sequence

Rejected. It creates a required database dependency for an otherwise portable engine and adds latency and failure modes to every accepted command.

### Prefix a local counter with session ID

Rejected. A restarted engine serving the same session still collides, and the root IDs become unnecessarily coupled to externally supplied session values.

### Prefix with process or replica ID

Rejected. Stable replica identity requires configuration, leaks topology, and still needs collision-safe generation of that identity.

### Use request ID as root ID

Plausible but not selected. The request UUID already exists before command submission, but replay/external command paths and the distinction between request identity and run identity deserve separation. Reusing it would also make message allocation dependent on always-valid command payloads. A dedicated root generator keeps ownership explicit.

### Use UUIDv7 or ULID

Plausible but unnecessary. Sortability is not a requirement and may invite misuse. The generator seam allows a later project-wide decision.

## 17. Open questions

1. Should custom generator output be required to include `chat-msg-`, or should that be only a default convention? Recommendation: default convention only; tests and applications may use another opaque root.
2. Should all reserved derived delimiters be rejected now, or only `:text:` because production code parses it? Recommendation: reject `:text:` initially and document the full reserved set.
3. Can a sessionstream command replay invoke generation twice for one logically idempotent command? Verify before implementation completion.
4. Does any downstream analytics system infer turn number from `chat-msg-N`? Repository source did not reveal one; downstream search is still required.
5. Should already-corrupted CoinVault development data be rebuilt from raw events or discarded? Treat as a separate operational decision.

## 18. Review guide

Review in this order:

1. `pkg/chatapp/chat.go`: generator type, default, option, validation, and removal of `nextID`.
2. `pkg/chatapp/runtime_inference.go`: allocation occurs once and before publication.
3. Generator tests: deterministic injection and failure behavior.
4. Restart/replica tests: they must cross a new-engine boundary.
5. Derived-ID tests: root propagation remains exact.
6. Downstream browser evidence: no overwrite or mis-grouping after restart.

Reject the implementation if:

- production still relies on an engine-local numeric counter;
- uniqueness requires reading the current timeline;
- a package-global mutable test hook is introduced;
- message ordering is inferred from UUID strings;
- empty generator output can reach an event;
- a compatibility allocator preserves unsafe sequential production behavior;
- tests merely check a prefix and stop asserting parent/correlation relationships.

## 19. File and API reference

### Root allocation and submission

- `pkg/chatapp/chat.go:50-63` — engine fields and current counter.
- `pkg/chatapp/chat.go:71-131` — existing option/NewEngine pattern.
- `pkg/chatapp/chat.go:215-220` — defective allocator.
- `pkg/chatapp/service.go:60-89` — app-facing submission and existing UUID request ID.
- `pkg/chatapp/runtime_inference.go:21-51` — allocation, user derivation, and run creation.

### Propagation and derivation

- `pkg/chatapp/runtime_inference.go:71-214` — run events and correlation.
- `pkg/chatapp/runtime_sink.go:328-372` — text segment derivation and correlation sanitization.
- `pkg/chatapp/messages.go:11-17` — warning derivation.
- `pkg/chatapp/features.go:18-53` — plugin runtime context.
- `pkg/chatapp/plugins/reasoning.go:125-157` — reasoning derivation.
- `pkg/chatapp/plugins/toolcall.go:66-129` — tool propagation and projection.
- `pkg/chatapp/widgets/plugin.go:98-150` — widget identity and parent message field.

### Projection and tests

- `pkg/chatapp/projections.go:27-152` — ChatMessage entity keys and parent parsing.
- `pkg/chatapp/chat_test.go` — engine-level exact IDs and derived relationships.
- `pkg/chatapp/projections_protocol_test.go` — projection/correlation contract.
- `pkg/chatapp/runtime_sink_protocol_test.go` — runtime event identity propagation.
- `pkg/chatapp/plugins/reasoning_test.go` — reasoning parent relationships.
- `pkg/chatapp/plugins/toolcall_test.go` — tool message correlation.
- `cmd/web-chat/reasoning_chat_feature_test.go` — downstream web-chat grouping.

### Dependencies and external record

- `go.mod:23` — direct `github.com/google/uuid v1.6.0` dependency.
- [GitHub issue #202](https://github.com/go-go-golems/pinocchio/issues/202).

## 20. Intern handoff checklist

Before coding:

- Read Sections 5-10.
- Run the baseline tests.
- Search downstream consumers.
- Confirm the no-compatibility decision with the issue owner.

During coding:

- Keep generator changes in the engine package.
- Allocate once before publication.
- Use injected deterministic IDs in behavior tests.
- Preserve derived suffix and correlation assertions.
- Commit the API/core change separately from broad test fixture updates.

Before review:

- Run all gates.
- Execute the persistent restart scenario.
- Capture entity IDs and ordinals before and after restart.
- Update this ticket's diary, tasks, related files, and changelog.
- Update GitHub issue #202 with validation evidence.
