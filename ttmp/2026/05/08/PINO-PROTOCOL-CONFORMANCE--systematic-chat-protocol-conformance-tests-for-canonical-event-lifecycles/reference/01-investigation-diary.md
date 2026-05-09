---
Title: Investigation diary
Ticket: PINO-PROTOCOL-CONFORMANCE
Status: active
Topics:
    - pinocchio
    - chat
    - frontend
    - sessionstream
    - architecture
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts
      Note: Investigated frontend patch merge semantics.
    - Path: pinocchio/cmd/web-chat/web/src/ws/timelineEvents.ts
      Note: Investigated frontend sparse patch construction.
    - Path: pinocchio/pkg/chatapp/plugins/reasoning.go
      Note: Investigated reasoning segment identity behavior.
    - Path: pinocchio/pkg/chatapp/plugins/toolcall.go
      Note: Investigated tool sparse merge behavior.
    - Path: pinocchio/pkg/chatapp/runtime_inference.go
      Note: Investigated runtime run lifecycle and failure handling.
    - Path: pinocchio/pkg/chatapp/runtime_sink.go
      Note: Investigated active text finalization on stop/error.
ExternalSources: []
Summary: Chronological diary for the PINO-PROTOCOL-CONFORMANCE research/design task.
LastUpdated: 2026-05-09T03:55:00-04:00
WhatFor: Use this diary to understand how the protocol conformance guide was created and how to continue the work.
WhenToUse: Use when implementing, reviewing, or extending the protocol conformance test plan.
---


# Diary

## Goal

Capture the research and documentation work for `PINO-PROTOCOL-CONFORMANCE`: a new Pinocchio ticket that turns recent review-driven edge-case fixes into a systematic protocol conformance testing strategy.

## Step 1: Create the ticket workspace

I created a dedicated Pinocchio ticket so the protocol-testing work has its own durable home instead of being buried inside the larger Geppetto event-vocabulary migration. The goal was to separate implementation planning from immediate PR review response work.

### Prompt Context

**User prompt (verbatim):** The original user wording was not preserved in the compacted conversation context. The compacted task summary states: "The user asked to create a new ticket for the previously discussed issue, handle it systematically, and write a detailed intern-oriented analysis/design/implementation guide. The guide should include prose, bullets, pseudocode, diagrams, API references, and file references, be stored in the ticket, and then be uploaded to reMarkable."

**Assistant interpretation:** Create a new `docmgr` ticket in the Pinocchio ticket root, research the current chat protocol implementation, write a detailed implementation guide, validate the ticket, and upload the guide to reMarkable.

**Inferred user intent:** Replace reactive review-comment fixes with a clear, teachable, repeatable conformance-testing plan that a newer engineer can implement safely.

**Commit (code):** N/A — documentation/research ticket only.

### What I did

- Loaded the `ticket-research-docmgr-remarkable` skill and its writing/checklist references.
- Loaded the `diary` skill to keep this chronological record.
- Created ticket `PINO-PROTOCOL-CONFORMANCE` under the Pinocchio ticket root:
  - `/home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles`
- Created the primary design doc:
  - `design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md`
- Created this diary:
  - `reference/01-investigation-diary.md`

### Why

The event vocabulary migration touched Geppetto, Pinocchio, browser event mapping, and debug trace tooling. Recent Pinocchio PR feedback showed that the protocol needs explicit invariants and matrix tests instead of isolated patches.

### What worked

- The ticket was created successfully with standard files:
  - `index.md`
  - `README.md`
  - `tasks.md`
  - `changelog.md`
  - design doc
  - diary

### What didn't work

- `docmgr` initially picked up the parent/root workspace config rather than the Pinocchio ticket root. The workaround was to use the absolute Pinocchio docs root for subsequent `docmgr` commands:
  - `/home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp`

### What I learned

- In this multi-repository workspace, ticket operations should pass `--root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp` when the intended ticket belongs to Pinocchio.

### What was tricky to build

The tricky part was not the ticket creation itself; it was keeping the ticket scoped to Pinocchio while the shell working directory sits at the multi-repo workspace root. Without an explicit `--root`, docmgr can choose the wrong `ttmp` root.

### What warrants a second pair of eyes

- Confirm the ticket belongs in Pinocchio rather than the parent workspace or Geppetto ticket tree.

### What should be done in the future

- Use explicit `--root` for all follow-up docmgr commands in this ticket.

### Code review instructions

- Review the ticket path and generated documents.
- Validate with:
  - `docmgr doctor --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE --stale-after 30`

### Technical details

Ticket path:

```text
/home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles
```

## Step 2: Gather source evidence and map the protocol pipeline

I gathered line-referenced evidence from the runtime, projections, plugins, persistence code, protobuf contract, and frontend mapper/reducer. This ensured the design guide is grounded in actual source behavior rather than an abstract testing wishlist.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Build the implementation guide from concrete source references and explain the pipeline clearly enough for an intern to implement tests.

**Inferred user intent:** Make the guide evidence-backed and actionable, with file references and protocol-specific reasoning.

**Commit (code):** N/A — documentation/research ticket only.

### What I did

- Gathered source excerpts into `/tmp/pino_protocol_sources.txt` from:
  - `pkg/chatapp/runtime_inference.go`
  - `pkg/chatapp/runtime_sink.go`
  - `pkg/chatapp/projections.go`
  - `pkg/chatapp/plugins/toolcall.go`
  - `pkg/chatapp/plugins/reasoning.go`
  - `cmd/web-chat/web/src/ws/timelineEvents.ts`
  - `cmd/web-chat/web/src/store/timelineSlice.ts`
  - `pkg/ui/timeline_persist.go`
  - `proto/pinocchio/chatapp/v1/chat.proto`
  - `pkg/chatapp/chat_test.go`
  - `cmd/web-chat/web/src/ws/wsManager.test.ts`
- Inspected Geppetto canonical event APIs for upstream protocol shape:
  - `geppetto/pkg/events/correlation.go`
  - `geppetto/pkg/events/canonical_events.go`
  - `geppetto/pkg/events/canonical_tool_events.go`
- Used `rg -n` to capture exact line anchors for key functions and schemas.

### Why

The requested guide needed prose, bullets, pseudocode, diagrams, API references, and file references. The safest way to produce that was to first map the real pipeline and line-anchor the major claims.

### What worked

- The source inspection identified the full pipeline:
  - Geppetto canonical events
  - `runtimeEventSink`
  - Pinocchio protobuf backend events
  - sessionstream UI/timeline projections
  - timeline persistence
  - frontend UI-event mapping
  - Redux sparse patch merging
- Existing regression tests were found that already cover some important lifecycle rows:
  - runtime error after partial text
  - interrupt after partial text
  - error after closed segment
  - sparse frontend correlation props
  - sparse tool finish preserving input props

### What didn't work

- No command failures occurred during source inspection.
- The evidence dump was intentionally temporary under `/tmp`; the distilled evidence is now incorporated directly into the design doc instead of committing the full dump.

### What I learned

- The protocol bugs cluster around two contracts:
  1. terminal lifecycle handling for active child entities;
  2. sparse patch merging that must preserve prior meaningful state.
- Backend and frontend have parallel sparse-merge logic:
  - backend tool projection uses `mergeToolCallFields`;
  - frontend mapping uses `definedProps` and Redux prop merging.

### What was tricky to build

The tricky part was separating provider-call terminality from transcript terminality. Provider final events should not manufacture text; text finalization only belongs to actual text segments or active text segments being closed by stop/error handling.

### What warrants a second pair of eyes

- Verify that the proposed invariants align with the intended semantics of future multi-segment or concurrent provider streams.
- Verify that trace replay should initially use curated frontend frames rather than full SQLite artifacts.

### What should be done in the future

- If the deterministic matrix reveals repeated runtime-state logic, refactor `runtimeEventSink` into an explicit state-machine type after tests land.

### Code review instructions

- Start with these implementation files:
  - `pinocchio/pkg/chatapp/runtime_inference.go`
  - `pinocchio/pkg/chatapp/runtime_sink.go`
  - `pinocchio/pkg/chatapp/projections.go`
  - `pinocchio/pkg/chatapp/plugins/toolcall.go`
  - `pinocchio/pkg/chatapp/plugins/reasoning.go`
  - `pinocchio/pkg/ui/timeline_persist.go`
  - `pinocchio/cmd/web-chat/web/src/ws/timelineEvents.ts`
  - `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts`
- Validate line references with `rg -n` if files have moved.

### Technical details

Representative evidence command:

```bash
cd /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio
rg -n "func \\(e \\*Engine\\) runRuntimeInference|func \\(s \\*runtimeEventSink\\) PublishEvent|finishActiveTextSegment|func baseTimelineProjection|function timelineMutationFromUIEvent|function definedProps|upsertEntity\\(|message CorrelationInfo" \
  pkg/chatapp/runtime_inference.go \
  pkg/chatapp/runtime_sink.go \
  pkg/chatapp/projections.go \
  cmd/web-chat/web/src/ws/timelineEvents.ts \
  cmd/web-chat/web/src/store/timelineSlice.ts \
  proto/pinocchio/chatapp/v1/chat.proto
```

## Step 3: Write the intern-oriented design and implementation guide

I wrote the primary design document as a practical implementation guide rather than a high-level essay. It defines the protocol, explains the current pipeline, names lifecycle invariants, proposes deterministic test matrices, and includes pseudocode for Go runtime tests, plugin projection tests, frontend reducer tests, persistence tests, and trace replay.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Produce a durable, detailed design doc in the ticket that a newer engineer can follow step by step.

**Inferred user intent:** Make future protocol work less dependent on tribal memory and reduce repeated review-cycle fixes.

**Commit (code):** N/A — documentation/research ticket only.

### What I did

- Replaced the generated design-doc template with a full guide:
  - `design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md`
- Included:
  - executive summary;
  - problem statement and scope;
  - glossary;
  - source evidence table;
  - Mermaid pipeline diagrams;
  - protocol invariants `I1` through `I7`;
  - Go runtime matrix;
  - plugin projection matrix;
  - TypeScript reducer-backed matrix;
  - timeline persistence matrix;
  - trace replay design;
  - implementation phases;
  - validation commands;
  - risks, alternatives, and open questions.

### Why

The user specifically asked for an intern-oriented analysis/design/implementation guide with prose, bullets, pseudocode, diagrams, API references, and file references.

### What worked

- The guide now gives concrete file names for the next implementation PR:
  - `pkg/chatapp/chat_protocol_conformance_test.go`
  - `pkg/chatapp/plugins/toolcall_protocol_conformance_test.go`
  - `pkg/chatapp/plugins/reasoning_protocol_conformance_test.go`
  - `pkg/ui/timeline_persist_protocol_test.go`
  - `cmd/web-chat/web/src/ws/protocolConformance.test.ts`
- The guide explains why fuzzing should come after deterministic invariant tests.

### What didn't work

- No write or validation failure occurred while drafting the guide.

### What I learned

- The most useful first abstraction is a test-only `ProtocolProgram`, not a production state machine.
- The conformance layer should assert final semantic entities rather than private implementation fields.

### What was tricky to build

The guide had to be detailed without prescribing premature production refactors. The recommended path is tests first, then optional state-machine refactor once the behavior is protected.

### What warrants a second pair of eyes

- Whether `ChatTextSegmentFinished` with empty content should be treated as a sparse terminal UI patch that preserves previous content, or whether backend should always ensure final content before UI projection.
- Whether the first trace replay fixtures should live in source testdata or remain ticket artifacts until stable.

### What should be done in the future

- Implement Phase 1 through Phase 4 as a focused Pinocchio PR.
- Promote a curated trace fixture only after the deterministic matrix is stable.

### Code review instructions

- Read the design doc from top to bottom once.
- Then review the proposed matrices against current tests:
  - `pkg/chatapp/chat_test.go`
  - `cmd/web-chat/web/src/ws/wsManager.test.ts`
- Validate docs with:
  - `docmgr doctor --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE --stale-after 30`

### Technical details

Primary design doc:

```text
/home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md
```

## Step 4: Validate and upload the ticket bundle

I validated the ticket with `docmgr doctor` and uploaded the guide bundle to reMarkable. The bundle includes the design guide, diary, tasks, and changelog so the reviewer can read both the plan and the investigation context on-device.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the ticket deliverable by validating docs and publishing the bundle to reMarkable.

**Inferred user intent:** Make the protocol conformance plan available outside the terminal workflow for review and annotation.

**Commit (code):** N/A — documentation/research ticket only.

### What I did

