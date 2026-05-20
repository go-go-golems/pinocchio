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
    - Path: cmd/web-chat/app/server.go
      Note: Investigation source for working sessionstream chatapp integration
    - Path: pkg/chatapp/runtime_sink.go
      Note: Investigation identified existing Geppetto-to-chat event mapping
    - Path: pkg/cmds/cmd.go
      Note: Primary investigation target for runtime output handler selection
    - Path: pkg/cmds/cmdlayers/helpers.go
      Note: Primary investigation target for public CLI flags
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
