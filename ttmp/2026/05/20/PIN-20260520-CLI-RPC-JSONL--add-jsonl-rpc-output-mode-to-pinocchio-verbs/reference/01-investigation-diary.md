---
Title: Investigation diary
Ticket: PIN-20260520-CLI-RPC-JSONL
Status: active
Topics:
    - pinocchio
    - cli
    - rpc
    - jsonl
    - structured-output
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/events/printer.go
      Note: Confirmed existing structured printer behaviour and compatibility constraints
    - Path: ../../../../../../../geppetto/pkg/events/step-printer-func.go
      Note: Confirmed source of human output markers mentioned in the prompt
    - Path: ../../../../../../../sessionstream/pkg/sessionstream/fanout.go
      Note: Investigation identified UIFanout as the key adapter seam
    - Path: cmd/switch-profiles-tui
      Note: Removal recorded in Step 19
    - Path: cmd/web-chat/app/server.go
      Note: Investigation source for working sessionstream chatapp integration
    - Path: pkg/chatapp/chat_test.go
      Note: Phase 5 validation tests recorded in diary
    - Path: pkg/chatapp/rpc/jsonl/fanout.go
      Note: Phase 3 implementation artifact recorded in diary
    - Path: pkg/chatapp/rpc/jsonl/fanout_test.go
      Note: Phase 3 validation tests recorded in diary
    - Path: pkg/chatapp/rpc/jsonl/writer.go
      Note: Phase 2 implementation artifact recorded in diary
    - Path: pkg/chatapp/rpc/jsonl/writer_test.go
      Note: Phase 2 validation tests recorded in diary
    - Path: pkg/chatapp/rpc/rpc_proto_test.go
      Note: Phase 1 validation test recorded in diary
    - Path: pkg/chatapp/runner.go
      Note: Phase 4 implementation artifact recorded in diary
    - Path: pkg/chatapp/runner_test.go
      Note: Phase 4 validation tests recorded in diary
    - Path: pkg/chatapp/runtime_inference.go
      Note: Phase 5 runtime seeding changes recorded in diary
    - Path: pkg/chatapp/runtime_sink.go
      Note: Investigation identified existing Geppetto-to-chat event mapping
    - Path: pkg/chatapp/service.go
      Note: Phase 5 implementation artifact recorded in diary
    - Path: pkg/cmds/cmd.go
      Note: |-
        Primary investigation target for runtime output handler selection
        Phase 6 CLI RPC implementation recorded in diary
        Step 18 command TUI migration diary artifact
    - Path: pkg/cmds/cmd_rpc_jsonl_test.go
      Note: Phase 6 integration tests recorded in diary
    - Path: pkg/cmds/cmdlayers/helpers.go
      Note: |-
        Primary investigation target for public CLI flags
        Phase 6 helper flag changes recorded in diary
    - Path: pkg/cmds/run/context.go
      Note: Phase 6 run mode changes recorded in diary
    - Path: pkg/ui/backend.go
      Note: Raw forwarder removal recorded in Step 19
    - Path: pkg/ui/chatapp_backend.go
      Note: Step 18 TUI migration diary artifact
    - Path: pkg/ui/chatapp_fanout.go
      Note: Phase 7 adapter implementation recorded in diary
    - Path: pkg/ui/chatapp_fanout_test.go
      Note: Phase 7 adapter validation recorded in diary
    - Path: pkg/ui/fanout_proxy.go
      Note: Step 18 fanout proxy diary artifact
    - Path: proto/pinocchio/chatapp/rpc/v1/rpc.proto
      Note: Phase 1 implementation artifact recorded in diary
ExternalSources: []
Summary: Chronological investigation diary for the Pinocchio JSONL/RPC CLI output design.
LastUpdated: 2026-05-20T12:45:00-04:00
WhatFor: Use to understand how the JSONL/RPC output-mode design was researched and what evidence shaped the recommendations.
WhenToUse: When continuing the ticket, reviewing the design, or implementing the proposed Pinocchio/Geppetto changes.
---












# Diary

## Goal

This diary records the investigation and design work for adding a parse-safe JSON Lines / RPC-style output mode to Pinocchio's CLI verbs. It captures the commands that were run, the files that shaped the design, and the decisions that should guide a future implementation.

## Step 1: Create the ticket workspace and inspect repository shape

I created a dedicated docmgr ticket for the requested design work, then checked the workspace layout to confirm that the current shell was one level above the actual Pinocchio repository. This mattered because docmgr was configured from the parent workspace, while source files and ticket files live under the `pinocchio/` subdirectory.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket to add a json output flag to pinocchio's verbs, so that one can run pinocchio in a jsonl RPC fashion on the CLI, instead of having the standard streaming structured output with like --- Output ended --- and other things. That just makes it hard to parse the output in a script context, and it also makes it hard to do multi round interactions with the system (without using the TUI). So it's in a way a --rpc mode .

Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.

Keep a diary as you work."

**Assistant interpretation:** Create a docmgr ticket, research Pinocchio's CLI output/event architecture, write an intern-ready design and implementation guide for a JSONL/RPC output mode, keep a diary, validate the ticket, and upload the deliverable bundle to reMarkable.

**Inferred user intent:** Make Pinocchio usable from scripts and subprocess clients without brittle parsing of human-oriented stream markers or requiring the TUI for multi-round workflows.

**Commit (code):** N/A — no source code changes were made; this turn created and updated ticket documentation only.

### What I did

- Ran `pwd && docmgr status --summary-only && git status --short && rg --files | head -200` from `/home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli`.
- Observed docmgr status for the Pinocchio docs root.
- Observed `git status --short` failed because the current directory was not a Git repository root.
- Ran `find . -maxdepth 3 -type d | sed 's#^./##' | head -80 && find . -maxdepth 3 -name .git -type d` to inspect the workspace layout.
- Ran `cd pinocchio && find ttmp/2026 -maxdepth 4 -name index.md | head -40 | sed 's#/index.md##'` to inspect existing ticket naming conventions.
- Created ticket `PIN-20260520-CLI-RPC-JSONL` with:
  - `docmgr ticket create-ticket --ticket PIN-20260520-CLI-RPC-JSONL --title "Add JSONL RPC output mode to Pinocchio verbs" --topics pinocchio,cli,rpc,jsonl,structured-output`
- Added the design doc:
  - `docmgr doc add --ticket PIN-20260520-CLI-RPC-JSONL --doc-type design-doc --title "JSONL RPC output mode for Pinocchio CLI verbs"`
- Added this diary doc:
  - `docmgr doc add --ticket PIN-20260520-CLI-RPC-JSONL --doc-type reference --title "Investigation diary"`

### Why

- A ticket workspace provides stable paths for the design, diary, changelog, and tasks.
- Inspecting existing tickets avoided inventing a naming scheme inconsistent with the repository.
- Confirming the repository layout prevented accidental source-control assumptions from the parent workspace.

### What worked

- `docmgr status --summary-only` showed the configured docs root as `pinocchio/ttmp`.
- `docmgr ticket create-ticket` created the expected ticket structure under `ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs`.
- `docmgr doc add` created both the primary design document and diary document.

### What didn't work

- Running `git status --short` from `/home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli` failed with:

```text
fatal: not a git repository (or any of the parent directories): .git
```

This was expected after discovering that the actual repository is in the `pinocchio/` subdirectory.

### What I learned

- The workspace contains at least two sibling repositories, `pinocchio` and `geppetto`, and the requested feature crosses that boundary because Pinocchio's CLI output handlers call Geppetto event printers.
- The ticket root is governed by `.ttmp.yaml` in the parent workspace, but ticket files are stored below `pinocchio/ttmp`.

### What was tricky to build

- The only tricky part in this step was disambiguating the workspace root from the Git repository root. The symptom was the failed `git status`; the solution was to inspect directories and then run source-control and repository discovery commands from `pinocchio/`.

### What warrants a second pair of eyes

- The ticket ID `PIN-20260520-CLI-RPC-JSONL` follows observed Pinocchio ticket conventions, but a maintainer may prefer a shorter `GP-*` or project-specific tracking ID if this will map to an external tracker.

### What should be done in the future

- Continue source-code commands from `/home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio` unless intentionally inspecting the sibling `geppetto` repository.

### Code review instructions

- Review ticket setup at `ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/`.
- Validate with `docmgr doctor --ticket PIN-20260520-CLI-RPC-JSONL --stale-after 30` after all docs and relations are updated.

### Technical details

- Ticket path: `pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs`.
- Primary design doc path: `pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/design-doc/01-jsonl-rpc-output-mode-for-pinocchio-cli-verbs.md`.
- Diary path: `pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/reference/01-investigation-diary.md`.

## Step 2: Map Pinocchio command loading and output architecture

I traced how Pinocchio verbs are loaded, how common helper flags are attached, and where runtime output handlers are selected. The key discovery was that the requested feature belongs in Pinocchio's helper layer and run context, while the low-level printer is best added near Geppetto's existing event printers.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Research the concrete files involved in Pinocchio's CLI command and output path, then turn that evidence into implementation guidance.

**Inferred user intent:** Ensure the design is not hand-wavy and gives a new intern enough code references to implement safely.

**Commit (code):** N/A — investigation only.

### What I did

- Ran `cd pinocchio && rg --files cmd pkg | sort > /tmp/pino_files.txt` to capture source file inventory.
- Ran a broad ripgrep query over `cmd` and `pkg` for output/router/event/Cobra terms and saved it to `/tmp/pino_rg.txt`.
- Ran `find cmd/pinocchio pkg/cmds pkg/geppettocompat pkg/inference -maxdepth 4 -type f | sort` to narrow the file set.
- Ran targeted ripgrep over `cmd/pinocchio`, `pkg/cmds`, `pkg/inference`, and `pkg/geppettocompat` for key symbols including `RunIntoWriter`, `StepPrinterFunc`, `NewStructuredPrinter`, `output-format`, and `json`.
- Read these Pinocchio files:
  - `pkg/cmds/cmd.go`
  - `pkg/cmds/cmdlayers/helpers.go`
  - `pkg/cmds/run/context.go`
  - `cmd/pinocchio/main.go`
  - `pkg/cmds/loader.go`