- Ran `docmgr doctor` for the Pinocchio ticket root.
- Adjusted ticket topics to existing vocabulary values:
  - `pinocchio`
  - `chat`
  - `frontend`
  - `sessionstream`
  - `architecture`
- Performed a dry-run reMarkable bundle upload.
- Uploaded the bundle to:
  - `/ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE/PINO_PROTOCOL_CONFORMANCE_chat_protocol_guide.pdf`
- Updated `tasks.md` and `changelog.md` with validation/upload results.

### Why

The ticket should be valid according to `docmgr` and the requested guide should be available on reMarkable for reading and annotation.

### What worked

- `docmgr doctor --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE --stale-after 30` passed.
- The reMarkable dry-run showed the intended bundle contents and destination.
- The actual upload succeeded with:
  - `OK: uploaded PINO_PROTOCOL_CONFORMANCE_chat_protocol_guide.pdf -> /ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE`
- Correction: the bundle was reuploaded with the default PDF layout after removing the layout option from the local reMarkable upload skill.

### What didn't work

- The first `docmgr doctor` run warned about unknown topics `chatapp` and `protocol-testing`.
- I initially tried adding those vocabulary values, but `docmgr` resolved the vocabulary file through the parent workspace config. I reverted that vocabulary change and instead updated the ticket topics to existing known vocabulary values.

### What I learned

- For this workspace, topic choices should prefer the existing vocabulary unless there is a strong reason to add new terms.
- The reMarkable upload path can be validated with a dry-run before running the actual upload.

### What was tricky to build

The tricky part was avoiding accidental vocabulary churn in the sibling CoinVault ticket root. The safer fix was to use existing topic slugs rather than add new vocabulary entries through the workspace-level config.

### What warrants a second pair of eyes

- Confirm the topic vocabulary choice is acceptable: the ticket uses `chat` and `architecture` instead of narrower `chatapp` and `protocol-testing` slugs.

### What should be done in the future

- Implement the conformance matrices described in the design doc.
- If `protocol-testing` becomes a recurring topic, add it deliberately to the correct shared vocabulary in a separate docs hygiene change.

### Code review instructions

- Check `tasks.md` and `changelog.md` for validation/upload status.
- Confirm the uploaded bundle name and destination:
  - `PINO_PROTOCOL_CONFORMANCE_chat_protocol_guide.pdf`
  - `/ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE`

### Technical details

Validation command:

```bash
docmgr doctor --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE --stale-after 30
```

Dry-run upload command:

```bash
remarquee upload bundle \
  "$TICKET_DIR/design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md" \
  "$TICKET_DIR/reference/01-investigation-diary.md" \
  "$TICKET_DIR/tasks.md" \
  "$TICKET_DIR/changelog.md" \
  --name "PINO PROTOCOL CONFORMANCE chat protocol guide" \
  --remote-dir "/ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE" \
  --toc-depth 2 \
  --dry-run \
  --non-interactive
```

Upload result:

```text
OK: uploaded PINO_PROTOCOL_CONFORMANCE_chat_protocol_guide.pdf -> /ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE
```

## Step 5: Expand all guides to cover provider-native normalization

I updated the protocol conformance documentation after the user pointed out that the first pass still started too high in the stack. The lowest layer is the provider-specific normalization layer inside Geppetto, and it is arguably the most complex layer because each provider exposes a different streaming vocabulary before Geppetto emits canonical events.

### Prompt Context

**User prompt (verbatim):** "are these documents even addressing the lowest level of transforming provider specific events into the geppetto events? We had quite a bit of issues with those too."

**Follow-up user prompt (verbatim):** "in the 01-... document too! all 3"

**Follow-up user prompt (verbatim):** "don't overwrite after ruploading btw. Update all 3 documents as needed to cover the lowest layer too, in fact I think it's the most complex layer, isn't it?"

**Assistant interpretation:** Expand the original design guide, the static analysis guide, and the finite-state model guide so they all treat provider-native-to-Geppetto normalization as Phase 1 and as a first-class conformance target.

**Inferred user intent:** Ensure the testing strategy catches bugs at the earliest provider adapter boundary, not only after events have already become canonical Geppetto events.

**Commit (code):** N/A — documentation update only.

### What I did

- Updated `design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md` to add:
  - provider-native stream events before Geppetto canonical events in the pipeline;
  - source evidence for OpenAI Responses, Chat Completions, Claude, Gemini, and correlation builders;
  - `I0. Provider-native normalization boundary`;
  - a Geppetto provider-normalization conformance design section;
  - provider fixture matrix and provider-level assertions;
  - implementation Phase 1 for Geppetto provider-normalization matrices.
- Updated `design-doc/02-static-analysis-for-protocol-conformance.md` to add:
  - Layer 0 provider-specific event normalization;
  - provider adapter route-table extraction;
  - provider-specific graph colors and forbidden edges;
  - static checks for OpenAI Responses, Chat Completions, Claude, and Gemini;
  - provider adapter route table as the first Go analyzer target.
- Updated `design-doc/03-finite-state-model-for-protocol-conformance.md` to add:
  - provider-normalization model before downstream Pinocchio lifecycles;
  - provider-native model events;
  - provider-normalization invariants `PN1` through `PN7`;
  - provider-normalization production execution examples;
  - Phase 1 deterministic provider-normalization model.
- Updated `README.md`, `index.md`, `tasks.md`, and `changelog.md` to reflect all three guides and provider-normalization scope.

### Why

The provider adapter layer is where external provider semantics enter the system. If an adapter emits the wrong canonical Geppetto event, Pinocchio can process that event correctly and still produce wrong behavior. The documentation therefore needs to make provider-native normalization the first conformance layer, not an implied upstream concern.

### What worked

- The three design documents now consistently describe the protocol as provider-to-browser instead of only Geppetto-to-browser.
- Phase 1 is now provider-normalization testing in Geppetto provider packages.
- The static-analysis and finite-state-model guides both explain what can and cannot be proven at the provider adapter layer.

### What didn't work

- The first version of the documentation under-scoped the problem by starting at canonical Geppetto events. That was corrected in this step.

### What I learned

- The provider adapter layer is the right place to catch several past defect classes:
  - provider terminal events treated as text terminal events;
  - duplicate or manufactured text finals;
  - streamed tool arguments not accumulating;
  - provider IDs/indexes not preserved in typed correlation;
  - reasoning deltas routed as assistant text.

### What was tricky to build

The tricky part is that the provider layer is both provider-specific and semantically shared. Each provider needs fixtures in its own native vocabulary, but the assertions should converge on shared canonical invariants: provider terminals are not transcript terminals, text finish requires text existence, reasoning stays separate, and tool arguments accumulate.

### What warrants a second pair of eyes

- Confirm that the proposed provider fixture matrix covers the highest-risk stream shapes for each provider.
- Confirm whether the provider-normalization model should live in Geppetto test packages only or be shared as a small helper package.

### What should be done in the future

- Add provider-native fixtures for OpenAI Responses, Chat Completions, Claude, and Gemini before downstream Pinocchio matrix work.
- Include provider fixtures in trace replay once the format is stable.

### Code review instructions

- Start with these sections:
  - `01-chat-protocol-conformance-analysis-and-implementation-guide.md`: `I0. Provider-native normalization boundary` and `Geppetto provider-normalization conformance design`.
  - `02-static-analysis-for-protocol-conformance.md`: `Layer 0: provider-specific event normalization` and `Go analyzer rule 3: provider adapter route table`.
  - `03-finite-state-model-for-protocol-conformance.md`: `Provider-normalization model`.
- Validate docs with:
  - `docmgr doctor --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE --stale-after 30`

### Technical details

Important provider adapter files referenced by the updated docs:

```text
geppetto/pkg/steps/ai/openai_responses/streaming.go
geppetto/pkg/steps/ai/openai_responses/nonstreaming.go
geppetto/pkg/steps/ai/openai/engine_openai.go
geppetto/pkg/steps/ai/openai/chat_stream.go
geppetto/pkg/steps/ai/claude/content-block-merger.go
geppetto/pkg/steps/ai/gemini/engine_gemini.go
geppetto/pkg/events/correlation_builders.go
```

## Step 6: Upload the provider-normalization guide bundle without overwriting

I uploaded a new bundle after expanding all three guides. I used a different bundle name so the earlier reMarkable PDF remains untouched.

### Prompt Context

**User prompt (verbatim):** (same as Step 5 follow-up)

**Assistant interpretation:** Publish the updated three-guide bundle, but do not overwrite the existing uploaded document.

**Inferred user intent:** Preserve prior uploaded artifacts while making the expanded provider-layer documentation available on reMarkable.

**Commit (code):** N/A — documentation/upload update only.

### What I did

- Ran a dry-run upload without `--layout` and without `--force`.
- Uploaded a new bundle named `PINO PROTOCOL CONFORMANCE provider normalization guides`.
- Included:
  - `01-chat-protocol-conformance-analysis-and-implementation-guide.md`
  - `02-static-analysis-for-protocol-conformance.md`
  - `03-finite-state-model-for-protocol-conformance.md`
  - diary, tasks, and changelog.

### Why

The user explicitly asked not to overwrite after reuploading. A new document name avoids replacing the previous PDF.

### What worked

- Dry-run reported `layout=default`.
- Upload succeeded:
  - `OK: uploaded PINO_PROTOCOL_CONFORMANCE_provider_normalization_guides.pdf -> /ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE`

### What didn't work

N/A.

### What I learned

Use a distinct bundle name when a user wants a new reMarkable upload without overwriting an existing PDF.

### What was tricky to build

The only important detail was avoiding both `--force` and accidental reuse of the previous bundle name.

### What warrants a second pair of eyes

- Confirm whether future uploads should use incrementing suffixes or descriptive names like this one.

### What should be done in the future

- Keep using default layout and non-overwriting names unless explicitly instructed otherwise.

### Code review instructions

- Confirm the new upload path:
  - `/ai/2026/05/08/PINO-PROTOCOL-CONFORMANCE/PINO_PROTOCOL_CONFORMANCE_provider_normalization_guides.pdf`

### Technical details

Upload command intentionally omitted `--force` and `--layout`.

## Step 8: Narrow implementation focus to an OpenAI Chat Completions reducer

The user chose a practical refactor path over static-analysis or model-checking implementation. The target is `geppetto/pkg/steps/ai/openai/engine_openai.go`, whose current stream loop interleaves provider I/O, mutable protocol state, correlation construction, canonical event emission, observability, and final turn construction.

### Prompt Context

**User prompt (verbatim):** "ok, let's add a design document for this refacator and store it in the ticket, then add tasks to the ticket, and go through them one by one, committing at appropriate intervals. Use table driven tests. Go for code clarity and elegance, think peter norvig style."

**Assistant interpretation:** Create a ticket design document for a reducer-style refactor, update ticket tasks, then start implementation in small committed checkpoints using table-driven tests.

**Inferred user intent:** Replace whack-a-mole protocol fixes with clear, ordinary code: explicit state, explicit inputs, explicit outputs, and examples-as-tests.

**Commit (code):** Pending at the time this diary entry was written.

### What I did

- Added `design-doc/04-openai-chat-stream-reducer-refactor.md`.
- Updated `tasks.md` to make reducer refactoring and table-driven tests the immediate implementation path.
- Updated `README.md` and `index.md` so the new reducer design is the current implementation starting point.
- Explicitly marked static-analysis and model-checking implementation as out of scope for this ticket.

### Why

The prior conformance plan identified provider normalization as the first layer to test, but the OpenAI Chat Completions adapter is currently too complex to test cleanly. A reducer gives us a small executable protocol core:

```text
state + stream input -> next state + effects
```

That lets tests describe behavior without network calls, provider SDK mocks, or browser runs.

### What worked

The reducer design naturally absorbed the important lifecycle rules:

- EOF, cancel, and error all close active text/reasoning segments.
- Terminal events do not manufacture segments.
- Cancel/error do not emit executable `ToolCallRequested` events.
- Tool argument deltas preserve both the current fragment and accumulated arguments.
- Correlation construction has one home on the reducer state.

### What didn't work

No implementation was attempted in this step. One unrelated Geppetto working-tree deletion was already present before starting this step:

```text
D pkg/steps/ai/openai/transcribe.go
```

Do not accidentally include that deletion in reducer commits unless it is intentionally resolved first.

### What I learned

The cleanest first implementation checkpoint is not a full engine rewrite. It is:

1. add reducer types and helpers;
2. add table-driven tests for reducer behavior;
3. then wire `engine_openai.go` to use the reducer.

This keeps the review small and makes the tests drive the refactor.

### What was tricky to build

The tricky semantic point is cancellation: it should close open segments like EOF, but it should not mean success. It should emit interruption semantics and avoid requesting partial tool calls.

