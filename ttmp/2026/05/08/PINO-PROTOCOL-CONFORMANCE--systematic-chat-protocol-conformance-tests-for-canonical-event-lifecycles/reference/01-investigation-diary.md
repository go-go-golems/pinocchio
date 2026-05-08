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
LastUpdated: 2026-05-08T15:45:00-04:00
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