- Searched the sibling Geppetto repository for event printer and codec symbols.
- Read these Geppetto files:
  - `geppetto/pkg/events/printer.go`
  - `geppetto/pkg/events/step-printer-func.go`
  - `geppetto/pkg/events/chat-events.go`
  - `geppetto/pkg/events/sink.go`
  - `geppetto/pkg/events/event-router.go`
- Used a short Python script to print line references for important symbols across Pinocchio and Geppetto.

### Why

- The design needed to explain the whole path from Cobra command creation to streaming event printing.
- The user specifically mentioned `--- Output ended ---`, so I needed to find the exact printer that emits that marker.
- The feature is about scriptability; therefore, it was important to identify whether existing `--output json` already solved the problem or only partially overlapped.

### What worked

- `pkg/cmds/cmdlayers/helpers.go` clearly showed the existing helper settings: `Output`, `WithMetadata`, and `FullOutput`.
- `pkg/cmds/cmd.go` clearly showed two output-handler selection branches: one in blocking mode and one in the initial interactive step.
- `geppetto/pkg/events/step-printer-func.go` directly confirmed that `--- Output started ---` and `--- Output ended ---` are emitted by the default human text printer.
- `geppetto/pkg/events/printer.go` confirmed that a structured printer already exists, which should be preserved for compatibility rather than repurposed silently.

### What didn't work

- No source-code failure occurred during investigation.
- The main ambiguity was product-level naming: the user asked for a "json output flag" and "in a way a `--rpc` mode". I resolved this by recommending both `--rpc` and `--output jsonl`, where `--rpc` is the semantic convenience flag and `jsonl` is the output-format value.

### What I learned

- Pinocchio prompt commands are already centralized enough that one helper-layer change can reach YAML-loaded prompt verbs.
- The existing `--output json` emits newline-delimited JSON-like records, but its shape is a simplified debug/structured output, not a protocol envelope.
- A clean implementation should add a new Geppetto event printer rather than changing `StepPrinterFunc` or `NewStructuredPrinter`.
- `--rpc` should imply `--non-interactive` for the first implementation so JSONL stdout is not mixed with chat continuation prompts or TUI control output.

### What was tricky to build

- The output mode selection is duplicated in `pkg/cmds/cmd.go`. If an intern adds JSONL logic in only one branch, blocking mode and interactive initial-step mode will diverge. The design therefore recommends extracting a shared `addCLIOutputHandler` helper before wiring the JSONL printer.

### What warrants a second pair of eyes

- The exact boundary between Pinocchio and Geppetto should be reviewed. I recommend putting the printer in Geppetto because the codec and event types live there, but the envelope is motivated by Pinocchio's CLI UX.
- The semantics of `--rpc --chat` should be decided explicitly. The design recommends fail-fast or future work rather than mixing Bubble Tea with JSONL stdout.

### What should be done in the future

- During implementation, add unit tests before any live LLM smoke tests.
- Consider a later bidirectional stdin/stdout mode only after output-only JSONL is stable.

### Code review instructions

- Start with `pkg/cmds/cmdlayers/helpers.go` to review public flags.
- Then inspect `pkg/cmds/run/context.go` to confirm settings propagation.
- Then inspect `pkg/cmds/cmd.go` to verify output handler selection is centralized.
- Finally inspect `geppetto/pkg/events/rpc_printer.go` once implemented to validate the JSONL contract.

### Technical details

Key evidence captured in the design:

- `cmd/pinocchio/main.go:85` builds Cobra commands for `run-command` YAML files.
- `cmd/pinocchio/main.go:207-245` loads prompt repositories through `PinocchioCommandLoader`.
- `pkg/cmds/loader.go:74` adds the helper layer for YAML-loaded commands.
- `pkg/cmds/cmd.go:192` adds the helper layer for directly constructed commands.
- `pkg/cmds/cmdlayers/helpers.go:17-35` defines output-related helper settings.
- `pkg/cmds/cmdlayers/helpers.go:138-140` defines `--output` choices as `text`, `json`, and `yaml` today.
- `pkg/cmds/run/context.go:23-31` stores UI output settings.
- `pkg/cmds/cmd.go:427-444` selects the blocking-mode printer.
- `pkg/cmds/cmd.go:584-593` selects the interactive initial-step printer.
- `geppetto/pkg/events/step-printer-func.go:107` prints `--- Output started ---`.
- `geppetto/pkg/events/step-printer-func.go:113` prints `--- Output ended ---`.
- `geppetto/pkg/events/printer.go:71-98` implements existing structured printing.
- `geppetto/pkg/events/chat-events.go:98-102` defines the event interface.
- `geppetto/pkg/events/chat-events.go:155-167` defines metadata IDs that are useful for RPC envelopes.
- `geppetto/pkg/events/chat-events.go:220` starts `NewEventFromJson`.

## Step 3: Write the intern-ready design and implementation guide

I wrote the primary design document as an implementation guide rather than a short RFC. The document explains the current architecture, the gaps, the proposed public API, the JSONL envelope, pseudocode, diagrams, implementation phases, tests, risks, alternatives, and file references.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Produce a thorough but navigable design package that a new engineer can follow without prior Pinocchio or Geppetto context.

**Inferred user intent:** Reduce implementation risk by turning the investigation into a concrete roadmap with enough context for onboarding.

**Commit (code):** N/A — documentation only.

### What I did

- Rewrote `design-doc/01-jsonl-rpc-output-mode-for-pinocchio-cli-verbs.md` with:
  - Executive summary.
  - Problem statement and scope.
  - Current-state architecture with evidence.
  - Gap analysis.
  - Proposed user-facing flags.
  - JSONL/RPC envelope schema.
  - Event kind mapping table.
  - Mermaid flow and sequence diagrams.
  - Pinocchio and Geppetto pseudocode.
  - Multi-round RPC direction.
  - Phased implementation plan.
  - Testing strategy.
  - Risks and alternatives.
  - Intern implementation checklist.
  - Absolute file references.

### Why

- The user asked for a guide suitable for a new intern, so the document includes explanatory prose and step-by-step guidance rather than assuming knowledge of Glazed, Cobra, Watermill, or Geppetto events.
- The proposed design intentionally separates immediate output-only JSONL from later bidirectional RPC so the first implementation remains tractable.

### What worked

- The evidence gathered in Step 2 mapped cleanly into a minimal implementation plan.
- The existing event router and printer architecture supports the feature without replacing the inference engine or command loader.

### What didn't work

- No doc-writing command failed.

### What I learned

- The best design is additive: preserve text, JSON, and YAML output modes; add `jsonl`/`--rpc` as a stronger contract.
- The intern guide should explicitly warn about duplicated printer selection logic and stdout contamination from logging or TUI prompts.

### What was tricky to build

- The design needed to be specific enough to implement but not overpromise a full daemon-style JSON-RPC system. I handled this by defining a versioned output envelope now and adding a separate future section for stdin request lines.

### What warrants a second pair of eyes

- The exact JSON field names (`kind`, `event_type`, `delta`, `text`, `raw_event`) should be reviewed before implementation because they become a client contract.
- Whether `--output jsonl` should be exactly equivalent to `--rpc` or only select the output format should be finalized by the maintainer.

### What should be done in the future

- Add the implementation following the phases in the design doc.
- Add examples to Pinocchio help once the flag exists.

### Code review instructions

- Review the design for API contract clarity first.
- Review the implementation phases for whether they match the repository's preferred test boundaries.
- Validate the final design with `docmgr doctor` and upload bundle verification.

### Technical details

- Primary design doc updated at `/home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/design-doc/01-jsonl-rpc-output-mode-for-pinocchio-cli-verbs.md`.

## Step 4: Re-evaluate RPC around sessionstream and chatapp

I studied the sibling `sessionstream/` repository and Pinocchio's `pkg/chatapp/` package to test the hypothesis that a new JSONL/RPC printer would duplicate existing stream-mapping work. The result changed the recommendation: JSONL/RPC should be built as an adapter over `sessionstream` + `chatapp`, not as another direct raw Geppetto event mapper.

### Prompt Context

**User prompt (verbatim):** "Can you study sessionstream/ and @pinocchio/pkg/chatapp/ and how this might apply to pinocchio verbs for the TUI and the RPC and just the streamign even mapping in general? I think we are recreating the same thing here and we might just as well use the existing chatapp package. It's used by web-chat for now but I think it should be abl eto be used for the TUI and CLI as well.

Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Extend the existing ticket with a deeper design that analyzes sessionstream and chatapp as a shared foundation for Pinocchio CLI, TUI, RPC JSONL, and event-stream mapping.

**Inferred user intent:** Avoid building yet another event mapping layer and instead converge web-chat, TUI, and CLI/RPC around the same canonical chat application model.

**Commit (code):** N/A — documentation and design only.

### What I did

- Inspected repository layout and confirmed `sessionstream/` is a sibling repository of `pinocchio/`.
- Listed `pinocchio/pkg/chatapp` files and `sessionstream/pkg/sessionstream` files.
- Ran broad ripgrep searches for `chatapp`, `sessionstream`, `Runtime`, `Projection`, `Event`, `Sink`, `web-chat`, `timeline`, `Subscribe`, `hydration`, and `protocol`.
- Read the core sessionstream files:
  - `sessionstream/README.md`
  - `sessionstream/pkg/sessionstream/types.go`
  - `sessionstream/pkg/sessionstream/schema.go`
  - `sessionstream/pkg/sessionstream/hub.go`
  - `sessionstream/pkg/sessionstream/projection.go`
  - `sessionstream/pkg/sessionstream/hydration.go`
  - `sessionstream/pkg/sessionstream/fanout.go`
  - `sessionstream/pkg/sessionstream/transport/ws/server.go`