### What warrants a second pair of eyes

- Confirm the terminal event ordering: child segment finishes, then interrupt/error if any, then provider-call finished.
- Confirm whether error/cancel segment finish reasons should be exactly `error` and `cancelled` or should preserve provider stop reason when available.

### What should be done in the future

- Implement `chat_stream_reducer.go` and `chat_stream_reducer_test.go`.
- Keep the reducer package-private until a wider provider-normalization test harness needs it.
- Commit the documentation checkpoint before code changes.

### Code review instructions

Review:

- `design-doc/04-openai-chat-stream-reducer-refactor.md`
- `tasks.md`
- `README.md`
- `index.md`

Then validate the ticket with:

```bash
docmgr doctor --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE --stale-after 30
```

### Technical details

Immediate reducer implementation target:

```text
/home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/geppetto/pkg/steps/ai/openai/engine_openai.go
```

## Step 9: Implement OpenAI Chat Completions reducer and wire the engine

I implemented the reducer refactor described in Step 8 and kept the implementation intentionally ordinary: small state, explicit inputs, explicit terminal kinds, effects, and table-driven tests.

### Prompt Context

**User prompt (verbatim):** "ok, let's add a design document for this refacator and store it in the ticket, then add tasks to the ticket, and go through them one by one, committing at appropriate intervals. Use table driven tests. Go for code clarity and elegance, think peter norvig style."

**Assistant interpretation:** After the documentation checkpoint, implement the reducer in incremental commits and validate each checkpoint.

**Inferred user intent:** Make `engine_openai.go` easier to reason about by moving protocol state transitions out of the provider stream loop.

**Commits (code):**

- Geppetto `4262075 Add OpenAI chat stream reducer tests`
- Geppetto `12d58dc Wire OpenAI chat stream reducer`

### What I did

- Added `geppetto/pkg/steps/ai/openai/chat_stream_reducer.go`.
- Added `geppetto/pkg/steps/ai/openai/chat_stream_reducer_test.go`.
- Moved Chat Completions stream state into `openAIChatStreamState`.
- Added reducer inputs for chunks and terminal events.
- Added terminal kinds for EOF, cancellation, and stream error.
- Added reducer effects for canonical events and observability callbacks.
- Added table-driven reducer tests for:
  - text delta then EOF;
  - EOF with no content;
  - cancellation after active text;
  - error after active reasoning;
  - tool argument accumulation then EOF;
  - cancellation after partial tool arguments.
- Wired `engine_openai.go` to call the reducer and apply effects.
- Kept stream I/O, final turn-block construction, and inference-result persistence in the engine.

### Why

The previous stream loop mixed provider I/O with lifecycle state. The reducer makes the lifecycle rules local and testable. It also makes cancellation/error cleanup explicit: active text/reasoning segments are closed, but partial tool calls are not promoted into executable `ToolCallRequested` events.

### What worked

- `go test ./pkg/steps/ai/openai -run TestReduceOpenAIChatStream -count=1` passed after adding the reducer tests.
- The first commit attempt caught an `exhaustive` lint issue in the terminal switch; adding the explicit EOF no-op case fixed it.
- Geppetto pre-commit ran full `go test ./...` and lint successfully for both code commits.

### What didn't work

- There was an unrelated pre-existing working-tree deletion of `pkg/steps/ai/openai/transcribe.go` before this implementation step.
- The first reducer commit accidentally included that deletion because it was already staged/visible in the working tree.
- I immediately restored `transcribe.go` from `HEAD^` and amended the commit, so the final reducer commit contains only the new reducer files.

### What I learned

- Avoid storing `strings.Builder` directly in a value-returning reducer state. Copying a non-zero builder is unsafe. The implementation uses plain strings for `Message` and `Reasoning`, which is simpler and safer for a small reducer.
- The reducer pattern made the cancellation semantics straightforward: terminal cleanup is one code path, while classification differs by terminal kind.

### What was tricky to build

The main tricky detail was preserving observability without mixing it back into protocol logic. The reducer returns effects; `engine_openai.go` applies those effects by calling `observeProviderEvent`, `observeProviderNormalizeDelta`, and `publishEvent`.

### What warrants a second pair of eyes

- Confirm event order on cancellation/error: finish child segments, emit interrupt/error, then finish provider call.
- Confirm that provider-call finished events on stream error/cancel are desired across all downstream consumers. This is more complete lifecycle behavior than the old loop, which returned after publishing only interrupt/error.

### What should be done in the future

- Extend provider-normalization table tests to OpenAI Responses, Claude, and Gemini.
- Consider adding a small fake stream integration test for `RunInference` once the reducer behavior is accepted.
- Use the reducer pattern as a template for any future provider adapter cleanup only where it reduces complexity.

### Code review instructions

Start with the tests:

```text
geppetto/pkg/steps/ai/openai/chat_stream_reducer_test.go
```

Then review the reducer:

```text
geppetto/pkg/steps/ai/openai/chat_stream_reducer.go
```

Finally review the engine diff:

```text
geppetto/pkg/steps/ai/openai/engine_openai.go
```

Validation already run by Geppetto pre-commit:

```bash
go test ./...
make lintmax
```

Targeted validation also run:

```bash
go test ./pkg/steps/ai/openai -run TestReduceOpenAIChatStream -count=1
```

## Step 10: Share terminal completion for EOF, cancel, and error

After wiring the reducer, the user pointed out that the cancellation path should do at least part of `streamingComplete`, and that the main function should show the overall principle more clearly through smaller named functions.

### Prompt Context

**User prompt (verbatim):** "continue. also, i think the cancel path should probably at lest do part of streamingComplete. Maybe extract some smaller parts of the main function into smaller named functions, so that the overall principle is nicely visible"

**Assistant interpretation:** Keep the reducer, but make terminal handling symmetric: EOF, cancel, and error should all run a shared completion path where appropriate. Extract named helpers so `RunInference` reads as setup, consume stream, complete stream.

**Inferred user intent:** Preserve partial work on cancellation/error and make the code teach the protocol instead of hiding it in branches.

**Commit (code):** Geppetto `ec6be03 Finalize OpenAI chat terminal streams`

### What I did

- Extracted `consumeOpenAIChatStream`, which owns only stream reading and chunk-effect application.
- Extracted `completeOpenAIChatStream`, which finalizes metadata, reduces the terminal input, applies terminal effects, appends turn blocks, and persists inference metadata.
- Extracted `appendOpenAIChatTurnBlocks`, so transcript construction is named and testable.
- Added terminal stop-reason helpers so cancellation records `cancelled` and stream errors record `error` in final metadata.
- Changed cancel/error paths to return the partially updated turn together with the error instead of returning `nil, err` after publishing only interrupt/error events.
- Kept tool-call block appending limited to EOF; cancel/error preserve partial text/reasoning but do not create executable tool requests or tool-call blocks.
- Added table-driven tests for transcript block appending and terminal stop-reason behavior.

### Why

Cancellation and stream errors are terminal states of the same stream lifecycle. They should close active segments and preserve useful partial transcript state, but they should not pretend the provider completed successfully or request partial tools. The shared completion helper makes that rule visible.

### What worked

- `go test ./pkg/steps/ai/openai -count=1` passed.
- Geppetto pre-commit passed full `go test ./...` and lint.
- The top-level `RunInference` now reads as:
  1. build request and start provider call;
  2. consume stream chunks;
  3. complete stream terminal;
  4. return the turn plus terminal error, if any.

### What didn't work

No command failures occurred in this step.

### What I learned

The reducer by itself was useful, but the main function still hid policy in branches. The helper extraction made the policy legible: all terminal kinds complete the stream, while only EOF includes tool-call requests/blocks.

### What was tricky to build

The subtle part was deciding what "part of streamingComplete" means for cancellation. The chosen behavior is:

- preserve reasoning and text blocks;
- persist final metadata with stop reason `cancelled`;
- publish interrupt and provider-call finished events;
- do not append tool-call blocks;
- return the partial turn with `ctx.Err()`.

### What warrants a second pair of eyes

- Confirm downstream callers are comfortable receiving a non-nil turn with a non-nil error on cancellation/error.
- Confirm `error` as the persisted stop reason is the right canonical value for stream failures.

### What should be done in the future

- Consider whether Claude should get the same shared terminal completion treatment.
- Add an integration-style fake stream test if future regressions happen around `RunInference` return values.

### Code review instructions

Review these helpers in `geppetto/pkg/steps/ai/openai/engine_openai.go`:

- `consumeOpenAIChatStream`
- `completeOpenAIChatStream`
- `appendOpenAIChatTurnBlocks`
- `withTerminalStopReason`

Review the new table-driven tests in `geppetto/pkg/steps/ai/openai/chat_stream_reducer_test.go`:

- `TestAppendOpenAIChatTurnBlocks`
- `TestOpenAIChatTerminalStopReason`

Validation run by pre-commit:

```bash
go test ./...
make lintmax
```

## Step 11: Design the OpenAI Responses stream refactor

The user asked to pause broad provider-normalization tests and first create a design document for refactoring the OpenAI Responses engine to follow the same structure as the newly refactored Chat Completions engine.

### Prompt Context

**User prompt (verbatim):** "Let's wait with the normalization tests, and create a document for refactoring the responses engine in the same pattern, to be as close as possible to the structure of completion, to adaopt a pattern so to say. Then add detailed tasks to the ticket and implement and commit as you go and keep a diary"

**Assistant interpretation:** Do not start the next conformance matrix tests yet. First document and implement a Responses structural refactor so provider adapters converge on one visible consume/complete/state pattern.

**Inferred user intent:** Establish a reusable provider-adapter shape before multiplying tests, so the code itself becomes easier to review and teach.

**Commit (code):** Pending at the time this diary entry was written.

### What I did

- Added `design-doc/05-openai-responses-stream-refactor.md`.
- Updated `README.md`, `index.md`, `tasks.md`, and `changelog.md` to make the Responses refactor the next implementation focus.
- Explicitly paused broad provider-normalization matrix tests until after this structural refactor.

### Why

OpenAI Responses streaming is significantly more complex than Chat Completions. It handles provider-native output items, reasoning items, reasoning summaries, function-call items, citations, web-search lifecycle events, and final turn persistence. Before adding broader conformance tests, the code should adopt the same visible structure:

```text
setup request
initialize stream state
consume stream
complete terminal state
append/persist final turn data
return turn + terminal error if any
```

### What worked

The design doc maps the existing complexity into implementation tasks without pretending that Responses can immediately be a tiny pure reducer. It proposes an incremental structural refactor: explicit state, state-owned correlation helpers, consume helper, provider-object handler, and shared completion helper.

### What didn't work

No implementation was attempted in this step.

### What I learned

The Responses engine should probably converge on the Chat Completions pattern in stages. The first goal is not a perfect reducer; it is making terminal behavior and stream state explicit enough that future smaller reducers can emerge naturally.

### What was tricky to build

The tricky semantic distinction is reasoning persistence. Responses reasoning blocks are usually appended when `response.output_item.done` arrives because that event carries item metadata and encrypted content. The shared completion helper should preserve already-appended reasoning and final metadata, but it should not manufacture rich reasoning blocks without provider item metadata.

### What warrants a second pair of eyes

- Confirm how much partial reasoning should be appended on cancel/error if a reasoning item started but never reached `output_item.done`.
- Confirm whether Responses cancellation should return a non-nil partial turn with `ctx.Err()`, matching the Chat Completions refactor.

### What should be done in the future

- Implement the Responses stream state and terminal helpers.
- Extract completion helpers before extracting the full consume loop.
- Use existing Responses package tests as the main behavior guard.
- Add only small table-driven helper tests for new helper behavior, not broad normalization matrices yet.

### Code review instructions

Review:

```text
pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/design-doc/05-openai-responses-stream-refactor.md
```

Then implement against:

```text
geppetto/pkg/steps/ai/openai_responses/streaming.go
```

## Step 12: Start Responses implementation and remove the non-streaming path

The user asked whether killing `runNonStreamingInference` would simplify the Responses refactor. I agreed and removed it, because maintaining two runtime paths would work against the desired provider-adapter pattern.

### Prompt Context

**User prompt (verbatim):** "kill nonstreaminginference, does that simplify things?"

**Assistant interpretation:** Remove the Responses non-streaming branch if that makes the consume/complete/state pattern cleaner and safer.

**Inferred user intent:** Avoid preserving a second provider-normalization path that duplicates final metadata, turn-block persistence, usage handling, and provider-call finishing.

**Commits (code):**