- Read the core chatapp files:
  - `pinocchio/pkg/chatapp/chat.go`
  - `pinocchio/pkg/chatapp/service.go`
  - `pinocchio/pkg/chatapp/runtime_inference.go`
  - `pinocchio/pkg/chatapp/runtime_sink.go`
  - `pinocchio/pkg/chatapp/projections.go`
  - `pinocchio/pkg/chatapp/features.go`
  - `pinocchio/pkg/chatapp/plugins/reasoning.go`
  - `pinocchio/pkg/chatapp/plugins/toolcall.go`
  - `pinocchio/proto/pinocchio/chatapp/v1/chat.proto`
- Read the web-chat integration:
  - `pinocchio/cmd/web-chat/app/server.go`
  - `pinocchio/cmd/web-chat/main.go`
  - `pinocchio/cmd/web-chat/runtime_composer.go`
- Re-read the current TUI/raw event mapping:
  - `pinocchio/pkg/ui/backend.go`
  - `pinocchio/pkg/ui/timeline_persist.go`
  - `pinocchio/pkg/cmds/cmd.go`
- Created a second design doc: `design-doc/02-unify-pinocchio-cli-tui-and-rpc-streams-on-sessionstream-chatapp.md`.

### Why

- The original design proposed a new JSONL printer over raw Geppetto events. The user pointed out that `chatapp` likely already solves the semantic mapping problem.
- `sessionstream.UIFanout` is a natural adapter seam for stdout JSONL and Bubble Tea TUI output, just as websocket transport already uses it.
- Studying the existing packages prevents designing a redundant protocol that diverges from web-chat.

### What worked

- `chatapp.runtimeEventSink` clearly already maps Geppetto events into typed Pinocchio chat events.
- `chatapp` plugins already handle reasoning and tool calls.
- `sessionstream` already owns command/event/projection/hydration/fanout mechanics.
- `cmd/web-chat/app/server.go` provides a working integration blueprint for non-web runners.

### What didn't work

- The reusable runtime composer is currently in `cmd/web-chat/runtime_composer.go`, which is not a good import boundary for Pinocchio CLI code. The design recommends extracting pure runtime composition to a package.
- `chatapp.PromptRequest` currently carries `Prompt string` but not a full `turns.Turn`, while Pinocchio verbs can produce richer turns with system prompts, blocks, and images. The design identifies this as the main mismatch and recommends adding `InitialTurn *turns.Turn` first.

### What I learned

- The best RPC stream is probably not raw `geppetto/pkg/events` JSONL. It should be JSONL frames over `sessionstream`/`chatapp` UI events or backend events.
- The current TUI path is a parallel projection system because it decodes raw Geppetto events and emits Bubble Tea timeline messages directly.
- The current CLI timeline persistence path also duplicates projection logic that chatapp/sessionstream already centralizes.

### What was tricky to build

- The tricky design point was separating transport from semantics. Web-chat currently makes it look like chatapp is web-specific, but the actual package boundary is already transport-agnostic: `chatapp.Service` submits commands, `sessionstream.Hub` runs projections, and `sessionstream.UIFanout` is just an interface. The design therefore proposes new non-web adapters rather than moving websocket concepts into CLI.

### What warrants a second pair of eyes

- Whether `--rpc` should emit projected UI events, backend events, or both by default.
- Whether `chatapp.PromptRequest` should add an in-memory `InitialTurn *turns.Turn` or whether a durable protobuf turn schema should be designed immediately.
- Where to place the extracted runtime composer: `pkg/chatapp`, `pkg/inference/runtime`, or another package.

### What should be done in the future

- Implement a reusable `chatapp.Runner` that mirrors web-chat setup without HTTP.
- Implement JSONL and Bubble Tea adapters around `sessionstream.UIFanout`.
- Route `--rpc` through chatapp first, then migrate the TUI after parity tests.

### Code review instructions

- Start with `design-doc/02-unify-pinocchio-cli-tui-and-rpc-streams-on-sessionstream-chatapp.md`.
- Review `chatapp/runtime_sink.go` to verify that the proposed RPC adapter is not duplicating existing Geppetto-to-chat mapping.
- Review `sessionstream/fanout.go` and websocket transport to confirm `UIFanout` is the right adapter seam.
- Review `pkg/ui/backend.go` and `pkg/ui/timeline_persist.go` for the duplicated raw mappings that the migration aims to retire.

### Technical details

- New design doc path: `/home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/design-doc/02-unify-pinocchio-cli-tui-and-rpc-streams-on-sessionstream-chatapp.md`.
- The revised recommendation is to use `chatapp` and `sessionstream` as the canonical stream model for RPC/TUI instead of adding a second raw Geppetto JSONL mapping layer.

## Step 5: Make the JSONL boundary protobuf-defined

I updated the follow-up design so the RPC JSONL line format is itself defined by protobuf. The revised boundary is no longer an ad-hoc JSON object produced from maps or hand-written structs; it is one `pinocchio.chatapp.rpc.v1.RpcLine` protobuf JSON message per stdout line, carrying chatapp payloads through `google.protobuf.Any`.

### Prompt Context

**User prompt (verbatim):** "ok update the ticket to make the whole thing protobuf defined, since that gives us a clear boundary."

**Assistant interpretation:** Amend the ticket design so the CLI JSONL/RPC protocol has a generated protobuf schema for the outer line envelope as well as for inner chatapp payloads.

**Inferred user intent:** Avoid an untyped JSON contract and make the CLI/RPC boundary explicit, versioned, generated, and aligned with sessionstream/chatapp's protobuf-first architecture.

**Commit (code):** N/A — documentation and design only.

### What I did

- Updated `design-doc/02-unify-pinocchio-cli-tui-and-rpc-streams-on-sessionstream-chatapp.md` with a new section: `Protobuf-defined JSONL line format`.
- Added a proposed proto location: `proto/pinocchio/chatapp/rpc/v1/rpc.proto`.
- Added a `pinocchio.chatapp.rpc.v1.RpcLine` schema sketch with `HelloFrame`, `SnapshotFrame`, `UiEventFrame`, `BackendEventFrame`, `ErrorFrame`, and `DoneFrame`.
- Specified `google.protobuf.Any` for UI/backend event payloads and snapshot entity payloads.
- Added example JSONL lines showing protobuf JSON output with `@type` payloads.
- Added writer and `sessionstream.UIFanout` pseudocode using `protojson.MarshalOptions` and `anypb.New`.
- Updated open questions to mark the websocket `ServerFrame` reuse question as decided: define a Pinocchio-specific `RpcLine` envelope while mirroring sessionstream concepts.
- Updated `tasks.md` so implementation tasks include creating the proto, regenerating bindings, implementing the protojson writer, and testing round-trip unmarshal of every emitted line.

### Why

- Protobuf gives the JSONL protocol a clear schema boundary while preserving shell-friendly line-delimited JSON.
- The inner payloads are already concrete chatapp protobuf messages, so the outer envelope should also be generated and versioned.
- `google.protobuf.Any` keeps payloads strongly typed and self-describing through the protobuf JSON `@type` field.

### What worked

- The existing sessionstream/chatapp design made this update straightforward because commands, backend events, UI events, and timeline entities are already protobuf-backed.
- The new design fits naturally into the earlier recommendation to implement JSONL as a `sessionstream.UIFanout` adapter.

### What didn't work

- No commands failed in this step.

### What I learned

- Reusing `sessionstream.v1.ServerFrame` directly would be possible, but it is too websocket/subscription-oriented for a CLI subprocess protocol. A Pinocchio-specific `RpcLine` envelope is the clearer boundary.
- Protobuf JSON will encode `uint64` ordinals as strings, so CLI docs and jq examples must mention `tonumber` when numeric comparisons are needed.

### What was tricky to build

- The subtle design point was keeping the protocol strongly typed without making simple shell usage unpleasant. The compromise is protobuf JSON over JSONL: generated schema and `Any` payloads for typed clients, ordinary JSON objects for `jq` users.

### What warrants a second pair of eyes

- The exact package name and generated Go path for `pinocchio.chatapp.rpc.v1` should be reviewed before implementation.
- The frame set should be reviewed before it becomes a public contract, especially whether `backend_event` should be included from day one or reserved for a later `--rpc-events` option.

### What should be done in the future

- Add the actual `rpc.proto` file and generated bindings.
- Implement the JSONL writer and fanout adapter with round-trip tests.
- Update CLI help to describe protobuf JSON output and `Any` payloads.

### Code review instructions

- Review the new `Protobuf-defined JSONL line format` section in design doc 02.
- Check that implementation uses generated protobuf types, not ad-hoc JSON maps, for the outer line envelope.
- Verify tests unmarshal JSONL lines back into `chatapprpcv1.RpcLine` and unpack representative `Any` payloads.

### Technical details

- Updated design doc: `/home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/design-doc/02-unify-pinocchio-cli-tui-and-rpc-streams-on-sessionstream-chatapp.md`.
- Updated task list: `/home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/tasks.md`.

## Step 6: Split implementation work into phased ticket tasks

I turned the design into a phased implementation checklist so the work can proceed in reviewable commits. The phases start at the protocol boundary, then add the JSONL writer, sessionstream fanout, chatapp runner, rich Pinocchio verb input, CLI integration, TUI integration, and final cleanup.

### Prompt Context

**User prompt (verbatim):** "Ok, add detailed tasks split up in phases to the ticket, then work on them step by step, committing at appropriate intervals, keeping a detailed diary"

**Assistant interpretation:** Expand the ticket task list into concrete implementation phases, then begin executing those phases with focused commits and diary updates.

**Inferred user intent:** Make the large migration tractable and reviewable, with enough traceability to resume or audit each implementation step.

**Commit (code):** pending — planning docs will be committed before source implementation begins.

### What I did

- Rewrote `tasks.md` with eight implementation phases:
  - Phase 1: protobuf contract and generated bindings.
  - Phase 2: protojson JSONL writer package.
  - Phase 3: sessionstream JSONL fanout and snapshot helpers.
  - Phase 4: reusable non-web chatapp runner.
  - Phase 5: rich Pinocchio verb input in chatapp.
  - Phase 6: CLI RPC integration.
  - Phase 7: Bubble Tea/TUI adapter over sessionstream.
  - Phase 8: docs, cleanup, and de-duplication.
- Added a current implementation checkpoint marking Phase 1 as active.

### Why

- The design spans protocol, generation, runtime, CLI, and TUI changes. Splitting work into phases reduces review risk.
- The first implementation phase should establish the protobuf boundary before any adapters depend on it.

### What worked

- The existing design docs already contained enough detail to turn into concrete tasks.
- The phased plan gives each commit an obvious validation target.

### What didn't work

- N/A for this planning step.

### What I learned

- The work should not start with `pkg/cmds/cmd.go`; that would entangle protocol design, runtime composition, and CLI behavior. Starting with proto and writer packages is safer.

### What was tricky to build

- The tricky part was making the task list detailed enough for an intern without pretending that every later phase is fully known. I kept later TUI de-duplication tasks explicit but conditional on parity tests.

### What warrants a second pair of eyes

- Phase 5's `InitialTurn *turns.Turn` approach should be reviewed before implementation because it introduces in-process-only request data that is not yet replayable through protobuf commands.

### What should be done in the future

- Begin Phase 1 by adding `rpc.proto`, regenerating bindings, adding a compile test, validating with Buf/Go tests, and committing.

### Code review instructions

- Review `tasks.md` first to confirm phase boundaries.
- Ensure each implementation commit maps to one phase or a small part of one phase.

### Technical details

- Updated task list path: `/home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/tasks.md`.

## Step 7: Implement Phase 1 protobuf contract and generated bindings

I implemented the first phase of the plan: the JSONL/RPC line envelope now has a protobuf schema, generated Go and TypeScript bindings, and a minimal Go round-trip test that proves the generated `RpcLine` type can be serialized and deserialized with protobuf JSON.

### Prompt Context

**User prompt (verbatim):** (same as Step 6)

**Assistant interpretation:** Begin executing the phased implementation plan with focused source changes and validation.

**Inferred user intent:** Establish the generated protocol boundary first so later writer and fanout code cannot drift into ad-hoc JSON.

**Commit (code):** pending — Phase 1 source and diary changes are ready for a focused commit.

### What I did

- Added `proto/pinocchio/chatapp/rpc/v1/rpc.proto`.
- Defined `pinocchio.chatapp.rpc.v1.RpcLine` with:
  - `version`
  - `session_id`
  - `request_id`
  - `oneof frame`
- Defined frame messages:
  - `HelloFrame`
  - `SnapshotFrame`
  - `SnapshotEntity`
  - `UiEventFrame`
  - `BackendEventFrame`
  - `ErrorFrame`
  - `DoneFrame`
- Used `google.protobuf.Any` for event/entity payloads.
- Reserved field range `100 to 199` for future expansion.
- Ran Buf lint for the new proto.
- Generated Go bindings with `buf.chatapp.gen.yaml`.
- Generated TypeScript bindings with `buf.chatapp.web.gen.yaml`.
- Added `pkg/chatapp/rpc/rpc_proto_test.go` to import `chatapprpcv1` and round-trip a hello frame through `protojson`.

### Why

- This creates the explicit protocol boundary requested by the user.
- Later JSONL writer code can now accept `*chatapprpcv1.RpcLine` instead of untyped maps.
- TypeScript clients can consume the same generated schema.

### What worked

- `buf lint --path proto/pinocchio/chatapp/rpc/v1/rpc.proto` passed.
- `buf generate --template buf.chatapp.gen.yaml --path proto/pinocchio/chatapp/rpc/v1/rpc.proto` generated `pkg/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc.pb.go`.
- `buf generate --template buf.chatapp.web.gen.yaml --path proto/pinocchio/chatapp/rpc/v1/rpc.proto` generated `cmd/web-chat/web/src/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc_pb.ts`.
- `go test ./pkg/chatapp/rpc ./pkg/chatapp/... -count=1` passed.

### What didn't work

- The first test version used a composite literal directly in an `if err := ...; err != nil` initializer:

```go
if err := protojson.UnmarshalOptions{DiscardUnknown: false}.Unmarshal(b, &decoded); err != nil {
```

Go parsed that incorrectly and reported:

```text
pkg/chatapp/rpc/rpc_proto_test.go:29:5: expected boolean expression, found assignment (missing parentheses around composite literal?)
```

I fixed it by adding parentheses around the composite literal:

```go
if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(b, &decoded); err != nil {
```

### What I learned

- The existing chatapp Buf generation templates already work for the new nested `chatapp/rpc/v1` proto path.
- We can generate web TypeScript bindings immediately; no separate generator wiring was needed for Phase 1.

### What was tricky to build

- The main implementation detail was choosing the `go_package` path so it stays under the existing chatapp generated package tree while getting a clean import alias (`chatapprpcv1`).

### What warrants a second pair of eyes

- The exact field numbers and frame set in `rpc.proto` should be reviewed because this is the public contract foundation.
- The reserved field range is intentionally broad but arbitrary; reviewers may prefer explicit comments or a different reservation strategy.

### What should be done in the future

- Implement Phase 2: a mutex-protected protojson JSONL writer that writes exactly one `RpcLine` per line.

### Code review instructions

- Start with `proto/pinocchio/chatapp/rpc/v1/rpc.proto`.
- Verify generated files match the proto and do not introduce unexpected changes to existing chatapp v1 bindings.
- Run `buf lint --path proto/pinocchio/chatapp/rpc/v1/rpc.proto`.
- Run `go test ./pkg/chatapp/rpc ./pkg/chatapp/... -count=1`.

### Technical details

Commands run:

```bash
buf lint --path proto/pinocchio/chatapp/rpc/v1/rpc.proto
buf generate --template buf.chatapp.gen.yaml --path proto/pinocchio/chatapp/rpc/v1/rpc.proto
buf generate --template buf.chatapp.web.gen.yaml --path proto/pinocchio/chatapp/rpc/v1/rpc.proto
gofmt -w pkg/chatapp/rpc/rpc_proto_test.go
go test ./pkg/chatapp/rpc ./pkg/chatapp/... -count=1
```

## Step 8: Fix generated TypeScript import ordering found by pre-commit

The first Phase 1 commit attempt ran the repository pre-commit hooks and failed in `web-check`. The generated TypeScript file was valid but did not satisfy Biome's import-order rule. I fixed the generated file with Biome's safe write mode and kept the fix in the same Phase 1 commit.

### Prompt Context

**User prompt (verbatim):** (same as Step 6)

**Assistant interpretation:** Keep committing at appropriate intervals and record validation failures as they happen.

**Inferred user intent:** Preserve a reliable implementation diary that captures both successful validation and hook failures.

**Commit (code):** pending — this fix is part of the Phase 1 protobuf contract commit.

### What I did

- Attempted to commit Phase 1 with `git commit -m "chatapp: add protobuf JSONL RPC envelope"`.
- The pre-commit hook ran broad validation, including `go generate`, `go build`, golangci-lint, `go test ./...`, frontend typecheck, and frontend Biome lint.
- Biome reported unsorted imports in the generated TypeScript file.
- Ran:

```bash
cd cmd/web-chat/web && npx --yes @biomejs/biome@2.3.8 check --write src/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc_pb.ts
```

### Why

- The generated web binding is part of the protobuf boundary. Keeping it checked in means it must satisfy the repository's frontend lint rules.

### What worked

- The pre-commit hook's Go-side validation passed before the frontend lint failure:
  - `go build ./...`
  - golangci-lint
  - `go vet -vettool=/tmp/geppetto-lint ./...`
  - `go test ./...`
  - frontend typecheck
- Biome fixed the import ordering automatically.

### What didn't work

- The commit failed because Biome reported:

```text
src/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc_pb.ts:5:1 assist/source/organizeImports FIXABLE
✖ The imports and exports are not sorted.
```

- The first commit command also previously timed out at 120 seconds while the hook was still running, so I reran with a longer timeout.

### What I learned

- Generating chatapp TypeScript bindings may require a Biome organize-imports pass before commit.
- The pre-commit hook is expensive because it can install frontend dependencies, build the web UI, run full Go tests, and run frontend checks.

### What was tricky to build

- The generated file is tool-owned, but repository lint still enforces formatting. The least invasive fix was running the repository's existing Biome version against only that generated file.

### What warrants a second pair of eyes

- If generated TS files frequently need Biome post-processing, the generation workflow could add an explicit formatting step.

### What should be done in the future

- After future TypeScript generation, run `npx --yes @biomejs/biome@2.3.8 check --write <generated-file>` before committing.

### Code review instructions

- Review the TypeScript generated file only for expected generated-schema content and import ordering; do not hand-edit semantic generated code.

### Technical details

- File fixed: `/home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/web-chat/web/src/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc_pb.ts`.

## Step 9: Implement Phase 2 protojson JSONL writer

I implemented the small writer package that turns generated `RpcLine` protobuf messages into newline-delimited protobuf JSON. This phase deliberately avoids sessionstream fanout logic; it only guarantees the low-level output invariant that every successful write is one complete JSON object followed by one newline.

### Prompt Context

**User prompt (verbatim):** (same as Step 6)

**Assistant interpretation:** Continue executing the phased implementation plan after the protobuf envelope commit.

**Inferred user intent:** Build the implementation in layers so the RPC writer can be reviewed independently from fanout and CLI integration.

**Commit (code):** pending — Phase 2 writer changes are ready for focused commit.

### What I did

- Added `pkg/chatapp/rpc/jsonl/writer.go`.
- Implemented `Writer` with:
  - `protojson.MarshalOptions{EmitUnpopulated:false, UseProtoNames:false}`.
  - mutex-protected writes.
  - `WriteLine(*chatapprpcv1.RpcLine) error`.
  - nil-writer and nil-line validation.
- Added frame constructor helpers:
  - `NewHelloLine`
  - `NewErrorLine`
  - `NewDoneLine`