- Geppetto `fe6423d Share Responses stream completion state`
- Geppetto `db0c69b Remove Responses nonstreaming path`

### What I did

- Added `geppetto/pkg/steps/ai/openai_responses/stream_state.go` with:
  - `responsesStreamTerminalKind` and `responsesStreamTerminal`;
  - `responsesStreamState`;
  - state-owned provider-call/segment/tool correlation helpers;
  - final metadata helper;
  - final turn-block append helper;
  - finish-class and inference-result persistence helpers.
- Updated `streaming.go` to use the new state helpers for final completion.
- Extracted the SSE read loop into `consumeResponsesSSE`.
- Updated stream-error behavior to publish a failed provider-call finish event, matching the canonical lifecycle direction already used in Chat Completions.
- Removed `geppetto/pkg/steps/ai/openai_responses/nonstreaming.go`.
- Updated `Engine.RunInference` so Responses always uses `runStreamingInference`.
- Updated the previous non-streaming usage test so it verifies cached/reasoning usage through an SSE response instead.
- Removed an unused non-streaming-only helper after lint identified it.

### Why

Removing non-streaming simplifies the refactor substantially:

- one provider-call lifecycle path;
- one final metadata path;
- one turn-block persistence path;
- one place to fix cancellation/error behavior;
- no need to keep non-streaming and streaming event semantics aligned by hand.

The Responses API may still receive profiles with `Chat.Stream=false`, but this engine now ignores that for runtime path selection. It always requests/consumes the provider through the streaming code path.

### What worked

- `go test ./pkg/steps/ai/openai_responses -count=1` passed after the refactor.
- Full Geppetto pre-commit passed for `db0c69b`, including `go test ./...` and lint.
- Lint caught dead code (`reasoningTextFromOutputContent`) left behind by deleting the non-streaming path, which was removed.

### What didn't work

- The first commit attempt failed because the new state methods were not all referenced yet. I wired the existing inline correlation closures through the state methods so lint could verify the helper extraction is real, not aspirational.
- The first updated test incorrectly expected a JSON `stream:true` field in the Responses request. The request schema does not currently expose such a field; streaming is selected by engine runtime path and SSE `Accept` header. I removed that assertion and kept the usage coverage through an SSE fixture.

### What I learned

The non-streaming path had hidden cleanup cost: it carried separate usage parsing, reasoning block persistence, tool block persistence, and final event logic. Removing it makes the remaining Responses refactor much easier to reason about.

### What was tricky to build

The tricky part was preserving existing behavior while introducing state helpers. The current implementation still has a large provider-event switch, but final completion and correlation now have clearer homes.

### What warrants a second pair of eyes

- Confirm that ignoring `Chat.Stream=false` for Responses is acceptable. This now matches the Chat Completions direction: the engine uses streaming internally so observers and canonical lifecycle events stay consistent.
- Confirm failed provider-call finish events on Responses stream errors are desired downstream.

### What should be done in the future

- Continue extracting `runStreamingInference` so its top-level shape becomes as clear as Chat Completions.
- Move more provider-event switch branches into named handlers only where doing so improves readability.
- Add small table-driven helper tests for the new state/completion helpers if review wants direct coverage beyond package integration tests.

### Code review instructions

Review these files:

```text
geppetto/pkg/steps/ai/openai_responses/stream_state.go
geppetto/pkg/steps/ai/openai_responses/streaming.go
geppetto/pkg/steps/ai/openai_responses/engine.go
geppetto/pkg/steps/ai/openai_responses/engine_test.go
```

Validation run:

```bash
go test ./pkg/steps/ai/openai_responses -count=1
go test ./...
make lintmax
```

## Step 13: Continue Responses extraction in small safe commits

I continued the Responses structural refactor after removing the non-streaming path. The focus was to extract obvious pieces without doing a risky whole-file rewrite.

### Prompt Context

**User prompt (verbatim):** "continue."

**Assistant interpretation:** Continue implementing the Responses refactor in committed checkpoints, keeping the diary current.

**Inferred user intent:** Keep converging Responses toward the Chat Completions pattern, but preserve behavior and avoid a giant unsafe rewrite.

**Commits (code):**

- Geppetto `2735014 Extract Responses stream opening`
- Geppetto `b56187c Extract Responses stream completion helper`
- Geppetto `a07ebac Extract Responses stream helper functions`

### What I did

- Extracted `openResponsesStream` from `runStreamingInference`:
  - request construction;
  - SSE headers;
  - debug tap HTTP capture;
  - HTTP status handling.
- Extracted `newResponsesProviderCallCorrelation`.
- Extracted `completeResponsesStream`, so terminal metadata, summary emission, final turn-block appending, inference-result persistence, and provider-call finish are in one named completion helper.
- Extracted small provider stream helpers:
  - `missingProviderSuffix`;
  - `responsesChunkFromValue`.
- Added table-driven helper tests in `stream_helpers_test.go`.

### Why

These extractions make the main Responses function more readable without changing the large provider-event switch yet. They also continue the same shape used by Chat Completions:

```text
open stream
consume stream
complete terminal state
```

### What worked

- `go test ./pkg/steps/ai/openai_responses -count=1` passed after each checkpoint.
- Full Geppetto pre-commit passed for each commit.
- The helper tests make the provider backfill logic less magical.

### What didn't work

I briefly tried to move all mutable locals directly into `responsesStreamState` in one broad rewrite. That produced noisy and unsafe replacements, including replacements inside string keys and log field names. I reverted that uncommitted attempt and continued with smaller named extractions instead.

### What I learned

For this file, a Peter-Norvig-style refactor means small obvious transformations, not a heroic conversion. The large provider-event switch should be carved up by semantic cases, one at a time.

### What was tricky to build

The overlap/backfill logic is small but subtle: provider terminal payloads may repeat already-streamed text or contain only the missing suffix. Extracting `missingProviderSuffix` made that behavior testable.

### What warrants a second pair of eyes

- Decide which provider-event switch cases should be extracted next. Good candidates are message text handling, reasoning summary handling, and function-call handling.
- Confirm whether the large switch should remain in `streaming.go` with named helpers, or move into a `stream_handlers.go` file.

### What should be done in the future

- Extract message/text provider-event handling into named helpers.
- Extract function-call argument handling into named helpers.
- Move state gradually into `responsesStreamState`, preferably as part of those handler extractions.

### Code review instructions

Review:

```text
geppetto/pkg/steps/ai/openai_responses/streaming.go
geppetto/pkg/steps/ai/openai_responses/stream_helpers_test.go
```

Validation run by pre-commit:

```bash
go test ./...
make lintmax
```

## Step 14: Move Responses mutable stream data into explicit state

I continued the Responses refactor by moving more of the provider stream's mutable data into `responsesStreamState`. This was deliberately split into small commits so each checkpoint preserved behavior and kept the large provider-event switch readable enough to review.

The main result is that the completion helper now receives the same state object that was initialized before consuming the stream, instead of a mostly reconstructed state assembled from many local variables at the end. This makes the Responses path closer to the Chat Completions consume/complete shape.

### Prompt Context

**User prompt (verbatim):** "Continue, keep a diary, and commit at appropriate intervals (read docmgr and diary skill)"

**Assistant interpretation:** Continue the OpenAI Responses structural refactor, explicitly follow the docmgr/diary workflows, record the implementation journey, validate, and commit focused checkpoints.

**Inferred user intent:** Make the Responses implementation easier to reason about without losing the traceability needed for review and later continuation.

**Commit (code):** `acd7812` — "Move Responses assistant stream state into reducer state"

**Commit (code):** `6ed2113` — "Keep Responses response id in stream state"

**Commit (code):** `c9bebc8` — "Keep Responses tool stream state in reducer state"

**Commit (code):** `f1ddf3b` — "Keep Responses terminal stream state in reducer state"

### What I did

- Read the `docmgr` and `diary` skills before continuing.
- Moved assistant text/message state into `responsesStreamState`:
  - accumulated message text;
  - per-item assistant text map;
  - `sayBuf`;
  - latest message item/output/status.
- Kept `currentResponseID` directly in `responsesStreamState`, removing the extra local response-id variable and the synchronization closure around correlation helpers.
- Moved function-call accumulation into `responsesStreamState`:
  - `callsByItem`;
  - `finalCalls`.
- Moved terminal envelope state into `responsesStreamState`:
  - usage totals;
  - stop reason;
  - `responseCompleted`;
  - stream/provider error.
- Ran targeted package tests after each migration:

```bash
go test ./pkg/steps/ai/openai_responses -count=1
```

- Let the Geppetto pre-commit hook run full validation for each code commit:

```bash
go test ./...
make lintmax
```

### Why

The previous version created `responsesStreamState` early, but still tracked many important values in locals and copied them into a new state at the end. That made the function look less like an explicit reducer/state-machine path than it really was.

Moving state gradually lets the code converge on a single source of truth while avoiding the failed broad rewrite pattern from Step 13.

### What worked

- The assistant state migration removed several locals and the final reconstructed-state copy for assistant text data.
- Response ID correlation is now simpler: state correlation helpers read the current response id directly.
- Tool-call accumulation now lives in the same state object used by final turn-block appending.
- Terminal usage/stop/error state now flows directly into `finalizeResponsesStreamMetadata` through `responsesStreamState`.
- Package tests and full pre-commit validation passed for each code commit.

### What didn't work

- A scripted replacement for `currentResponseID` briefly produced invalid code:

```text
streamState.streamState.currentResponseID
```

- This was caught immediately by:

```bash
go test ./pkg/steps/ai/openai_responses -count=1
```

- The exact build error was:

```text
pkg/steps/ai/openai_responses/streaming.go:673:14: streamState.streamState undefined (type *responsesStreamState has no field or method streamState)
```

- I fixed the bad replacement before committing and kept the successful response-id state migration as a focused commit.
- I also attempted to use `docmgr doc relate` with absolute and root-relative document paths after the docs edits. The relation lookup did not resolve the target document even though `docmgr doc list --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE` listed it. The attempted command failed with:

```text
Error: expected exactly 1 doc for --doc "2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/design-doc/05-openai-responses-stream-refactor.md", got 0
```

### What I learned

Identifier-level rewrites in this file are still risky because the function has nested closures, state helpers, and final synchronization code. Small semantic migrations are safer than broad replacement, and the targeted package test gives fast feedback before the heavier pre-commit hook.

### What was tricky to build

The tricky part is preserving correlation timing while moving response id into state. `segmentCorrelation` and `toolCorrelation` must see the latest provider response id, but they should not rely on a shadow local being copied into state at call time. The migration worked by making the SSE flush update `streamState.currentResponseID` as soon as the provider object exposes it, then letting correlation helpers read that field directly.

For tool calls, the sharp edge is preserving partial argument accumulation without materializing partial calls on cancel/error. Moving `callsByItem` into state is safe because final executable `ToolCallRequested` and final turn blocks still only come from the existing completion/event conditions.

### What warrants a second pair of eyes

- Review `response.function_call_arguments.delta` and `response.output_item.done` to ensure the state migration did not subtly change argument accumulation order.
- Review response-id propagation in `response.output_item.added`, reasoning summary events, reasoning text deltas, and final block metadata.
- Confirm that terminal errors still publish failed provider-call finish events and return partial turns as before.

### What should be done in the future

- Move remaining reasoning scratch state into `responsesStreamState` carefully.
- Extract semantic handlers for the large provider-event switch after state ownership is clear.
- Add focused tests if a future handler extraction changes reasoning or tool-call behavior.

### Code review instructions

Start with:

```text
geppetto/pkg/steps/ai/openai_responses/streaming.go
geppetto/pkg/steps/ai/openai_responses/stream_state.go
```

Review the four commits in order:

```bash
git -C geppetto show acd7812
git -C geppetto show 6ed2113
git -C geppetto show c9bebc8
git -C geppetto show f1ddf3b
```

Validate with:

```bash
cd geppetto
go test ./pkg/steps/ai/openai_responses -count=1
go test ./...
make lintmax
```

### Technical details

The current Responses shape now has one state object initialized before stream consumption and used by terminal completion. Remaining local mutable state is mostly reasoning-specific scratch data that should be migrated or extracted next.

## Step 15: Finish Responses state ownership and pull provider events out of the runner

I finished the remaining state-ownership pass for OpenAI Responses and then separated provider-event handling from the top-level stream runner. The runner now reads like the intended pattern: open the HTTP/SSE stream, initialize `responsesStreamState`, consume SSE frames, complete terminal state, and return the turn plus terminal error.

This step does not attempt to reinterpret provider semantics. It is a structural cleanup: the same mutable state is now owned by the explicit state object throughout the stream, and the large provider-native event switch no longer obscures the high-level flow in `runStreamingInference`.

### Prompt Context

**User prompt (verbatim):** "go ahead, do them all"

**Assistant interpretation:** Complete the remaining Responses refactor items: move the rest of the mutable scratch state into `responsesStreamState`, extract provider-event handling out of the runner, keep diary/docs current, validate, and commit at safe checkpoints.

**Inferred user intent:** Finish the Responses structural refactor rather than stopping at partial state migration.

**Commit (code):** `78990d0` — "Keep Responses reasoning stream state in reducer state"

**Commit (code):** `f67e02d` — "Extract Responses provider event handler"

**Commit (code):** `5bfa040` — "Move Responses provider event handling to stream events"

### What I did

- Removed the remaining reasoning scratch locals from `runStreamingInference` and used `responsesStreamState` directly for:
  - `thinkBuf`;
  - `summaryBuf`;
  - current reasoning text/summary builders;
  - current/last reasoning item ids;
  - current/last reasoning output and summary indexes;
  - current reasoning status;
  - encrypted reasoning content.
- Removed the final synchronization block that copied reasoning locals back into state.
- Extracted provider-native event handling from `runStreamingInference` into `handleResponsesProviderEvent`.
- Moved the extracted handler into `stream_events.go`, next to `normalizeResponsesEventName` and `toInt`.
- Preserved the top-level stream lifecycle shape in `streaming.go`:

```text
openResponsesStream
newResponsesStreamState
consumeResponsesSSE
completeResponsesStream
```

### Why

The purpose of this refactor is to make the Responses stream code readable in the same way as the Chat Completions path. Keeping all mutable stream data in one explicit state object removes the mental model where some data lives in locals and some data lives in the state object.

Moving provider-event handling out of `runStreamingInference` makes the runner small enough to audit for lifecycle behavior: provider-call start, consume, terminal error normalization, completion, and return.

### What worked

- The package tests passed after migrating reasoning state:

```bash
go test ./pkg/steps/ai/openai_responses -count=1
```

- Full Geppetto pre-commit validation passed for all three code commits:

```bash
go test ./...
make lintmax
```

- `streaming.go` is now focused on setup/consume/complete rather than provider-specific event details.
- `stream_events.go` now contains provider-native event normalization details.

### What didn't work

- The first mechanical reasoning-state replacement created temporary `streamState.streamState...` expressions. Package tests caught the problem before commit. I removed the accidental self-assignments and reran:

```bash
go test ./pkg/steps/ai/openai_responses -count=1
```

- While moving the provider-event handler into `stream_events.go`, I initially overwrote the small existing helper file and temporarily lost `normalizeResponsesEventName` and `toInt`. The package test failed with errors including:

```text
pkg/steps/ai/openai_responses/streaming.go:50:24: undefined: normalizeResponsesEventName
pkg/steps/ai/openai_responses/token_count.go:135:14: undefined: toInt
```

- I restored those helpers at the top of `stream_events.go`, reran the package tests, and only then committed.

### What I learned

The Responses code is now structurally much closer to the Chat Completions pattern, but the provider event handler remains semantically dense. The right next move, if review asks for more clarity, is not another broad rewrite; it is small semantic extraction inside `stream_events.go` for groups such as reasoning events, message events, tool-call events, and terminal/error events.

### What was tricky to build

The tricky part was keeping provider-native semantics stable while changing ownership and file boundaries. Reasoning deltas use both provider text and normalized display text, summary deltas update both per-item and aggregate summary buffers, and reasoning finalization appends transcript blocks while also emitting canonical segment-finished events. Moving that state into `responsesStreamState` had to preserve all of those relationships.

The file move had a second sharp edge: `stream_events.go` already existed for shared event/number helpers. Replacing it with the extracted handler accidentally removed helpers used by both streaming and usage parsing. The fix was to restore `normalizeResponsesEventName` and `toInt` before the new handler.

### What warrants a second pair of eyes

- Review the reasoning-event paths in `stream_events.go`, especially summary delta accumulation and reasoning block finalization.
- Review that `runStreamingInference` now has the desired high-level lifecycle shape and still emits provider-call start/finish correctly.
- Decide whether `handleResponsesProviderEvent` should be split further by semantic groups now that it is isolated from the runner.

### What should be done in the future

- If review finds the provider handler too large, split it into smaller handlers:
  - output item added/done;
  - reasoning summary/text;
  - assistant output text/json;
  - function-call arguments;
  - response terminal/error.
- Resume Phase 1 provider-normalization conformance tests after this Responses structural checkpoint is accepted.

### Code review instructions

Start with:

```text
geppetto/pkg/steps/ai/openai_responses/streaming.go
geppetto/pkg/steps/ai/openai_responses/stream_events.go
geppetto/pkg/steps/ai/openai_responses/stream_state.go
```

Review commits:

```bash
git -C geppetto show 78990d0
git -C geppetto show f67e02d
git -C geppetto show 5bfa040
```

Validate with:

```bash
cd geppetto
go test ./pkg/steps/ai/openai_responses -count=1
go test ./...
make lintmax
```

### Technical details

The top-level Responses stream runner now delegates provider-native event semantics to `handleResponsesProviderEvent`. `responsesStreamState` is the single mutable state object used for assistant output, reasoning output, tool-call accumulation, usage/stop/error state, response id, and final completion.

## Step 16: Write the provider event table-driven testing guide

I wrote a dedicated implementation reference for provider event testing so the concrete test suites can share scenario intent without forcing all providers into the same artificial input format. The document clarifies that OpenAI Chat Completions, OpenAI Responses, Claude, and Gemini should each keep provider-native fixtures, while their expected outputs are projected into the same canonical lifecycle vocabulary.

This gives the next implementation phase a single checklist of provider-normalization scenarios. It also captures the current decision that Claude already has a reducer-like `ContentBlockMerger`, while Gemini still needs a stream-state/reducer seam before deep table-driven tests will be pleasant.

### Prompt Context

**User prompt (verbatim):** "create a docs/design/implementation/01-provider-event-testing.md which details how the table driven testing works, and a list of scenario that apply to each provider. That way we have one freference document from which we can derive the concrete table driven tests for the providers, but the scenarios are all similar across providers."

**Assistant interpretation:** Create a reference implementation document that explains how to build provider-specific table-driven tests from shared canonical lifecycle scenarios.

**Inferred user intent:** Avoid an over-generic cross-provider test framework while still coordinating provider tests around one common scenario matrix.

**Commit (docs):** pending at time of diary entry.

### What I did

- Created:

```text
geppetto/docs/design/implementation/01-provider-event-testing.md
```

- Documented the testing pattern:
  - same lifecycle questions;
  - provider-specific native inputs;
  - provider-specific reducer/adapter seams;
  - shared-ish canonical trace assertions.
- Added a canonical trace projection sketch for comparing emitted canonical events without brittle generated IDs.
- Listed shared invariants for provider-call, text, reasoning, tool, terminal, and correlation lifecycles.
- Added a scenario matrix covering OpenAI Chat Completions, OpenAI Responses, Claude, and Gemini.
- Added provider-specific table shapes and priority rows.
- Updated ticket links in `README.md` and `index.md`.
- Updated `tasks.md` and `changelog.md`.

### Why

The providers have very different native stream grammars, so a single shared input table would hide provider-specific edge cases. But the canonical Geppetto output protocol should obey the same lifecycle invariants regardless of provider. The new guide captures that split explicitly.

### What worked

- The document gives a concrete path from scenario names to provider-specific tests.
- The scenario matrix marks which rows are required, conditional, or not applicable for each provider.
- The guide recommends keeping helpers local at first and only extracting a shared `streamtest` package if duplication becomes obvious.

### What didn't work

N/A.

### What I learned

The important shared layer is not native input shape. It is the expected canonical lifecycle trace after provider normalization. That is where a small assertion vocabulary can be shared without flattening provider semantics.

### What was tricky to build

The tricky part was being precise about Gemini and Claude. Claude already has a reducer-like seam in `ContentBlockMerger`, so the guide should not imply a full rewrite is necessary. Gemini still has inline stream logic, so the guide needs to recommend reducer extraction before deep table tests.

### What warrants a second pair of eyes

- Review the scenario matrix for provider applicability, especially reasoning support and Gemini function-call partiality.
- Review whether the canonical trace projection fields are enough for the first tests or whether correlation details need a richer projected shape.

### What should be done in the future

- Use this guide to implement the next provider-specific table tests.
- Start with OpenAI Chat Completions and Responses as reference implementations, then Claude, then Gemini reducer extraction and tests.

### Code review instructions

Start with:

```text
geppetto/docs/design/implementation/01-provider-event-testing.md
```

Then review the index/readme links and task/changelog updates.

Validate with:

```bash
docmgr doctor --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE --stale-after 30
```

### Technical details

The guide intentionally avoids a large shared provider conformance framework. It recommends provider-native tables plus a compact projected canonical trace helper, potentially local to each provider until commonality stabilizes.

## Step 17: Move the provider testing guide into Geppetto docs

I moved the provider event testing guide from the Pinocchio ticket-local `docs/` directory into `geppetto/docs`. The guide is about Geppetto provider adapter tests, so keeping it next to the provider code makes it a better candidate for the docmgr extension experiments around repository-local design documentation.

The Pinocchio ticket still links to the guide and records the decision, but the canonical document path is now in Geppetto.

### Prompt Context

**User prompt (verbatim):** "the docs/ folder should be in ./geppetto/docs"

**Assistant interpretation:** Relocate the provider event testing guide so the `docs/design/implementation` tree lives under the Geppetto repository rather than inside the Pinocchio ticket workspace.

**Inferred user intent:** Experiment with docmgr-style documentation that can live in a code repository's own docs tree while still being linked from the ticket workspace.

**Commit (docs):** pending at time of diary entry.

### What I did

- Moved:

```text
pinocchio/ttmp/.../docs/design/implementation/01-provider-event-testing.md
```

  to:

```text
geppetto/docs/design/implementation/01-provider-event-testing.md
```

- Updated the guide's canonical path in its docmgr-style preamble.
- Rewrote relative links in the guide:
  - Geppetto source links now point to `../../../pkg/...`.
  - Pinocchio ticket links now point back into `../../../../pinocchio/ttmp/...`.
- Updated Pinocchio ticket `README.md`, `index.md`, `tasks.md`, `changelog.md`, and this diary to refer to the Geppetto docs location.

### Why

The document is an implementation reference for Geppetto provider-normalization tests. It should therefore live in the repository where those tests will be written, while the Pinocchio ticket remains the coordination and history layer.

### What worked

- The guide now lives under `geppetto/docs/design/implementation/`.
- Pinocchio ticket links point to the moved guide.
- `docmgr doctor` for the Pinocchio ticket still passes after the move.

### What didn't work

N/A.

### What I learned

This is a useful split for docmgr extension experiments: ticket docs can coordinate work across repositories, while implementation reference docs can live in the code repository that owns the eventual tests.

### What was tricky to build

The only tricky part was relative links. Moving from a deep ticket-local docs path into `geppetto/docs/design/implementation` changed both directions of navigation: source links became shorter, while links back to the ticket became cross-repository relative links.

### What warrants a second pair of eyes

- Confirm whether cross-repository relative links are the desired convention for the docmgr extension experiment.
- Confirm whether the Geppetto docs file should keep `Ticket: PINO-PROTOCOL-CONFORMANCE` frontmatter even though it lives outside the ticket root.

### What should be done in the future

- If docmgr grows support for repository-local docs linked to ticket workspaces, use this document as a test case.
- Consider adding a Geppetto docs index if more provider-testing docs are added.

### Code review instructions

Review:

```text
geppetto/docs/design/implementation/01-provider-event-testing.md
pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/README.md
pinocchio/ttmp/2026/05/08/PINO-PROTOCOL-CONFORMANCE--systematic-chat-protocol-conformance-tests-for-canonical-event-lifecycles/index.md
```

Validate with:

```bash
docmgr doctor --root /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp --ticket PINO-PROTOCOL-CONFORMANCE --stale-after 30
```

### Technical details

This move spans two Git repositories: the new guide is tracked in Geppetto, while the ticket link updates and deletion of the old copy are tracked in Pinocchio.

## Step 18: Add review-derived provider scenarios and defer downstream ones

I updated the Geppetto provider event testing guide with the review-derived scenarios that should be tested at the provider-normalization layer now. I also added a clear boundary for related PR review issues that belong to later Pinocchio runtime, projection, persistence, and frontend phases.

This keeps the provider guide honest: it covers provider-native stream normalization, while sparse protobuf/sessionstream/frontend patch merge behavior remains a downstream conformance topic.