- Added `pkg/chatapp/rpc/jsonl/writer_test.go` with tests for:
  - newline-terminated single-line output.
  - protojson round-trip back into `RpcLine`.
  - invalid input not emitting `{}`.
  - concurrent writes producing complete parseable lines.
  - helper constructor fields.
- Ran `go test ./pkg/chatapp/rpc/... -count=1`.

### Why

- Later fanout and CLI code should not manually marshal JSON maps. This writer makes the protobuf-defined boundary easy to use correctly.
- The mutex is important because sessionstream fanout could be called from concurrent paths in future adapters.

### What worked

- Targeted tests passed:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc	0.002s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc/jsonl	0.002s
```

### What didn't work

- N/A for this step.

### What I learned

- Keeping the writer package separate from sessionstream fanout makes the output framing invariant straightforward to test.

### What was tricky to build

- The main edge case is avoiding accidental empty JSON objects. A nil `RpcLine` is technically marshalable to `{}`, but that would be a protocol bug, so the writer rejects nil lines.

### What warrants a second pair of eyes

- Whether `NewErrorLine` should duplicate `err.Error()` into both `message` and `detail` or reserve `detail` for richer diagnostics later.

### What should be done in the future

- Phase 3 should use this writer from a `sessionstream.UIFanout` implementation and add `Any` payload tests.

### Code review instructions

- Review `pkg/chatapp/rpc/jsonl/writer.go` first.
- Confirm `WriteLine` is the only place that writes raw bytes and that it always appends exactly one newline per call.
- Run `go test ./pkg/chatapp/rpc/... -count=1`.

### Technical details

Commands run:

```bash
gofmt -w pkg/chatapp/rpc/jsonl/writer.go pkg/chatapp/rpc/jsonl/writer_test.go
go test ./pkg/chatapp/rpc/... -count=1
```

## Step 10: Implement Phase 3 sessionstream JSONL fanout and snapshot helpers

I implemented the adapter that connects sessionstream projected UI events to the protobuf-defined JSONL writer. This is the first code that uses the new `RpcLine` boundary as an application transport: UI events and snapshot entities are packed into `google.protobuf.Any`, wrapped in generated RPC frames, and emitted as protobuf JSON lines.

### Prompt Context

**User prompt (verbatim):** (same as Step 6)

**Assistant interpretation:** Continue implementing the phased plan by connecting sessionstream fanout to the protobuf JSONL writer.

**Inferred user intent:** Reuse the existing sessionstream/chatapp projection model instead of adding raw Geppetto stream mapping.

**Commit (code):** pending — Phase 3 fanout changes are ready for focused commit.

### What I did

- Added `pkg/chatapp/rpc/jsonl/fanout.go`.
- Implemented `UIFanout` satisfying `sessionstream.UIFanout`.
- Implemented `PublishUI` to:
  - iterate over `sessionstream.UIEvent` batches,
  - pack event payloads with `anypb.New`,
  - write `RpcLine_UiEvent` frames with session ID, ordinal, event name, and typed payload.
- Added control/supplementary helpers:
  - `WriteHello`
  - `WriteError`
  - `WriteDone`
  - `WriteSnapshot`
  - `WriteBackendEvent`
- Added nil payload validation so malformed UI events and snapshot entities fail instead of emitting ambiguous frames.
- Added `pkg/chatapp/rpc/jsonl/fanout_test.go` covering:
  - `ChatTextPatch` UI event output.
  - `ChatTextSegmentFinished` UI event output.
  - `ChatRunFinished` UI event output.
  - `Any` unpacking into concrete chatapp messages.
  - snapshot entity packing/unpacking.
  - hello/error/done/backend-event helper frames.
  - nil payload errors.

### Why

- `sessionstream.UIFanout` is the shared seam for web, CLI JSONL, and future TUI adapters.
- Packing payloads with `Any` preserves typed chatapp payload boundaries while keeping the stream shell-readable as JSONL.

### What worked

- Targeted tests passed:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc	0.003s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc/jsonl	0.009s
```

### What didn't work

- The first helper signature for unpacking `Any` payloads used a variadic `proto.UnmarshalOptions` signature, but the installed `anypb.Any.UnmarshalTo` method has the simpler signature `UnmarshalTo(proto.Message) error`. The build failed with messages like:

```text
*anypb.Any does not implement interface{UnmarshalTo(proto.Message, ...proto.UnmarshalOptions) error}
```

I fixed the test helper to accept `*anypb.Any` directly.

### What I learned

- The existing protobuf runtime version exposes `Any.UnmarshalTo(proto.Message) error`, so test helpers should not assume newer/alternate variadic signatures.
- The fanout adapter stays small because sessionstream already batches UI events with ordinals and chatapp already produces concrete protobuf payloads.

### What was tricky to build

- The subtle part was keeping this package generic enough for sessionstream UI events while still enforcing the Pinocchio chatapp RPC boundary. The package does not inspect specific chatapp message types; tests use chatapp messages to prove `Any` payloads round-trip.

### What warrants a second pair of eyes

- `WriteBackendEvent` lives on `UIFanout` for convenience even though backend events are not part of the `sessionstream.UIFanout` interface. Reviewers may prefer a separate backend recorder type later.

### What should be done in the future

- Phase 4 should add a reusable non-web chatapp runner that accepts this fanout and mirrors web-chat setup without HTTP.

### Code review instructions

- Review `pkg/chatapp/rpc/jsonl/fanout.go` for the `sessionstream.UIFanout` implementation.
- Confirm `PublishUI` does not inspect or reinterpret Geppetto events.
- Run `go test ./pkg/chatapp/rpc/... -count=1`.

### Technical details

Commands run:

```bash
gofmt -w pkg/chatapp/rpc/jsonl/fanout.go pkg/chatapp/rpc/jsonl/fanout_test.go
go test ./pkg/chatapp/rpc/... -count=1
```

## Step 11: Implement Phase 4 reusable non-web chatapp runner

I added a package-level runner that wires together chatapp and sessionstream without HTTP or websocket dependencies. This is the reusable foundation that CLI RPC and TUI adapters can use instead of duplicating the web-chat server setup.

### Prompt Context

**User prompt (verbatim):** (same as Step 6)

**Assistant interpretation:** Continue the phased migration by extracting the non-web chatapp/sessionstream wiring needed by CLI and TUI surfaces.

**Inferred user intent:** Make web-chat's sessionstream/chatapp architecture reusable by non-web Pinocchio verbs.

**Commit (code):** pending — Phase 4 runner changes are ready for focused commit.

### What I did

- Added `pkg/chatapp/runner.go`.
- Implemented `RunnerOptions` with:
  - optional schema registry,
  - optional hydration store,
  - optional `sessionstream.UIFanout`,
  - optional turn store,
  - plugins,
  - chunk delay.
- Implemented `Runner` with handles to:
  - schema registry,
  - hydration store,
  - hub,
  - engine,
  - service.
- Implemented `NewRunner` to:
  - register chatapp schemas and plugin schemas,
  - create an in-memory SQLite hydration store if none is provided,
  - create a chatapp engine,
  - create a sessionstream hub,
  - install chatapp commands/projections,
  - return a service.
- Implemented `Runner.Close` for cleanup of the default in-memory store.
- Added `pkg/chatapp/runner_test.go` with tests for:
  - submitting a demo prompt through the runner,
  - waiting for idle,
  - observing projected UI events through a recording fanout,
  - fetching a snapshot,
  - registering reasoning and tool plugin schemas.

### Why

- `cmd/web-chat/app/server.go` already has working setup logic, but it is tied to HTTP/websocket server construction. CLI and TUI need the same substrate without importing a command package.
- This runner is the bridge from the earlier protocol/fanout work to actual Pinocchio verb integration.

### What worked

- Targeted tests passed:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp	0.090s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/export	0.029s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/plugins	0.006s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc	0.002s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc/jsonl	0.005s
```

### What didn't work

- N/A for this step.

### What I learned

- The non-web setup is compact because `chatapp.Install` already owns command/projection registration and sessionstream already owns the hub mechanics.
- A recording `UIFanout` is enough to test that the runner produces projected UI events without involving websocket or JSONL transport.

### What was tricky to build

- The runner needed to avoid colliding with the existing `Option` type used by `chatapp.Engine`; the new public config type is `RunnerOptions` rather than another option-function API.

### What warrants a second pair of eyes

- Whether the default hydration store should be SQLite in-memory, as web-chat uses, or a simpler sessionstream memory store if one exists or is added later.
- Whether `Runner` should expose `Registry`, `Store`, `Hub`, and `Engine` directly or hide some of those behind methods.

### What should be done in the future

- Phase 5 should extend `PromptRequest` to accept rich initial turns so Pinocchio verbs can preserve system prompts, blocks, and images when routed through the runner.

### Code review instructions

- Review `pkg/chatapp/runner.go` against `cmd/web-chat/app/server.go` to verify it mirrors the substrate setup without HTTP concerns.
- Run `go test ./pkg/chatapp/... -count=1`.

### Technical details

Commands run:

```bash
gofmt -w pkg/chatapp/runner.go pkg/chatapp/runner_test.go
go test ./pkg/chatapp/... -count=1
```

## Step 12: Implement Phase 5 rich Pinocchio verb input for chatapp

I extended `chatapp.PromptRequest` so non-web callers can provide a fully rendered Geppetto `turns.Turn`. This lets future Pinocchio CLI/RPC integration preserve verb inputs that include system prompts, pre-seeded blocks, image blocks, and templated content instead of reducing every verb to a plain prompt string.

### Prompt Context

**User prompt (verbatim):** (same as Step 6)

**Assistant interpretation:** Continue the phased implementation by resolving the main mismatch between Pinocchio verb execution and chatapp's prompt-string input.

**Inferred user intent:** Route real Pinocchio verbs through chatapp/sessionstream without losing existing command semantics.

**Commit (code):** pending — Phase 5 input changes are ready for focused commit.

### What I did

- Added `InitialTurn *turns.Turn` to `chatapp.PromptRequest`.
- Documented that `InitialTurn` is for in-process CLI/TUI use where the command has already rendered a full Geppetto turn.
- Changed `runPrompt` / `runRuntimeInference` so `runRuntimeInference` receives the full `PromptRequest`, not only the runtime.
- Updated `runRuntimeInference` to:
  - use `pending.InitialTurn.Clone()` when provided,
  - skip turn-store history loading when an explicit initial turn is provided,
  - preserve existing history + `AppendNewTurnFromUserPrompt` behavior when no initial turn is provided.
- Added tests:
  - `TestRuntimeInferenceUsesInitialTurnWhenProvided`
  - `TestRuntimeInferenceInitialTurnSkipsTurnStoreHistory`
- Ran `go test ./pkg/chatapp/... -count=1`.

### Why

- `PinocchioCommand.buildInitialTurn` can produce richer input than a prompt string. Without this change, CLI RPC through chatapp would lose system prompt and block context.
- Skipping turn-store history when `InitialTurn` is explicit avoids accidental double context or history-load failures when the caller is providing the entire seed turn.

### What worked

- Targeted tests passed:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp	0.075s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/export	0.035s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/plugins	0.006s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc	0.003s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc/jsonl	0.004s
```