### Prompt Context

**User prompt (verbatim):** "ok, add the ones you said to add now."

**Assistant interpretation:** Add the provider-relevant PR review-derived scenarios to the provider testing guide now, without pulling downstream Pinocchio projection/frontend scenarios into the Geppetto provider test phase.

**Inferred user intent:** Preserve the lessons from PR 146 and related review feedback in the concrete testing guide, but keep the phase boundaries clear.

**Commit (docs):** pending at time of diary entry.

### What I did

- Added a `Review-derived edge scenarios` section to:

```text
geppetto/docs/design/implementation/01-provider-event-testing.md
```

- Added provider-normalization scenarios for:
  - active text plus error/cancel;
  - active reasoning plus error/cancel;
  - partial tool arguments plus error/cancel;
  - sparse provider deltas preserving accumulated state;
  - final usage/stop without transcript content.
- Added a downstream deferral list for:
  - sparse protobuf/sessionstream/frontend patches;
  - display fallback labels not being persisted as state;
  - closed segments not being rewritten by later terminal events;
  - centralized runtime terminal handling;
  - correlation surviving projection, persistence, and frontend reduction.
- Updated the scenario matrix with:
  - `P06` metadata-only final/stop/completed event;
  - `T07` active text plus error/cancel;
  - `S01` sparse provider-native deltas preserving accumulated state.
- Updated provider-specific priority rows to include the new scenario IDs where relevant.

### Why

PR review surfaced real edge cases that should not remain oral history. Some are provider-normalization issues, while others only appear after canonical events pass through Pinocchio projections and UI patch merging. The guide now distinguishes those groups explicitly.

### What worked

- The added scenarios fit the existing matrix without changing the provider-specific fixture strategy.
- The downstream deferral section keeps the document from becoming a Pinocchio runtime/frontend testing guide.

### What didn't work

N/A.

### What I learned

The provider-side version of sparse state is about not erasing accumulated provider-native identities or arguments when later native deltas omit fields. The Pinocchio-side version is about sparse patches not erasing projected UI state. They are related, but they need different tests at different layers.

### What was tricky to build

The tricky part was naming the scenarios so they are provider-applicable without overclaiming. For example, Gemini currently receives complete function calls, so partial tool argument scenarios only apply if a future Gemini seam models partial calls.

### What warrants a second pair of eyes

- Confirm whether `S01` should be required for Claude or only applied to tool/input JSON cases.
- Confirm whether `P06` should assert a provider-call metadata update event or only final metadata on provider-call finish for providers that do not emit intermediate metadata updates.

### What should be done in the future

- Carry the deferred downstream scenarios into the Phase 2-5 Pinocchio runtime/projection/frontend test documents.
- When implementing provider tests, reference the review-derived section first for terminal and sparse-state edge cases.

### Code review instructions

Review:

```text
geppetto/docs/design/implementation/01-provider-event-testing.md
```

Focus on the `Review-derived edge scenarios` section and the new `P06`, `T07`, and `S01` matrix rows.

### Technical details

No code behavior changed. This is a documentation update that refines the Phase 1 test scenario source.

## Step 19: Implement first review-derived provider-normalization tests

I implemented the provider-side scenarios from the Geppetto provider event testing guide across the current provider adapters. The work stayed provider-specific: OpenAI Chat Completions uses its reducer inputs, OpenAI Responses uses SSE-style fixtures through the engine, Claude uses `api.StreamingEvent` with `ContentBlockMerger`, and Gemini now has a small stream reducer seam for `genai.GenerateContentResponse` chunks.

This is the first concrete Phase 1 provider-normalization test checkpoint derived from the guide. It does not implement the downstream Pinocchio sparse patch/projection/frontend scenarios; those remain later phases.

### Prompt Context

**User prompt (verbatim):** "now implement them, using the guide as guidane, committing at appropriate intervals."

**Assistant interpretation:** Implement the provider-normalization tests described in the guide, especially the review-derived scenarios just added, and commit stable checkpoints as each provider is covered.

**Inferred user intent:** Move from planning documents to concrete provider-specific table-driven tests while preserving review traceability.

**Commit (code):** `4bcf089` — "Test OpenAI chat review-derived stream scenarios"

**Commit (code):** `fab1d3c` — "Test Claude review-derived stream scenarios"

**Commit (code):** `904c77a` — "Test Responses review-derived stream scenarios"

**Commit (code):** `aeb3c38` — "Extract Gemini stream reducer tests"

**Commit (docs):** `1be5777` — "Docs: update Gemini testing seam"

**Commit (code):** `e57d532` — "Extract Gemini stream completion helpers"

**Commit (docs):** `59df153` — "Docs: record Gemini completion helpers"

### What I did

- OpenAI Chat Completions:
  - added table-driven review-derived reducer scenarios for metadata-only final chunks and sparse tool argument deltas;
  - added canonical event validation for the new projected event traces.
- Claude:
  - added table-driven `ContentBlockMerger` scenarios for metadata-only message stop, split/sparse tool JSON deltas preserving identity, and stream error after active text.
- OpenAI Responses:
  - added SSE-backed engine tests for metadata-only `response.completed`, sparse function-call finalization, and stream error after active text;
  - fixed sparse function-call finalization so `response.output_item.done` can reuse previously observed call id, name, output index, status, and arguments from `responsesStreamState.callsByItem`.
- Gemini:
  - extracted `geminiStreamState` and `reduceGeminiStreamResponse` into `stream_reducer.go`;
  - changed `RunInference` to delegate per-chunk provider normalization to the reducer while keeping SDK iteration and final completion in the engine;
  - added table-driven reducer tests for metadata-only final chunks, multiple text chunks, and complete function calls.
- Updated the Geppetto provider testing guide to mention the new Gemini seam.
- Extracted Gemini stream consumption and terminal completion helpers into `stream_helpers.go`.
- Added a Gemini helper test proving that a terminal stream error after active text closes the active text segment, appends the partial assistant text block, emits an error event, and publishes a failed provider-call finish.

### Why

The guide intentionally says that each provider should keep its native input fixtures while sharing canonical lifecycle scenario names. These tests prove that approach works: each provider got provider-specific tables or fixtures, but the checked behavior maps back to the same scenario IDs (`P06`, `T07`, `S01`, tool lifecycle, and text accumulation).

### What worked

- Targeted package tests passed after each provider checkpoint:

```bash
go test ./pkg/steps/ai/openai -count=1
go test ./pkg/steps/ai/claude -count=1
go test ./pkg/steps/ai/openai_responses -count=1
go test ./pkg/steps/ai/gemini -count=1
```

- Geppetto pre-commit ran and passed for each code commit:

```bash
go test ./...
make lintmax
```

- The OpenAI Responses sparse function-call test exposed a real provider-normalization gap, and the fix stayed small.
- Gemini now has a reducer seam suitable for additional provider-native chunk tests.

### What didn't work

Nothing remained broken, but there were important implementation constraints:

- OpenAI Responses active-text terminal error currently preserves partial text and emits failed provider-call finish, but the test does not assert a text-segment-finished event because the current Responses completion path does not explicitly close active text on provider error.
- Gemini terminal completion is no longer entirely inline; the new `completeGeminiStream` helper covers final turn-block/error handling, but additional terminal scenario rows can still be added if review wants broader EOF/cancel coverage.

### What I learned

The provider-specific table approach is practical. Claude did not need a rewrite because `ContentBlockMerger` is already reducer-like. Gemini was the outlier; extracting a small chunk reducer made tests easy without a large engine rewrite.

### What was tricky to build

OpenAI Responses sparse function-call finalization was the sharp edge. The final `response.output_item.done` event can be sparse and omit name/call id even though an earlier `response.output_item.added` carried them. The fix was to merge missing final fields from `streamState.callsByItem[itemID]` before deciding whether to emit `ToolCallRequested` and final tool-call blocks.

Gemini was tricky because its SDK chunks use concrete `genai` types. The reducer keeps those native types, rather than inventing a fake provider-neutral input shape, which aligns with the guide.

### What warrants a second pair of eyes

- Review OpenAI Responses sparse function-call finalization to ensure fallback from `callsByItem` is correct for call id, name, output index, status, and arguments.
- Review whether Responses should also emit `TextSegmentFinished` on stream error with active text, or whether preserving the final assistant block plus failed provider-call finish is enough for Phase 1.
- Review Gemini helper boundaries: chunk normalization, stream consumption, and terminal completion are now extracted, while request setup and SDK model orchestration remain in `RunInference`.

### What should be done in the future

- Add more OpenAI Responses tests for reasoning summary/text terminal behavior.
- Add any remaining Gemini terminal completion scenarios if we want broader EOF/error/cancel final turn-block behavior.
- Carry the deferred Pinocchio sparse patch/projection/frontend scenarios into Phase 2-5 test docs and implementations.

### Code review instructions

Review commits in order:

```bash
git -C geppetto show 4bcf089
git -C geppetto show fab1d3c
git -C geppetto show 904c77a
git -C geppetto show aeb3c38
git -C geppetto show 1be5777
git -C geppetto show e57d532
git -C geppetto show 59df153
```

Start with these files:

```text
geppetto/pkg/steps/ai/openai/chat_stream_reducer_test.go
geppetto/pkg/steps/ai/claude/content-block-merger_test.go
geppetto/pkg/steps/ai/openai_responses/engine_test.go
geppetto/pkg/steps/ai/openai_responses/stream_events.go
geppetto/pkg/steps/ai/gemini/stream_reducer.go
geppetto/pkg/steps/ai/gemini/stream_helpers.go
geppetto/pkg/steps/ai/gemini/engine_gemini_test.go
geppetto/docs/design/implementation/01-provider-event-testing.md
```

Validate with:

```bash
cd geppetto
go test ./pkg/steps/ai/openai ./pkg/steps/ai/claude ./pkg/steps/ai/openai_responses ./pkg/steps/ai/gemini -count=1
go test ./...
make lintmax
```

### Technical details

The new tests intentionally do not share a provider-neutral fixture format. They share scenario intent and canonical trace expectations while keeping native provider fixtures in each provider package.

## Step 20: Start Pinocchio Phase 2 runtime sink protocol matrix

I started the Pinocchio-side conformance work at the layer where canonical Geppetto events first enter Pinocchio: `runtimeEventSink`. This is the right first table because it exercises the Go runtime translation boundary before sessionstream projection, frontend reduction, or persistence add more transformations.

The implementation is intentionally table-driven rather than a broad framework. Each row is a small protocol program made of synthetic Geppetto canonical events, and the assertions inspect the protobuf backend events published by the sink. This gives us reducer-like coverage of the stateful runtime translator without rewriting it first.

### Prompt Context

**User prompt (verbatim):** "remember: where would we build table diven tests, and does it make sense to do a similar more reducer oriented refactor?

continue"

**Assistant interpretation:** Continue into Pinocchio, answer the testing/refactor placement question through implementation, and keep recording the work in the ticket diary.

**Inferred user intent:** Begin the Pinocchio conformance phase with concrete table-driven tests, while deciding whether to refactor runtime/projection/frontend code toward reducer-shaped seams.

**Commit (code):** `52a16af` — "Test chat runtime protocol terminal cases"

### What I did

- Added `pkg/chatapp/runtime_sink_protocol_test.go`.
- Created a small `recordingEventPublisher` for capturing backend `sessionstream.Event` values published by `runtimeEventSink`.
- Added a table-driven `TestRuntimeEventSinkProtocolMatrix` with rows for:
  - `RUNTIME-01` active text plus `EventError` closes the text segment and publishes `ChatRunFailed`;
  - `RUNTIME-02` active text plus `EventInterrupt` closes the text segment and publishes `ChatRunStopped`;
  - `RUNTIME-03` a closed text segment is not rewritten by a later error;
  - `RUNTIME-04` an error without active text does not manufacture `ChatTextSegmentFinished`;
  - `RUNTIME-05` provider-call finish after text finish does not rewrite text and preserves provider-call correlation.
- Added helper assertions for event names, event counts, and typed correlation propagation, including optional zero index pointers.

### Why

The provider-side work made Geppetto canonical events reliable enough to use as test inputs. Pinocchio now needs to prove that its first translation layer preserves those semantics. The runtime sink is stateful: it remembers the active text segment, latest text, and latest text correlation. That makes it the natural first reducer-like target.

### What worked

Targeted tests passed:

```bash
cd pinocchio
go test ./pkg/chatapp -run TestRuntimeEventSinkProtocolMatrix -count=1
go test ./pkg/chatapp -count=1
```

The existing `runtimeEventSink` did not need a code refactor for this first matrix. The test could exercise its current state machine directly by feeding canonical Geppetto events and capturing published protobuf events.

### What didn't work

No implementation failure remained. The main constraint was choosing the correct scope: testing `runtimeEventSink` directly gives precise terminal/correlation coverage, while a full service/hub integration test would be slower and less explicit for these lifecycle rows.

### What I learned

Pinocchio already has reducer-shaped units. `runtimeEventSink.PublishEvent` is a stateful reducer from canonical Geppetto event to zero-or-more protobuf backend events. `baseTimelineProjection`, plugin projections, `timelineMutationFromUIEvent`, and `timelineSlice.upsertEntity` are also reducer-shaped. We should test these seams first and only extract pure helpers when tests become awkward.

### What was tricky to build

The sharp edge is terminal text finalization. `EventError` and `EventInterrupt` are not correlated text events, so `finishActiveTextSegment` must reuse the last active text correlation and message ID. The tests therefore need to assert not just that a finish event appears, but that it carries the original text segment correlation, including zero-valued optional indexes.

A second sharp edge is closed-segment protection. Once an explicit `EventTextSegmentFinished` has set `textActive=false`, a later `EventError` must not publish a second failed text finish. The `RUNTIME-03` row asserts this by counting `ChatTextSegmentFinished` events.

### What warrants a second pair of eyes

- Review whether `EventInterrupt` should prefer interrupt text over the last streamed text when the interrupt payload is non-empty. The current table uses an empty interrupt text so partial assistant content is preserved.
- Review whether the runtime sink should publish run-level correlation on `ChatRunFailed`/`ChatRunStopped`; the current test focuses on message ID/status/error because the current payloads do not include correlation in these code paths.
- Review whether the runtime sink should be split into a pure reducer helper later if additional rows make direct `PublishEvent` tests too verbose.

### What should be done in the future

- Add runtime sink rows for reasoning/tool plugin forwarding only where the base runtime sink is responsible; most tool/reasoning behavior belongs in plugin tests.
- Add base timeline projection matrix rows in `pkg/chatapp` for sparse text start/delta/finish behavior.
- Add plugin projection matrices for tool and reasoning sparse updates.
- Add frontend reducer matrix rows for sparse UI patches and Redux merge preservation.

### Code review instructions

Start with:

```text
pinocchio/pkg/chatapp/runtime_sink_protocol_test.go
pinocchio/pkg/chatapp/runtime_sink.go
pinocchio/pkg/chatapp/correlation.go
```

Validate with:

```bash
cd pinocchio
go test ./pkg/chatapp -run TestRuntimeEventSinkProtocolMatrix -count=1
go test ./pkg/chatapp -count=1
```

### Technical details

The test deliberately treats `runtimeEventSink` as the reducer seam instead of extracting a new abstraction first. The input is a native Go slice of `gepevents.Event`; the output is the ordered list of `sessionstream.Event` names and protobuf payloads published through `Engine.publish`.

## Step 21: Preserve sparse text projection correlation in base timeline projection

After the runtime sink table, I moved one layer downstream to `baseTimelineProjection`. This is the sessionstream projection layer that turns protobuf backend text events into `ChatMessageEntity` timeline snapshots. It is a reducer-like function already: it receives the current timeline view plus one backend event, then returns zero or more updated entities.

The new table found and fixed the same sparse-patch class we saw on the provider side. A sparse terminal text event may omit content or carry only a partial correlation object. The projection should preserve previous content and typed provider/correlation identity instead of treating missing fields as clears.

### Prompt Context

**User prompt (verbatim):** (same as Step 20)

**Assistant interpretation:** Continue the Pinocchio table-driven protocol matrix at the next reducer-like layer after runtime translation.

**Inferred user intent:** Turn the planned Pinocchio sparse patch/reducer invariants into concrete tests and fixes.

**Commit (code):** `54dd827` — "Preserve sparse chat projection correlation"

### What I did

- Added `pkg/chatapp/projections_protocol_test.go`.
- Added table-driven `TestBaseTimelineProjectionSparseTextMatrix` covering:
  - sparse `ChatTextSegmentFinished` preserves existing content and full correlation;
  - sparse terminal correlation merges with existing correlation instead of clearing provider identity;
  - sparse `ChatTextDelta` correlation merges while updating content;
  - empty `ChatTextSegmentStarted` without existing content still creates no placeholder entity.
- Updated `pkg/chatapp/projections.go` so text started/delta/finished cases use `mergeCorrelationInfo(existing, update)`.
- Added `mergeCorrelationInfo` to preserve existing correlation fields when sparse updates omit them, while accepting explicitly present update fields and optional zero index pointers.

### Why

The provider guide explicitly deferred sparse protobuf/sessionstream/frontend patch behavior to Pinocchio phases. This is the Go timeline projection version of that invariant. A terminal event with only `segmentIndex` and `segmentType` should not erase `provider`, `model`, `responseId`, `correlationKey`, or optional zero indexes that arrived earlier.

### What worked

The focused projection/runtime tests passed:

```bash
cd pinocchio
go test ./pkg/chatapp -run 'TestBaseTimelineProjectionSparseTextMatrix|TestRuntimeEventSinkProtocolMatrix' -count=1
go test ./pkg/chatapp -count=1
```

The fix stayed local to base text projection. No broader sessionstream changes were needed.

### What didn't work

No command failed. The main issue was that the initial projection code cloned the incoming correlation directly. That was fine for full events, but wrong for sparse terminal/update events because it treated omission as replacement.

### What I learned

The Pinocchio layer really does benefit from reducer terminology. `baseTimelineProjection` already has the shape of a reducer, but its merge semantics were incomplete for correlation. Naming the operation `mergeCorrelationInfo` makes the contract visible: timeline projection updates are patches over existing state, not always full replacements.

### What was tricky to build

Correlation merging is subtle because protobuf scalar strings and non-optional integers cannot always distinguish absent from explicitly empty/zero. For the current text projection use case, the safe rule is to preserve existing string fields when the update string is empty, preserve existing non-optional indexes when the update is zero, and replace optional index pointers only when the update pointer is non-nil. This preserves meaningful zero-valued optional indexes such as `choiceIndex=0`.

### What warrants a second pair of eyes

- Review whether `mergeCorrelationInfo` should move from `projections.go` to `correlation.go` if tool/reasoning projections need the same merge contract.
- Review whether non-optional integer fields such as `ProviderCallIndex` and `SegmentIndex` need explicit presence in a future protobuf revision; today zero is both a meaningful index and the default absence value.
- Review whether base text projection should merge any other sparse fields besides correlation, content, and prompt.

### What should be done in the future

- Reuse or generalize `mergeCorrelationInfo` for tool and reasoning projections if their tests expose the same sparse-correlation clearing behavior.
- Add frontend Redux reducer tests proving the browser-side sparse patch merge preserves the same fields.

### Code review instructions

Start with:

```text
pinocchio/pkg/chatapp/projections.go
pinocchio/pkg/chatapp/projections_protocol_test.go
```

Validate with:

```bash
cd pinocchio
go test ./pkg/chatapp -run TestBaseTimelineProjectionSparseTextMatrix -count=1
go test ./pkg/chatapp -count=1
```

### Technical details

The table constructs existing `ChatMessageEntity` values in a synthetic `sessionstream.TimelineView`, then applies sparse `ChatTextDelta` and `ChatTextSegmentFinished` events. The projected entity must keep the previous provider/correlation identity and optional zero indexes while updating content/status/final state.

## Step 22: Fix OpenAI Responses forced streaming request body

Before continuing deeper into Pinocchio, I handled a review item in Geppetto's OpenAI Responses engine. The runtime had been unified so Responses always uses the streaming path, but the serialized request body did not yet include `stream: true`. That meant OpenAI-compatible servers could return a normal JSON response while Geppetto tried to consume SSE frames.

This was a P1 correctness issue because it broke the single-path streaming design at the provider boundary. The fix keeps the unified runtime path, but makes the HTTP request match it.

### Prompt Context