### What didn't work

- The first edit referenced `pending.InitialTurn` inside `runRuntimeInference`, but the function still accepted only `runtime *infruntime.ComposedRuntime`. The build failed:

```text
pkg/chatapp/runtime_inference.go:95:5: undefined: pending
pkg/chatapp/runtime_inference.go:96:15: undefined: pending
```

- I fixed this by changing `runRuntimeInference` to accept `pending PromptRequest`, deriving `runtime := pending.Runtime` inside the function, and removing the now-unused `infruntime` import from `runtime_inference.go`.

### What I learned

- The existing history-loading tests made it easy to verify that prompt-only behavior still works.
- `InitialTurn` should be treated as authoritative seed context for this first implementation phase.

### What was tricky to build

- The ordering between turn-store history and explicit initial turns is the main semantic decision. I chose explicit `InitialTurn` over history loading because CLI verbs are already constructing the seed context they want the model to see.

### What warrants a second pair of eyes

- Review whether `InitialTurn` should skip history permanently or whether a later option should allow appending an explicit turn after persisted history.
- Review whether `InitialTurn` should eventually be represented in protobuf for durable/replayable command logs.

### What should be done in the future

- Phase 6 can now build a Pinocchio command seed turn and pass it into `chatapp.PromptRequest{InitialTurn: seed}`.

### Code review instructions

- Review `pkg/chatapp/service.go` for the new `PromptRequest.InitialTurn` field and comment.
- Review `pkg/chatapp/runtime_inference.go` for history-vs-initial-turn ordering.
- Run `go test ./pkg/chatapp/... -count=1`.

### Technical details

Commands run:

```bash
gofmt -w pkg/chatapp/service.go pkg/chatapp/runtime_inference.go pkg/chatapp/chat_test.go
go test ./pkg/chatapp/... -count=1
```

## Step 13: Implement Phase 6 CLI RPC JSONL integration

I added the first real Pinocchio CLI integration path for protobuf-defined JSONL RPC output. `--rpc` and `--output jsonl` now route blocking command execution through `chatapp.Runner`, `sessionstream`, and the JSONL `UIFanout` instead of the old raw Geppetto event printer.

### Prompt Context

**User prompt (verbatim):** `continue`

**Assistant interpretation:** Continue the phased implementation after Phase 5, with the next concrete phase being CLI RPC integration.

**Inferred user intent:** Keep implementing, validating, documenting, and committing focused slices of the ticket.

**Commit (code):** pending — Phase 6 CLI RPC changes are ready for a focused commit.

### What I did

- Added `jsonl` as an allowed `--output` choice.
- Added a helper `--rpc` boolean flag.
- Added `RPC bool` to `run.UISettings`.
- Added `run.RunModeRPCJSONL`.
- Updated `RunIntoWriter` so `--rpc` or `--output jsonl` selects the RPC JSONL path before chat/interactive modes.
- Implemented `PinocchioCommand.runRPCJSONL`:
  - renders the command into the same initial `turns.Turn` as the legacy path,
  - derives or assigns a session ID on the turn,
  - creates a protobuf JSONL `UIFanout`,
  - writes a `hello` frame,
  - creates a reusable `chatapp.Runner`,
  - writes an initial snapshot before prompt submission,
  - creates the runtime engine from the existing CLI engine factory/settings,
  - submits `chatapp.PromptRequest{InitialTurn: seed, Runtime: composedRuntime}`,
  - waits for idle,
  - writes final snapshot and done frames,
  - writes terminal error frames for runner, snapshot, engine-init, submit, and wait failures.
- Added integration-style tests in `pkg/cmds/cmd_rpc_jsonl_test.go`:
  - protojson line parsing for every stdout line,
  - hello/done frames,
  - projected user, finished-text, run-finished, and snapshot frames,
  - streaming `ChatTextPatch` from a fake streaming engine,
  - terminal error frame from a fake engine factory failure.

### Why

- This is the first end-to-end proof that Pinocchio verbs can use the same chatapp/sessionstream projection layer as web-chat while still producing script-friendly stdout.
- Keeping non-JSONL output on the legacy raw Geppetto path preserves compatibility for `--output text`, `--output json`, and `--output yaml`.

### What worked

- Targeted tests passed:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/cmds	0.169s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp	0.077s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/export	0.025s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/plugins	0.006s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc	0.003s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp/rpc/jsonl	0.005s
```

### What didn't work

- The first terminal-error test failed because engine creation happened before the JSONL fanout existed, so no error frame could be written. I moved engine creation to after hello, runner creation, and initial snapshot writing.
- The first smoke test initially expected old dotted event names, but chatapp currently emits names such as `ChatUserMessageAccepted`, `ChatTextSegmentFinished`, and `ChatRunFinished`. I updated the tests to assert the actual projected event names.

### What I learned

- The JSONL path now has a useful ordering: hello, initial snapshot, UI events, final snapshot, done/error.
- The CLI path can reuse `PromptRequest.InitialTurn` cleanly; no raw Geppetto event mapping was added to `pkg/cmds`.

### What was tricky to build

- Error frame semantics require the JSONL transport to exist before expensive engine construction. This slightly changes the setup order from the legacy blocking path but produces a better subprocess protocol.
- The fake streaming engine needed to publish canonical Geppetto text segment events so chatapp projections could generate a `ChatTextPatch` frame.

### What warrants a second pair of eyes

- Review whether initial snapshots should always be emitted by default, or only when a future flag asks for snapshots.
- Review whether `runRPCJSONL` should return the final turn from chatapp rather than the seed turn once persistence/replay is wired.
- Review whether `--rpc` should force stderr-only logs globally to guarantee stdout purity under all logger configurations.

### What should be done in the future

- Commit Phase 6.
- Exercise a real command manually with `--output jsonl | jq` once the focused commit is in place.
- Continue with Phase 7: Bubble Tea/TUI adapter over `sessionstream.UIFanout`.

### Code review instructions

- Review `pkg/cmds/cmdlayers/helpers.go` for the new flag/choice behavior.
- Review `pkg/cmds/run/context.go` for `RunModeRPCJSONL` and `UISettings.RPC`.
- Review `pkg/cmds/cmd.go` for the transport setup order and legacy compatibility path.
- Review `pkg/cmds/cmd_rpc_jsonl_test.go` for protocol and error-frame coverage.
- Run `go test ./pkg/cmds ./pkg/chatapp/... -count=1`.

### Technical details

Commands run:

```bash
gofmt -w pkg/cmds/cmd.go pkg/cmds/run/context.go pkg/cmds/cmdlayers/helpers.go pkg/cmds/cmd_rpc_jsonl_test.go
go test ./pkg/cmds ./pkg/chatapp/... -count=1
```

## Step 14: Commit Phase 6 CLI RPC integration

Phase 6 was committed after the full pre-commit hook completed successfully.

### Prompt Context

**User prompt (verbatim):** `continue`

**Assistant interpretation:** Finish the current phase's commit before continuing to the next implementation phase.

**Inferred user intent:** Keep clean phase boundaries and maintain the implementation diary.

**Commit (code):** `cfaf7fb8778b81417450235edc72cb3611d73ddf` (`cmds: route RPC JSONL through chatapp`)

### What I did

- Staged the Phase 6 CLI RPC files and ticket docs.
- Ran the repository pre-commit hook via `git commit`.
- The hook ran `go generate ./...`, frontend install/build, `go build ./...`, `golangci-lint`, `go vet`, and `go test ./...`.
- Commit succeeded.

### What worked

- The full pre-commit hook passed.
- The focused commit contains:
  - `pkg/cmds/cmd.go`
  - `pkg/cmds/cmdlayers/helpers.go`
  - `pkg/cmds/run/context.go`
  - `pkg/cmds/cmd_rpc_jsonl_test.go`
  - ticket task/changelog/design/diary updates

### What didn't work

- No new failures during commit.

### What should be done in the future

- Begin Phase 7 by inspecting current Bubble Tea/TUI event paths and identifying the clean adapter seam for `sessionstream.UIFanout`.

## Step 15: Start Phase 7 with a Bubble Tea chatapp UIFanout adapter

I started Phase 7 by implementing a testable `sessionstream.UIFanout` adapter for Bubble Tea timeline messages. This does not yet migrate `runChat` to the chatapp runner, but it creates the missing adapter seam needed for that migration.

### Prompt Context

**User prompt (verbatim):** `continue`

**Assistant interpretation:** Continue after the Phase 6 commit into the TUI adapter phase.

**Inferred user intent:** Make incremental implementation progress while preserving focused commits and diary updates.

**Commit (code):** pending — Phase 7 adapter slice is ready for review/commit decision.

### What I did

- Added `pkg/ui/chatapp_fanout.go`.
- Implemented `ChatAppUIFanout`, satisfying `sessionstream.UIFanout`.
- Introduced a small `BubbleTeaSender` interface so tests do not need a real `tea.Program`.
- Added `NewChatAppUIFanoutForProgram(*tea.Program)` for production wiring.
- Mapped projected chatapp UI payloads to existing bobatea timeline messages:
  - `ChatUserMessageAccepted`
  - `ChatTextSegmentStarted`
  - `ChatTextPatch`
  - `ChatTextSegmentFinished`
  - `ChatRunFailed`
  - `ChatRunStopped`
  - `ChatReasoningSegmentStarted`
  - `ChatReasoningPatch`
  - `ChatReasoningSegmentFinished`
- Added `HydrateSnapshot(sessionstream.Snapshot)` for pre-existing `ChatMessageEntity` timeline state.
- Added `pkg/ui/chatapp_fanout_test.go` covering:
  - assistant text streaming,
  - final assistant completion,
  - snapshot hydration,
  - user messages,
  - run failures,
  - reasoning/thinking segments.

### Why

- The existing TUI path maps raw Geppetto events in `StepChatForwardFunc`. The new adapter maps projected chatapp UI events instead, matching the design goal of avoiding duplicate raw event mapping in CLI/TUI/RPC surfaces.
- Keeping this as an adapter slice de-risks the eventual chat-mode migration.

### What worked

- Focused tests passed:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/ui	0.027s
```

- Broader targeted suite passed before the final reasoning test was added:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/ui	0.060s
ok  	github.com/go-go-golems/pinocchio/pkg/cmds	0.189s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp	0.123s
```

### What didn't work

- I did not wire the adapter into `runChat` yet. Current chat mode still uses the raw Geppetto Watermill handler path.

### What I learned

- A `sessionstream.UIFanout` is a good seam for the TUI because it can be tested without running the full Bubble Tea program.
- Snapshot hydration can map `ChatMessageEntity` directly into the same `timeline.UIEntityCreated` / `UIEntityCompleted` messages as streaming events.

### What was tricky to build

- The adapter has to preserve the existing bobatea message shape (`llm_text`, `streaming`, `BackendFinishedMsg`) so the current widgets do not need immediate changes.

### What warrants a second pair of eyes

- Review whether user messages should be represented as `llm_text` or a dedicated renderer kind.
- Review append-vs-snapshot semantics for `ChatTextPatch`; the current adapter forwards `text` directly, matching the chatapp projection's accumulated text behavior.
- Review whether interrupts should get a dedicated chatapp event before marking the interrupt parity task complete.

### What should be done in the future

- Wire chat mode to use `chatapp.Runner` with `NewChatAppUIFanoutForProgram`.
- Add an interrupt/stop projection test once chatapp has an explicit projected interruption payload.

### Code review instructions

- Review `pkg/ui/chatapp_fanout.go` for event-to-timeline mapping and snapshot hydration.
- Review `pkg/ui/chatapp_fanout_test.go` for coverage of streaming, completion, failure, reasoning, and hydration.
- Run `go test ./pkg/ui -count=1`.

## Step 16: Commit Phase 7 adapter slice

The preparatory Bubble Tea adapter slice for Phase 7 was committed successfully.

### Prompt Context

**User prompt (verbatim):** `continue`

**Assistant interpretation:** Preserve the focused phase-slice commit and record what remains.

**Inferred user intent:** Keep implementation and diary synchronized with commits.

**Commit (code):** `72a3d17265fafbcafdc470e2f1bdcda542360ec9` (`ui: add chatapp Bubble Tea fanout`)

### What I did

- Staged the new adapter/tests and ticket docs.
- Ran the full pre-commit hook through `git commit`.
- Commit succeeded after full lint/build/test validation.

### What worked

- The full repository pre-commit hook passed, including `go test ./...`.

### What remains

- Full chat-mode migration is not done yet. The next implementation step is to wire `runChat` to `chatapp.Runner` and `NewChatAppUIFanoutForProgram`, or defer that migration and perform Phase 8 documentation cleanup.

## Step 17: Real tmux smoke tests for RPC JSONL and TUI modes

I exercised the new RPC JSONL mode and the interactive TUI path against real configured profiles from tmux sessions.

### Prompt Context

**User prompt (verbatim):** `Continue. Remove all legacy / backwards compatibility / wrappers. Test the TUI and RPC mode for real with PINOCCHIO_PROFILE=gpt-5-nano-low and gpt-5-mini (for more thinking). Use tmux. Commit at appropriate intervals, keep a diary.`

**Assistant interpretation:** Continue implementation, remove transitional wrapper/compatibility code where safe, and validate both script/RPC and TUI paths against real profiles in tmux before committing.

**Inferred user intent:** Move from unit-level validation to real CLI behavior and keep phase documentation trustworthy.

**Commit (code):** pending — cleanup changes are prepared after smoke tests.

### What I tested

Created temporary smoke commands under `ttmp/manual/`:

```yaml
name: rpc-smoke
short: RPC smoke test
prompt: |
  Reply with exactly: rpc smoke ok
```

```yaml
name: rpc-thinking-smoke
short: RPC thinking smoke test
prompt: |
  Think briefly, then answer with exactly one line: mini smoke ok
```

Built a local binary:

```bash
go build -o /tmp/pinocchio-smoke ./cmd/pinocchio
```

Ran RPC JSONL with `PINOCCHIO_PROFILE=gpt-5-nano-low` in tmux:

```bash
tmux new-session -d -s pin-rpc-nano 'cd .../pinocchio && PINOCCHIO_PROFILE=gpt-5-nano-low /tmp/pinocchio-smoke run-command ttmp/manual/rpc-smoke.yaml --output jsonl > /tmp/pin-rpc-nano.out 2> /tmp/pin-rpc-nano.err; echo $? > /tmp/pin-rpc-nano.status'
```

Result:

- exit status: `0`
- stdout lines: `14`
- stderr lines: `0`
- every stdout line parsed as JSON
- frames included: `hello`, initial `snapshot`, `uiEvent`, final `snapshot`, `done`
- assistant text: `rpc smoke ok`

Ran RPC JSONL with `PINOCCHIO_PROFILE=gpt-5-mini` in tmux:

```bash
tmux new-session -d -s pin-rpc-mini 'cd .../pinocchio && PINOCCHIO_PROFILE=gpt-5-mini /tmp/pinocchio-smoke run-command ttmp/manual/rpc-thinking-smoke.yaml --output jsonl > /tmp/pin-rpc-mini.out 2> /tmp/pin-rpc-mini.err; echo $? > /tmp/pin-rpc-mini.status'
```

Result:

- exit status: `0`
- stdout lines: `26`
- stderr lines: `0`
- every stdout line parsed as JSON
- frame counts included:
  - `ChatTextPatch`: `15`
  - `ChatTextSegmentFinished`: `1`
  - `ChatRunFinished`: `1`
  - `snapshot`: `2`
  - `done`: `1`

Ran TUI with `PINOCCHIO_PROFILE=gpt-5-nano-low` in tmux:

```bash
tmux new-session -d -s pin-tui-nano 'cd .../pinocchio && PINOCCHIO_PROFILE=gpt-5-nano-low /tmp/pinocchio-smoke run-command ttmp/manual/rpc-smoke.yaml --chat --force-interactive 2> /tmp/pin-tui-nano.err; echo $? > /tmp/pin-tui-nano.status'
```

Captured pane showed:

```text
(user): Reply with exactly: rpc smoke ok
(thinking):
(assistant): rpc smoke ok
profile: gpt-5-nano-low  engine profile: gpt-5-nano-low
```

Exited the tmux TUI via key sends; exit status was `0`.

Ran TUI with `PINOCCHIO_PROFILE=gpt-5-mini` in tmux:

```bash
tmux new-session -d -s pin-tui-mini 'cd .../pinocchio && PINOCCHIO_PROFILE=gpt-5-mini /tmp/pinocchio-smoke run-command ttmp/manual/rpc-thinking-smoke.yaml --chat --force-interactive 2> /tmp/pin-tui-mini.err; echo $? > /tmp/pin-tui-mini.status'
```

Captured pane showed:

```text
(user): Think briefly, then answer with exactly one line: mini smoke ok
(thinking):
(assistant): No—there’s no safe level of smoking; even a “mini” smoke can harm your health.
profile: gpt-5-mini  engine profile: gpt-5-mini
```

Exited the tmux TUI via key sends; exit status was `0`.

### What worked

- RPC stdout stayed pure JSONL for both real profile runs.
- `gpt-5-mini` produced multiple `ChatTextPatch` frames, so streaming was validated with a real provider path.
- TUI auto-submit via `--chat --force-interactive` worked in tmux for both profiles.
- Profile selection via `PINOCCHIO_PROFILE` was visible in the TUI status bar.

### What didn't work

- The `gpt-5-mini` prompt used the phrase “mini smoke ok”, which the model interpreted as a safety-related smoking request instead of a literal output instruction. This still validated TUI/RPC transport behavior, but it is a bad future smoke prompt.

### What I changed after testing

- Removed the `NewChatAppUIFanoutForProgram` wrapper; callers can use `NewChatAppUIFanout` directly with `*tea.Program` because it already satisfies `BubbleTeaSender`.
- Simplified `pkg/ui/runtime/builder.go` by removing unused handler-factory and `BuildComponents` wrapper APIs.
- Removed a stale compatibility comment from `pkg/cmds/cmd.go`.

### Validation after cleanup

```bash
go test ./pkg/ui/... ./pkg/cmds/... -count=1
```

Result:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/ui	0.017s
ok  	github.com/go-go-golems/pinocchio/pkg/ui/profileswitch	0.017s
ok  	github.com/go-go-golems/pinocchio/pkg/cmds	0.121s
ok  	github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap	0.096s
```

### What warrants a second pair of eyes

- Decide whether the old raw Geppetto `StepChatForwardFunc` should be removed now or only after full chat-mode wiring through chatapp/sessionstream. It is still used by current TUI code paths, so deleting it now would require a broader migration.
- Replace the “mini smoke ok” text with a neutral token in future real-provider tests to avoid safety interpretations.