**User prompt (verbatim):** "pkg/steps/ai/openai_responses/engine.go
    // Responses always uses streaming internally so provider-to-canonical event
    // normalization has one lifecycle path. Profiles may still carry chat.stream
    // for other engines, but this engine forces the request and runtime path.
    return e.runStreamingInference(ctx, t, httpClient, url, b, apiKey, metadata, tap, startT

---



P1 Badge Send stream=true before forcing the streaming path

Because b is marshaled before this call from a responsesRequest that no longer contains a stream field, this forced streaming path still sends a non-streaming Responses request. OpenAI will return a normal JSON response rather than SSE frames, and consumeResponsesSSE will hit EOF without flushing any data: payload, so successful Responses calls complete with no assistant text/tool blocks. Keep the unified streaming runtime, but make the serialized request include \"stream\": true before calling it."

**Assistant interpretation:** Fix the Responses request schema/serialization so the forced streaming runtime sends `stream: true`, then add regression coverage.

**Inferred user intent:** Address a PR review correctness issue before continuing Pinocchio-side protocol tests.

**Commit (code):** `bb23476c` — "Force Responses streaming request body"

### What I did

- Added `Stream *bool json:"stream,omitempty"` to `responsesRequest`.
- Set `reqBody.Stream = &true` in `RunInference` before marshaling the request body.
- Added `TestRunInference_ForcesStreamingRequestBody` to assert the actual serialized HTTP request contains `stream: true` even when profile settings have `Chat.Stream=false`.

### Why

The Responses engine now intentionally has one runtime lifecycle path: streaming. The request body must agree with that runtime path. Otherwise the provider may return non-SSE JSON, and `consumeResponsesSSE` will see EOF without canonical content events.

### What worked

Validation passed:

```bash
cd geppetto
go test ./pkg/steps/ai/openai_responses -run 'TestRunInference_ForcesStreamingRequestBody|TestRunInference_ForcesStreamingUsageIncludesCachedTokens' -count=1
go test ./pkg/steps/ai/openai_responses -count=1
```

### What didn't work

No command failed. The bug was caught by review rather than by existing tests because the previous forced-streaming test used a fake server that returned SSE regardless of the request body.

### What I learned

A unified runtime path needs a request-level assertion. It is not enough to test the consumer path with a cooperative fake server; tests must inspect the outbound request when the runtime is intentionally overriding profile-level stream settings.

### What was tricky to build

The key ordering detail is that `b` is the marshaled request body. Setting a stream field after `json.Marshal(reqBody)` would not fix anything. The engine must set `reqBody.Stream` before marshaling and before calling `runStreamingInference`.

### What warrants a second pair of eyes

- Review whether any OpenAI-compatible Responses providers use a different streaming flag name. For the OpenAI Responses API, `stream: true` is the expected field.
- Review whether token-count/request-preview code should intentionally omit `stream` or share the same request struct with `Stream` left nil.

### What should be done in the future

- Keep outbound request-shape assertions around runtime-forced behaviors, not just response-consumer assertions.

### Code review instructions

Start with:

```text
geppetto/pkg/steps/ai/openai_responses/engine.go
geppetto/pkg/steps/ai/openai_responses/helpers.go
geppetto/pkg/steps/ai/openai_responses/engine_test.go
```

Validate with:

```bash
cd geppetto
go test ./pkg/steps/ai/openai_responses -count=1
```

### Technical details

The regression test installs a custom `http.DefaultClient` transport, decodes `r.Body`, and fails unless `body["stream"] == true`. The fake server then returns a minimal SSE `response.completed` event so the unified streaming runtime can finish normally.

## Step 23: Add tool plugin sparse projection matrix

I continued Pinocchio Phase 3 at the tool plugin layer. The tool plugin translates canonical Geppetto tool events into Pinocchio protobuf events and then projects those backend events into tool timeline entities. This is another reducer-like seam: a tool backend event plus the current timeline view produces an updated `ToolCallEntity` or `ToolResultEntity`.

The new table encodes the sparse update rules that were previously only implicit. A terminal tool update should not clear the known tool name, input, or correlation just because the terminal payload is sparse. Display fallback labels such as `tool` should also remain a rendering concern, not persisted canonical state.

### Prompt Context

**User prompt (verbatim):** "then, continue with pinocchio, commit at appropriate intervals, keep a detailed diary as you work"

**Assistant interpretation:** Resume Pinocchio conformance matrices, commit focused checkpoints, and keep detailed diary entries.

**Inferred user intent:** Continue implementing downstream protocol-hardening tests after the Geppetto review fix.

**Commit (code):** `08f1cfa` — "Preserve sparse tool projection correlation"

### What I did

- Added `TestToolCallPluginSparseProjectionMatrix` in `pkg/chatapp/plugins/toolcall_test.go`.
- Added rows for:
  - sparse `ChatToolCallFinished` preserving tool name, input, and full correlation;
  - sparse `ChatToolCallArgumentsDelta` preserving known tool name while updating accumulated input;
  - empty tool name staying empty instead of persisting the display fallback `tool`.
- Promoted correlation helpers into `pkg/chatapp/correlation.go`:
  - `CloneCorrelationInfo`;
  - `MergeCorrelationInfo`.
- Updated base text projection and tool projection to use the shared merge helper.

### Why

The same sparse-patch rule applies to tool entities: missing fields are omitted updates, not clears. Tool calls are particularly sensitive because name/input determine what the user sees and what was executable.

### What worked

Validation passed:

```bash
cd pinocchio
go test ./pkg/chatapp ./pkg/chatapp/plugins -run 'TestBaseTimelineProjectionSparseTextMatrix|TestToolCallPluginSparseProjectionMatrix' -count=1
go test ./pkg/chatapp ./pkg/chatapp/plugins -count=1
```

### What didn't work

Moving the clone helper out of `toolcall.go` briefly broke `reasoning.go`, which was using the package-level `cloneCorrelationInfo` helper from the same `plugins` package:

```text
pkg/chatapp/plugins/reasoning.go:191:23: undefined: cloneCorrelationInfo
```

I restored a small package-level wrapper in `toolcall.go` that delegates to `chatapp.CloneCorrelationInfo`, so existing plugin code kept compiling while the shared helper became available to other packages.

### What I learned

The tool and reasoning plugins already shared an accidental package-local helper. Promoting the real correlation merge/clone logic into `chatapp` is the right direction, but plugin-local wrappers may still be useful for compatibility inside the package until all plugin code is cleaned up.

### What was tricky to build

Tool correlation has optional zero-valued indexes (`outputIndex=0`, `toolCallIndex=0`) that must survive sparse updates. A simple clone of sparse incoming correlation would drop those pointers. `MergeCorrelationInfo` preserves existing optional pointers unless the update explicitly provides a pointer.

### What warrants a second pair of eyes

- Review whether `CloneCorrelationInfo` should use protobuf clone instead of manual struct copy. The manual copy currently deep-copies optional pointer fields and avoids unknown-field concerns for this internal DTO.
- Review whether `cloneCorrelationInfo` should move from `toolcall.go` into a small `plugins/correlation.go` helper file for clarity.

### What should be done in the future

- Add frontend Redux tests that apply tool requested/finished sparse mutations and assert final browser state preserves name/input/correlation.

### Code review instructions

Start with:

```text
pinocchio/pkg/chatapp/correlation.go
pinocchio/pkg/chatapp/plugins/toolcall.go
pinocchio/pkg/chatapp/plugins/toolcall_test.go
```

Validate with:

```bash
cd pinocchio
go test ./pkg/chatapp/plugins -run TestToolCallPluginSparseProjectionMatrix -count=1
```

### Technical details

`MergeCorrelationInfo` treats empty strings and zero non-optional integers as absent updates, and treats non-nil optional integer pointers as explicit updates. This is the same merge policy used by the base text projection matrix.

## Step 24: Add reasoning plugin sparse projection matrix

I then applied the same sparse projection approach to reasoning timeline entities. Reasoning segments are represented as chat message entities with role `thinking`, so sparse terminal reasoning events must preserve previous content, parent message ID, and correlation fields.

The new table also exposed a small empty-placeholder bug. A reasoning start with no content and no existing entity should be handled, but should not return a timeline entity with an empty ID and nil payload.

### Prompt Context

**User prompt (verbatim):** (same as Step 23)

**Assistant interpretation:** Continue Pinocchio plugin matrices after tool projection, focusing on reasoning sparse updates.

**Inferred user intent:** Complete the first pass over Go plugin projection protocol tests before moving to frontend reducers.

**Commit (code):** `9b69e84` — "Preserve sparse reasoning projection state"

### What I did

- Added `TestReasoningPluginSparseProjectionMatrix` in `pkg/chatapp/plugins/reasoning_test.go`.
- Added rows for:
  - sparse reasoning finish preserving prior content, parent ID, and full correlation;
  - sparse reasoning delta preserving provider identity while updating content;
  - empty reasoning start without existing content creating no entity.
- Updated `reasoningEntityFromFields` to:
  - preserve existing parent message ID when incoming parent ID is empty;
  - merge correlation via `chatapp.MergeCorrelationInfo`;
  - derive segment fields from merged correlation.
- Updated `ProjectTimeline` to return no timeline entity when the reducer decides there is no visible entity to create.

### Why

Reasoning has the same sparse patch risks as text and tools. Terminal reasoning events often carry less provider/correlation detail than earlier deltas. Projection should preserve the durable identity from earlier events.

### What worked

Validation passed:

```bash
cd pinocchio
go test ./pkg/chatapp/plugins -run 'TestReasoningPluginSparseProjectionMatrix|TestToolCallPluginSparseProjectionMatrix' -count=1
go test ./pkg/chatapp ./pkg/chatapp/plugins -count=1
```

### What didn't work

The first run of the new reasoning test exposed this failure:

```text
--- FAIL: TestReasoningPluginSparseProjectionMatrix
    --- FAIL: TestReasoningPluginSparseProjectionMatrix/REASONING-PROJECTION-03_empty_start_without_existing_content_creates_no_entity
        reasoning_test.go:265:
             Error Trace: .../pkg/chatapp/plugins/reasoning_test.go:265
             Error:       Expected nil, but got: []sessionstream.TimelineEntity{sessionstream.TimelineEntity{Kind:"ChatMessage", Id:"", CreatedOrdinal:0x0, LastEventOrdinal:0x0, Payload:(*chatappv1.ChatMessageEntity)(nil), Tombstone:false}}
```

The underlying cause was that `reasoningEntityFromFields` could return `(messageID="", entity=nil, ok=true, err=nil)` to mean “handled but no visible entity,” but `ProjectTimeline` always wrapped that into a timeline entity. I fixed `ProjectTimeline` to return nil when `entity == nil` or `messageID` is empty.

### What I learned

A projection reducer sometimes needs to distinguish “not handled” from “handled and intentionally produced no entity.” Returning `ok=true` with `entity=nil` is a valid pattern, but the caller must respect it.

### What was tricky to build

Reasoning projection merges two different concepts into one timeline entity: transcript-like content and reasoning-specific identity. Sparse terminal events may legitimately omit content and parent message ID while still closing the reasoning segment. The reducer must preserve existing visible content and identity while still applying terminal status/streaming state.

### What warrants a second pair of eyes

- Review the `ok=true, entity=nil` convention in `reasoningEntityFromEvent`; it now works, but a clearer return type could make this less error-prone.
- Review whether reasoning source should also be persisted/merged on `ChatMessageEntity`; current code does not store source in the Go timeline entity, while frontend mutations do carry source.

### What should be done in the future

- Move next to frontend table-driven tests in `cmd/web-chat/web/src/ws`, where sparse UI mutations and Redux merge behavior are the main remaining risk.

### Code review instructions

Start with:

```text
pinocchio/pkg/chatapp/plugins/reasoning.go
pinocchio/pkg/chatapp/plugins/reasoning_test.go
pinocchio/pkg/chatapp/correlation.go
```

Validate with:

```bash
cd pinocchio
go test ./pkg/chatapp/plugins -run TestReasoningPluginSparseProjectionMatrix -count=1
```

### Technical details

The reasoning table uses a synthetic timeline view with an existing `ChatMessageEntity`, then applies sparse reasoning delta/finish backend events. The projected entity must keep prior provider/model/response/item/correlation fields and optional zero indexes while updating content/status/streaming state.

## Step 25: Add frontend sparse timeline reducer matrix

After the Go runtime/projection/plugin matrices, I moved to the web-chat frontend reducer layer. This is where UI frames become sparse timeline mutations and Redux merges those mutations into renderable state.

The test is intentionally end-to-end within the frontend reducer layer: it calls `timelineMutationFromUIEvent`, then applies `timelineSlice.upsertEntity`. This catches bugs that do not appear if we inspect only the first mutation in isolation.

### Prompt Context

**User prompt (verbatim):** (same as Step 23)

**Assistant interpretation:** Continue Pinocchio protocol matrices into the frontend reducer layer and keep committing/documenting checkpoints.

**Inferred user intent:** Encode the deferred frontend sparse patch/reducer invariants after the Go-side runtime/projection/plugin matrices.

**Commit (code):** `c2027de` — "Test frontend sparse timeline reductions"

### What I did

- Added `cmd/web-chat/web/src/ws/timelineProtocol.test.ts`.
- Added frontend matrix rows for:
  - sparse tool finish preserving `name`, `toolName`, parsed input, raw input, done/status, and correlation after Redux merge;
  - missing tool name not persisting the generic display fallback `tool` as canonical state;
  - sparse text finish preserving previous content and correlation while updating terminal status/final state after Redux merge.
- Updated `timelineEvents.ts` so:
  - tool call/result mutations no longer persist `name: 'tool'` when the provider did not supply a tool name;
  - `ChatTextSegmentFinished` always emits a sparse message upsert with terminal status/streaming/final fields, even when terminal content is omitted;
  - sparse finished text mutations omit missing `content` so Redux keeps previous content.

### Why

The frontend is the last place sparse updates can accidentally become destructive replacements. The Redux merge reducer preserves existing props only if incoming sparse mutations omit absent fields. Therefore the UI-frame mapper must not include fake fallback values or explicit `undefined` content fields that overwrite previous state.

### What worked

After installing frontend dependencies, focused frontend tests and typecheck passed:

```bash
cd pinocchio/cmd/web-chat/web
npx vitest run src/ws/timelineProtocol.test.ts src/ws/wsManager.test.ts
npm run typecheck
```

### What didn't work

The first attempt to run Vitest failed because frontend dependencies were not installed:

```text
failed to load config from /home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/cmd/web-chat/web/vite.config.ts
Error [ERR_MODULE_NOT_FOUND]: Cannot find package '@vitejs/plugin-react'
```

I ran:

```bash
cd pinocchio/cmd/web-chat/web
npm install
```

Then Vitest ran, but the first frontend matrix exposed the text terminal sparse-update bug:

```text
FAIL src/ws/timelineProtocol.test.ts > frontend timeline protocol matrix > FRONTEND-03 sparse text finish preserves prior content and correlation after Redux merge
AssertionError: expected 'streaming' to be 'failed'
Expected: "failed"
Received: "streaming"
```

The cause was that `ChatTextSegmentFinished` returned no `upsert` when terminal content was sparse. Redux therefore had no entity patch to update status/final/streaming. I changed the mapper to emit a sparse message upsert even without content.

### What I learned

Frontend protocol tests need to apply both stages: UI-frame mapping and Redux merge. Looking only at `timelineMutationFromUIEvent` would not prove that sparse patches preserve existing state after merging.

### What was tricky to build

The tricky part is representing absence correctly in JavaScript objects. If `content: undefined` is included in the incoming props, Redux shallow merge can overwrite the previous content with `undefined`. The fix is to construct sparse props with `definedProps` and only include `content` when the terminal payload has visible text. The same reasoning applies to generic tool fallback labels: the renderer can display `tool`, but the state patch should omit `name`/`toolName` if the provider did not supply one.

### What warrants a second pair of eyes

- Review whether `ChatReasoningSegmentFinished` needs the same sparse terminal frontend behavior as `ChatTextSegmentFinished`; current matrix did not add that row yet.
- Review whether `ToolResultReady` should also avoid persisting fallback `name` in all display paths; the current change removes the fallback from state mutations and leaves rendering fallback to cards.

### What should be done in the future

- Add frontend reasoning sparse terminal rows.
- Add tool result sparse/fallback rows if review wants result-specific coverage.
- Consider adding a small helper for applying UI frames to a reducer state if more frontend protocol rows accumulate.

### Code review instructions

Start with:

```text
pinocchio/cmd/web-chat/web/src/ws/timelineEvents.ts
pinocchio/cmd/web-chat/web/src/ws/timelineProtocol.test.ts
pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts
```

Validate with:

```bash
cd pinocchio/cmd/web-chat/web
npx vitest run src/ws/timelineProtocol.test.ts src/ws/wsManager.test.ts
npm run typecheck
```

### Technical details

The new test file defines `applyFrames`, which calls `timelineMutationFromUIEvent` for each frame and then applies the resulting sparse mutation to `timelineSlice.reducer`. This keeps the test close to browser behavior while avoiding a full React/browser harness.