## Step 18: Wire command TUI chat mode to chatapp/sessionstream and test TAB multiturn

I replaced the Pinocchio command chat-mode path with a chatapp/sessionstream-backed Bubble Tea backend. The command TUI no longer routes its main chat stream through the raw Geppetto Watermill `StepChatForwardFunc` path.

### Prompt Context

**User prompt (verbatim):** `continue. Btw submititng in the TUI is done with TAB, so you can test multiturn`

**Assistant interpretation:** Continue the migration, use TAB instead of Enter for TUI submission, and specifically validate multi-turn behavior in a real tmux TUI.

**Inferred user intent:** Prove the new TUI path is not just one-shot auto-submit but can carry conversation state across turns.

**Commit (code):** pending — TUI migration slice is ready for focused commit.

### What I changed

- Added `pkg/ui/chatapp_backend.go`:
  - implements `bobatea/pkg/chat.Backend`,
  - submits prompts through `chatapp.Service.SubmitPromptRequest`,
  - uses `PromptRequest.InitialTurn` for rich command seed state,
  - waits for chatapp idle,
  - reconstructs the next turn from `sessionstream.Snapshot` so TAB-submitted follow-up prompts keep prior user/assistant context.
- Added `pkg/ui/fanout_proxy.go`:
  - lets `chatapp.Runner` be constructed before the Bubble Tea program exists,
  - then points the sessionstream fanout at `NewChatAppUIFanout(program)` once the program has been created.
- Rewrote `PinocchioCommand.runChat` to:
  - build the system/block seed turn,
  - create a chatapp runner with the fanout proxy,
  - create `ChatAppBackend`,
  - create the Bubble Tea chat model/program directly,
  - install the real `ChatAppUIFanout`,
  - auto-submit the initial command prompt for both `--chat` and `--interactive`,
  - show a simple `profile: ...` status line from the resolved profile.
- Removed `pkg/ui/runtime/builder.go` entirely; it was now an unused transitional raw-handler builder.
- Updated `ChatAppUIFanout` to ignore live `ChatUserMessageAccepted` events because bobatea already renders the submitted user message immediately. Snapshot hydration still renders user messages for existing sessions.
- Updated `chatapp.publishFallbackAssistantText` so non-streaming fallback only publishes assistant blocks produced by the current run. This fixes multi-turn fallback cases where prior assistant text could be concatenated into the new assistant segment.
- Added `pkg/ui/chatapp_backend_test.go` to verify snapshot-based history carry-over across turns.

### Real tmux validation

Built:

```bash
go build -o /tmp/pinocchio-chatapp-tui ./cmd/pinocchio
```

RPC after the migration:

```bash
PINOCCHIO_PROFILE=gpt-5-nano-low /tmp/pinocchio-chatapp-tui run-command /tmp/pin-smoke/tui-multiturn.yaml --output jsonl > /tmp/pin-rpc-final.out 2>/tmp/pin-rpc-final.err
```

Result:

- exit status: `0`
- stdout lines: `14`
- stderr lines: `0`
- all stdout lines parsed as JSON.

TUI multiturn with `PINOCCHIO_PROFILE=gpt-5-mini`:

```bash
tmux new-session -d -s pin-tui-mini2 'cd .../pinocchio && PINOCCHIO_PROFILE=gpt-5-mini /tmp/pinocchio-chatapp-tui run-command /tmp/pin-smoke/tui-multiturn.yaml --chat --force-interactive 2> /tmp/pin-tui-mini2.err; echo $? > /tmp/pin-tui-mini2.status'
```

I first sent a malformed tmux command without `send-keys -l`, which submitted only `Reply with exactly:`. That was useful because it confirmed TAB submission, but it was not a good content test. I then sent a literal follow-up correctly:

```bash
tmux send-keys -t pin-tui-mini2 -l 'Now reply with exactly the token tab_second_ok'
tmux send-keys -t pin-tui-mini2 Tab
```

Captured result:

```text
(user): Now reply with exactly the token tab_second_ok
(assistant): tab_second_ok
profile: gpt-5-mini
```

TUI multiturn with `PINOCCHIO_PROFILE=gpt-5-nano-low`:

```bash
tmux new-session -d -s pin-tui-nano2 'cd .../pinocchio && PINOCCHIO_PROFILE=gpt-5-nano-low /tmp/pinocchio-chatapp-tui run-command /tmp/pin-smoke/tui-multiturn.yaml --chat --force-interactive 2> /tmp/pin-tui-nano2.err; echo $? > /tmp/pin-tui-nano2.status'
tmux send-keys -t pin-tui-nano2 -l 'Now reply with exactly the token nano_second_ok'
tmux send-keys -t pin-tui-nano2 Tab
```

Captured result:

```text
(user): Now reply with exactly the token nano_second_ok
(assistant): nano_second_ok
profile: gpt-5-nano-low
```

Exit status was `0`.

### Automated validation

```bash
go test ./pkg/ui ./pkg/chatapp ./pkg/cmds -count=1
```

Result:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/ui	0.052s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp	0.137s
ok  	github.com/go-go-golems/pinocchio/pkg/cmds	0.183s
```

### What worked

- TAB submission works in tmux for real profiles.
- The new chatapp TUI path supports multiple turns.
- The duplicate user-message rendering bug was fixed by suppressing live user-accepted events in the TUI fanout.
- RPC JSONL still emits clean JSONL after the TUI migration.

### What didn't work

- My first tmux follow-up used `tmux send-keys` without `-l`, and the colon in the prompt caused tmux to submit only part of the string. The fix is to use `tmux send-keys -l 'literal text'` and then a separate `tmux send-keys Tab`.

### What remains

- `StepChatForwardFunc` still exists because `cmd/switch-profiles-tui` uses it. It is no longer used by Pinocchio command chat mode. Removing it completely requires either deleting or migrating that separate helper command.
- The rich profile-switch overlay was removed from the command chat path in favor of the direct chatapp/sessionstream path. The selected `PINOCCHIO_PROFILE` still applies and is shown in a simple status line.

### Code review instructions

- Review `pkg/ui/chatapp_backend.go` for multi-turn state reconstruction from `sessionstream.Snapshot`.
- Review `pkg/cmds/cmd.go` for the new direct chatapp TUI path.
- Review `pkg/chatapp/runtime_inference.go` for the fallback assistant offset fix.
- Run `go test ./pkg/ui ./pkg/chatapp ./pkg/cmds -count=1`.
- Optionally repeat the tmux TAB smoke test with `tmux send-keys -l` followed by `Tab`.

### Commit attempt note

The first `git commit -m "ui: route command chat through chatapp"` attempt failed in the pre-commit lint stage. The full test suite passed, but `golangci-lint` reported three newly-unused helpers left behind by removing the raw/profile-switch command chat path:

```text
pkg/cmds/profile_switch_events.go:13:6: func systemNoticeEntityCmd is unused
pkg/cmds/profile_switch_events.go:33:6: func publishProfileSwitchedInfo is unused
pkg/cmds/seed_emit.go:11:6: func emitSeedTurnToProgram is unused
```

I removed `pkg/cmds/profile_switch_events.go` and `pkg/cmds/seed_emit.go` rather than keeping wrappers/compatibility helpers. Targeted validation after removal passed:

```text
ok  	github.com/go-go-golems/pinocchio/pkg/cmds	0.112s
ok  	github.com/go-go-golems/pinocchio/pkg/ui	0.045s
ok  	github.com/go-go-golems/pinocchio/pkg/chatapp	0.074s
```

## Step 19: Remove switch-profiles TUI and remaining raw simple-chat forwarder

I removed the standalone `cmd/switch-profiles-tui` helper and the associated profile-switch package/scripts. Since the command chat path now runs through chatapp/sessionstream, these files were only keeping the old raw TUI forwarding stack alive.

### Prompt Context

**User prompt (verbatim):** `remove swithc-profiles-tui`

**Assistant interpretation:** Remove the misspelled-but-clear `switch-profiles-tui` command and related raw/profile-switching support code.

**Inferred user intent:** Finish deleting the old TUI/profile-switch compatibility path rather than preserving a separate legacy executable.

**Commit (code):** pending — deletion slice is ready for commit.

### What I removed

- `cmd/switch-profiles-tui/`
- `pkg/ui/profileswitch/`
- switch-profile TUI smoke/verification scripts:
  - `scripts/switch-profiles-tui-*.sh`
  - `scripts/pinocchio-chat-profile-switch-*.sh`
- raw simple-chat TUI backend/forwarder:
  - `pkg/ui/backend.go`
  - `pkg/ui/backend_test.go`

### What I updated

- Removed references to `profileswitch`, `switch-profiles-tui`, `StepChatForwardFunc`, and `EngineBackend` from docs.
- Updated profile-resolution docs so they describe profile resolution/baseline handling, not runtime TUI profile switching.
- Updated TUI integration docs to point simple chat users at the chatapp/sessionstream backend and fanout.
- Updated the run-context comment so `Profile` is described as selected-profile display metadata, not an interactive switching input.

### Validation

First `go test ./... -count=1` failed because `pkg/ui/backend_test.go` still referenced `backendWaitResultMsg` after deleting `pkg/ui/backend.go`:

```text
pkg/ui/backend_test.go:12:9: undefined: backendWaitResultMsg
pkg/ui/backend_test.go:24:9: undefined: backendWaitResultMsg
```

I deleted `pkg/ui/backend_test.go` too. Full validation then passed:

```bash
go test ./... -count=1
```

Result: all packages passed.

### What worked

- After deletion, repository-wide search no longer finds `StepChatForwardFunc`, `EngineBackend`, `NewEngineBackend`, `profileswitch`, or `switch-profiles-tui` under source/docs/scripts outside the ticket history.
- Full Go tests pass.

### What should be reviewed

- Confirm that dropping runtime TUI profile switching is acceptable now that command chat uses the selected launch profile only.
- Confirm whether any external documentation outside this repo mentions `switch-profiles-tui` and should be cleaned later.
