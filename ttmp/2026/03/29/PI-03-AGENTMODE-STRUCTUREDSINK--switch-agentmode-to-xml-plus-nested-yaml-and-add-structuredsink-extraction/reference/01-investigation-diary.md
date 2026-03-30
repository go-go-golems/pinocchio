---
Title: Investigation diary
Ticket: PI-03-AGENTMODE-STRUCTUREDSINK
Status: active
Topics:
    - pinocchio
    - structured-sinks
    - webchat
    - yaml
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/doc/topics/11-structured-sinks.md
      Note: Primary structuredsink documentation reviewed
    - Path: pinocchio/pkg/doc/topics/webchat-sem-and-ui.md
      Note: Primary web-chat SEM documentation reviewed
    - Path: pinocchio/pkg/middlewares/agentmode/middleware.go
      Note: Primary investigation target
    - Path: sanitize/README.md
      Note: Primary sanitize library documentation reviewed
ExternalSources: []
Summary: Chronological diary for the PI-03 investigation covering agentmode, structuredsink, web-chat SEM, and sanitize-backed YAML parsing.
LastUpdated: 2026-03-29T16:28:55.586034123-04:00
WhatFor: Record how the design was derived, which files and docs were inspected, what conclusions were evidence-backed, and how to continue or review the work.
WhenToUse: Use when implementing or reviewing the PI-03 design or when resuming the investigation later.
---


# Investigation diary

## Goal

Capture the investigation that led to the PI-03 design: where `agentmode` lives, how it currently parses YAML, where structured sinks fit into the stack, whether Pinocchio web-chat already supports the relevant SEM path, and how `sanitize/pkg/yaml` should be integrated.

## Step 1: Investigate current agentmode, structuredsink, and web-chat integration

This step was a documentation and architecture pass, not an implementation pass. The goal was to produce a ticket package that is concrete enough for a new intern to follow without reverse-engineering the codebase. The key question was not just "can this be done?" but "which layer should own each part of the work so we do not create duplicate parsers or duplicate UI events."

The investigation established that the requested direction is viable and that most of the needed architectural seams already exist. `agentmode` already owns policy and persistence. Geppetto already owns the generic structured streaming machinery. Pinocchio web-chat already exposes an event-sink wrapper seam and already translates `EventAgentModeSwitch` into `agent.mode` SEM frames. The missing work is the protocol migration and the agentmode-specific extractor and parser sharing.

### Prompt Context

**User prompt (verbatim):** "Create a new ticket to update the agentmode middleware to use the XML approach with nested yaml, and provide a structuredsink in addition to the middleware (update its yaml parsing too), and integrate that sturctured sink into SEM in pinocchio web-chat (if it isn't alr4eady). 

Create a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file
  references.
  It should be very clear and detailed. Store in the ticket and the nupload to remarkable.

Properly write up which doucmentation you found to do that."

**Assistant interpretation:** Create a new Pinocchio docmgr ticket, investigate the current `agentmode` middleware and the structured streaming infrastructure, design the migration to an XML-like structured tag with nested YAML plus sanitize-backed parsing, explain how to wire the extractor into web-chat and SEM, and upload the finished document bundle to reMarkable.

**Inferred user intent:** Replace ad hoc fenced-YAML control parsing with a more explicit, reusable, and stream-friendly protocol while producing durable onboarding documentation for an unfamiliar engineer.

**Commit (code):** N/A — no application code was changed in this step.

### What I did

- Ran `docmgr status --summary-only` to confirm docmgr root and vocabulary.
- Read the workflow and formatting guidance from:
  - `/home/manuel/.codex/skills/ticket-research-docmgr-remarkable/SKILL.md`
  - `/home/manuel/.codex/skills/docmgr/SKILL.md`
  - `/home/manuel/.codex/skills/remarkable-upload/SKILL.md`
  - `/home/manuel/.codex/skills/diary/SKILL.md`
- Inspected the current `agentmode` middleware:
  - `nl -ba pinocchio/pkg/middlewares/agentmode/middleware.go | sed -n '1,360p'`
  - `nl -ba pinocchio/pkg/middlewares/agentmode/service.go | sed -n '1,220p'`
- Inspected the current structured sink contracts:
  - `nl -ba geppetto/pkg/events/structuredsink/filtering_sink.go | sed -n '1,320p'`
  - `nl -ba geppetto/pkg/events/structuredsink/parsehelpers/helpers.go | sed -n '1,320p'`
- Inspected the parse helper used by `agentmode`:
  - `nl -ba geppetto/pkg/steps/parse/yaml_blocks.go | sed -n '1,220p'`
- Inspected Pinocchio web-chat integration seams:
  - `nl -ba pinocchio/pkg/webchat/router_options.go | sed -n '110,145p'`
  - `nl -ba pinocchio/pkg/webchat/router.go | sed -n '292,316p'`
  - `nl -ba pinocchio/pkg/webchat/sem_translator.go | sed -n '470,520p'`
  - `nl -ba pinocchio/cmd/web-chat/web/src/sem/registry.ts | sed -n '220,255p'`
- Inspected the existing event type and docs:
  - `nl -ba geppetto/pkg/events/chat-events.go | sed -n '860,890p'`
  - `nl -ba pinocchio/pkg/doc/topics/webchat-sem-and-ui.md | sed -n '60,90p'`
  - `nl -ba pinocchio/pkg/doc/topics/webchat-profile-registry.md | sed -n '160,190p'`
  - `nl -ba pinocchio/pkg/doc/tutorials/01-building-a-middleware-with-renderer.md | sed -n '20,40p'`
  - `nl -ba geppetto/pkg/doc/topics/11-structured-sinks.md | sed -n '96,140p'`
  - `nl -ba geppetto/pkg/doc/playbooks/03-progressive-structured-data.md | sed -n '236,266p'`
  - `nl -ba geppetto/pkg/doc/tutorials/04-structured-data-extraction.md | sed -n '190,240p'`
- Located the reusable sanitize library and inspected its public API:
  - `nl -ba sanitize/README.md | sed -n '60,95p'`
  - `nl -ba sanitize/pkg/yaml/sanitize.go | sed -n '1,240p'`
  - `nl -ba sanitize/pkg/yaml/options.go | sed -n '1,220p'`
- Created the new ticket workspace and document stubs:
  - `docmgr ticket create-ticket --root pinocchio/ttmp --ticket PI-03-AGENTMODE-STRUCTUREDSINK --title "Switch agentmode to XML plus nested YAML and add structuredsink extraction" --topics pinocchio,structured-sinks,webchat,yaml`
  - `docmgr doc add --root pinocchio/ttmp --ticket PI-03-AGENTMODE-STRUCTUREDSINK --doc-type design-doc --title "Intern guide to switching agentmode to XML plus nested YAML and structuredsink extraction"`
  - `docmgr doc add --root pinocchio/ttmp --ticket PI-03-AGENTMODE-STRUCTUREDSINK --doc-type reference --title "Investigation diary"`

### Why

- The request explicitly asked for a new ticket and a detailed design package, so the work needed to be evidence-first and docmgr-backed.
- `agentmode` touches prompting, runtime state, event sinks, and web-chat UI. A shallow document would not be enough for an intern.
- The user had already corrected the earlier YAML-sanitization guidance to prefer `sanitize/`, so the parser design needed to anchor to the actual sanitize package rather than generic YAML helpers.

### What worked

- The codebase already had a clean architectural seam for web-chat sink wrapping via `WithEventSinkWrapper(...)`.
- The existing `agent.mode` SEM mapping means part of the requested integration is already present and could be documented precisely rather than guessed.
- The sanitize library exposes a straightforward public API, so the design can recommend a concrete call path instead of inventing one.
- The existing Geppetto structured-sink docs are strong enough to support a detailed intern guide with real APIs and real wiring examples.

### What didn't work

- I initially tried to open `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/events/events_agent_mode.go` and got:

```text
nl: /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/events/events_agent_mode.go: No such file or directory
```

  The event definition actually lives in `geppetto/pkg/events/chat-events.go`.

- I also initially tried `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/steps/parse/parse.go` and got:

```text
nl: /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/steps/parse/parse.go: No such file or directory
```

  The relevant helper is in `geppetto/pkg/steps/parse/yaml_blocks.go`.

These misses were harmless, but they are worth recording because they explain why the references list points to `chat-events.go` and `yaml_blocks.go`.

### What I learned

- `agentmode` is currently a post-inference parser and state mutator, not a streaming extractor.
- Web-chat already supports sink wrapping at the router layer. This is the intended insertion point for a filtering sink.
- The current SEM path already covers final agent mode switch events.
- The main unresolved design choice is whether extractor-driven UI should use the same event type as the final middleware switch or a distinct proposal event.

### What was tricky to build

- The sharp edge in this design is not the parser itself. The hard part is preserving ownership boundaries. It is tempting to move all logic into the structured extractor once the streaming path exists, but that would overload the extractor with durable state responsibilities that the middleware and service currently own.
- Another subtle point is event duplication. If the structured extractor emits the same final switch event that the middleware emits after applying the new mode, the UI will likely show duplicate cards. That is why the guide recommends either a distinct proposal event or a sink that filters and parses without emitting a final duplicate.
- The user's "XML approach" wording could easily be misread as "use a general XML parser." In this codebase, the correct interpretation is the Geppetto structured tag format used by `FilteringSink`.

### What warrants a second pair of eyes

- The final event semantics: proposal versus applied switch.
- Whether a temporary fallback for legacy free-form fenced YAML is needed during rollout.
- Whether sanitizing malformed YAML before decode could hide provider quality issues that the team actually wants surfaced in debug tooling.
- Whether the simple chat agent should be updated in the same ticket or only documented as a follow-up integration.

### What should be done in the future

- After implementation, add a small follow-up doc or playbook showing a concrete prompt/response transcript with the new structured tag.
- Consider a second ticket if the team wants richer partial proposal updates in the UI rather than final-only extraction.

### Code review instructions

- Start with the design doc in this ticket, then verify the current code paths:
  - `pinocchio/pkg/middlewares/agentmode/middleware.go`
  - `geppetto/pkg/events/structuredsink/filtering_sink.go`
  - `pinocchio/pkg/webchat/router_options.go`
  - `pinocchio/pkg/webchat/router.go`
  - `pinocchio/pkg/webchat/sem_translator.go`
  - `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
  - `sanitize/pkg/yaml/sanitize.go`
- Validate the claims by rerunning the `nl -ba` and `rg -n` commands listed above.
- After implementation, review test coverage for:
  - sanitize-backed parser correctness,
  - structured extractor lifecycle,
  - web-chat sink wrapper behavior,
  - SEM translation for any new event types.

### Technical details

Quick current-state contract summary:

```go
// Current middleware parsing pattern
blocks, err := parse.ExtractYAMLBlocks(txt)
if err != nil { continue }
if err := yaml.Unmarshal([]byte(body), &data); err != nil { continue }

// Proposed shared parser pattern
parsed, err := ParseModeSwitchPayload(raw)
// internal sequence:
// StripCodeFenceBytes -> yamlsanitize.Sanitize -> yaml.Unmarshal -> semantic validation
```

Recommended structured payload shape:

```text
<pinocchio:agent_mode_switch:v1>
```yaml
mode_switch:
  analysis: |
    ...
  new_mode: category_regexp_reviewer
```
</pinocchio:agent_mode_switch:v1>
```

Key documentation found during the investigation:

- `geppetto/pkg/doc/topics/11-structured-sinks.md`
- `geppetto/pkg/doc/tutorials/04-structured-data-extraction.md`
- `geppetto/pkg/doc/playbooks/03-progressive-structured-data.md`
- `pinocchio/pkg/doc/tutorials/01-building-a-middleware-with-renderer.md`
- `pinocchio/pkg/doc/topics/webchat-sem-and-ui.md`
- `pinocchio/pkg/doc/topics/webchat-profile-registry.md`
- `sanitize/README.md`

## Step 2: Implement the shared sanitize-backed parser and migrate middleware

This step turned the design into code by introducing a real protocol and parser layer inside `agentmode`. The intent was to stop letting the middleware perform its own raw fenced-YAML scan and instead make it call a shared parser that both the middleware and the future structured sink could rely on.

The key architectural constraint stayed intact: the middleware still owns the final state transition. The parser is shared, but only the middleware updates `turns.KeyAgentMode`, records the mode change, and appends the final system notice.

### Prompt Context

**User prompt (verbatim):** "alright, add to the implementation/design document, then add detailed tasks to the ticket, and implement task by task, committing at appropriate intervals, keeping a detailed diary."

**Assistant interpretation:** Update the ticket package first, then implement the migration in reviewable phases with commits and a continuation-friendly diary.

**Inferred user intent:** Convert the ticket from analysis into a real implementation while keeping the documentation strong enough for another engineer to resume or review.

**Commit (code):** `af71a503e2c62c11e50a5c431c50711dc695c4c7` — `Refactor agentmode mode-switch parsing`

### What I did

- Added `pinocchio/pkg/middlewares/agentmode/protocol.go` with:
  - structured tag constants for `<pinocchio:agent_mode_switch:v1>`
  - `ParseOptions` with `sanitize_yaml` optional and defaulting to `true`
  - `BuildModeSwitchInstructions(...)`
  - compatibility wrapper `BuildYamlModeSwitchInstructions(...)`
- Added `pinocchio/pkg/middlewares/agentmode/parser.go` with:
  - `ParseModeSwitchPayload(...)`
  - `FindModeSwitchPayloadInText(...)`
  - `DetectModeSwitch(...)`
  - `DetectModeSwitchInBlocks(...)`
  - compatibility wrappers for the old `DetectYamlModeSwitch*` names
- Updated `pinocchio/pkg/middlewares/agentmode/middleware.go` to:
  - carry `ParseOptions` in config
  - use `BuildModeSwitchInstructions(...)`
  - replace old `parse.ExtractYAMLBlocks(...)` plus direct `yaml.Unmarshal` logic with `DetectModeSwitchInBlocks(...)`
- Added `pinocchio/pkg/middlewares/agentmode/middleware_test.go` covering:
  - structured instruction output
  - sanitize-on parse success
  - sanitize-off parse failure
  - structured-tag detection
  - authoritative final mode application
  - compatibility wrapper behavior
- Updated `pinocchio/go.mod` to require `github.com/go-go-golems/sanitize` and point it at the sibling workspace module with:
  - `replace github.com/go-go-golems/sanitize => ../sanitize`
- Ran:
  - `gofmt -w /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/middlewares/agentmode/protocol.go /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/middlewares/agentmode/parser.go /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/middlewares/agentmode/middleware.go /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/middlewares/agentmode/middleware_test.go`
  - `cd /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio && go test ./pkg/middlewares/agentmode -count=1`
- Committed the phase after the package test passed and the repo pre-commit hook completed.

### Why

- The user explicitly wanted middleware and structuredsink parsing unified.
- `sanitize/` had to be optional but on by default, which required a reusable options carrier instead of one-off call sites.
- Keeping the parser local to `agentmode` avoids pushing app-specific mode semantics into generic Geppetto helpers.

### What worked

- The parser shape mapped cleanly onto `parsehelpers.StripCodeFenceBytes(...)` plus `yamlsanitize.Sanitize(...)`.
- The malformed YAML used in tests was repairable when sanitize was enabled and failed when sanitize was disabled, which gives a sharp regression signal.
- The middleware migration did not require changing the persistence boundary or event semantics.

### What didn't work

- The first test run failed before reaching parser logic because the module dependency was incomplete:

```text
cd /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio && go test ./pkg/middlewares/agentmode -count=1
...
github.com/go-go-golems/sanitize@v0.0.0: reading github.com/go-go-golems/sanitize/go.mod at revision v0.0.0: unknown revision v0.0.0
```

- The fix was:

```text
cd /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio && go mod edit -replace=github.com/go-go-golems/sanitize=../sanitize
```

After that, `go test ./pkg/middlewares/agentmode -count=1` passed.

### What I learned

- The workspace `go.work` membership was not enough on its own once `pinocchio/go.mod` explicitly required a placeholder version.
- The compatibility wrappers were cheap to keep and let the migration stay incremental without preserving the old parsing logic.
- The most stable way to detect the structured payload in completed text was a simple `LastIndex` search for the closing tag and its matching opening tag.

### What was tricky to build

- The subtle part was not the YAML decoding itself. The tricky part was preserving the authority boundary while changing the protocol. The middleware still had to be the only place that mutates `turns.KeyAgentMode` and records the mode change, even though parsing moved out of the middleware body.
- Another small sharp edge was deciding how to represent the sanitize toggle. A positive `SanitizeYAML` option with defaulting logic is clearer than an inverted `DisableSanitize` flag once the option is exposed in runtime config and docs.

### What warrants a second pair of eyes

- Whether the compatibility wrappers should remain for longer than this ticket or be removed in a follow-up cleanup.
- Whether `ParseClean` should continue to be computed as `ParseClean && LintClean` or if lint cleanliness is too strict for future UI uses.

### What should be done in the future

- If more agentmode payload types are added later, keep them behind the same protocol/parser package boundary instead of adding more ad hoc YAML parsing.

### Code review instructions

- Start with:
  - `pinocchio/pkg/middlewares/agentmode/protocol.go`
  - `pinocchio/pkg/middlewares/agentmode/parser.go`
  - `pinocchio/pkg/middlewares/agentmode/middleware.go`
  - `pinocchio/pkg/middlewares/agentmode/middleware_test.go`
- Validate with:
  - `cd /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio && go test ./pkg/middlewares/agentmode -count=1`
- Review commit:
  - `git show af71a503e2c62c11e50a5c431c50711dc695c4c7`

### Technical details

- Structured tag constants:

```text
<pinocchio:agent_mode_switch:v1>
...
</pinocchio:agent_mode_switch:v1>
```

- Shared parser flow:

```go
ParseModeSwitchPayload(raw, opts):
    StripCodeFenceBytes(raw)
    Trim whitespace
    Optionally yamlsanitize.Sanitize(...)
    yaml.Unmarshal(...)
    semantic validation of analysis/new_mode
```

## Step 3: Add the structured sink, web-chat wiring, and logging cleanup

This step completed the runtime integration. The sink now filters the structured control payload out of user-visible streamed text, while the middleware remains the authoritative owner of the final mode switch. That split matches the design goal of adding progressive structured extraction without making streaming callbacks mutate durable state.

This step also exposed the sanitize toggle in Pinocchio web-chat runtime middleware config and folded in the noisy SEM ingress log reduction from debug to trace. That logging tweak is small, but it is part of the same web-chat runtime path and had been explicitly requested earlier.

### Prompt Context

**User prompt (verbatim):** (same as Step 2)

**Assistant interpretation:** Finish the runtime integration phase by wiring the structured sink into web-chat, updating runtime config, and committing the result separately from the parser migration.

**Inferred user intent:** Make the new protocol actually affect the running web-chat path while keeping the changes reviewable in discrete commits.

**Commit (code):** `ec658b51981c63cba1ffcf044242f4b08fbf70c7` — `Add agentmode structured sink to web chat`

### What I did

- Added `pinocchio/pkg/middlewares/agentmode/structured_extractor.go` with:
  - `ModeSwitchExtractor`
  - `ExtractorConfig`
  - `StructuredSinkConfig`
  - `WrapStructuredSink(...)`
- Added `pinocchio/cmd/web-chat/agentmode_sink_wrapper.go` with:
  - `agentModeStructuredSinkConfigFromRuntime(...)`
  - `newAgentModeStructuredSinkWrapper()`
- Updated `pinocchio/cmd/web-chat/main.go` to install the wrapper via `webchat.WithEventSinkWrapper(...)`.
- Updated `pinocchio/cmd/web-chat/middleware_definitions.go` to expose:
  - `sanitize_yaml` in the middleware schema
  - runtime config decoding into `agentmode.Config.ParseOptions`
- Added `pinocchio/cmd/web-chat/agentmode_sink_wrapper_test.go` covering:
  - default runtime config behavior
  - disabled middleware behavior
  - `sanitize_yaml: false` override behavior
  - structured payload filtering through the wrapped sink
- Kept the existing `agent.mode` SEM mapping as-is because no new event type was introduced.
- Changed the ingress log in `pinocchio/pkg/webchat/sem_translator.go` from `Debug` to `Trace`.
- Ran:
  - `gofmt -w /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/middlewares/agentmode/structured_extractor.go /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/agentmode_sink_wrapper.go /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/agentmode_sink_wrapper_test.go /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/middleware_definitions.go /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/main.go /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/webchat/sem_translator.go`
  - `cd /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio && go test ./cmd/web-chat ./pkg/webchat ./pkg/middlewares/agentmode -count=1`
- Committed the phase after the targeted tests passed and the repo pre-commit hook completed.

### Why

- The sink wrapper is the intended place to add progressive structured extraction in web-chat without rewriting router internals.
- Exposing `sanitize_yaml` in runtime config keeps the middleware and sink aligned from the same profile/runtime source of truth.
- Reusing the existing `agent.mode` SEM mapping avoids duplicate event semantics and extra frontend work.

### What worked

- The existing `webchat.WithEventSinkWrapper(...)` seam was exactly sufficient; no router redesign was needed.
- The wrapper test confirmed that the structured control block is filtered out of final visible text.
- The existing SEM mapping was sufficient, so there was no need to introduce a new proposal event in this ticket.

### What didn't work

- The pre-commit hook was slow because it reran the full repo validation stack, including frontend install/build, `golangci-lint`, `go vet`, and `go test ./...`. This was not a correctness failure, but it materially slowed the commit loop.
- The first attempt at the wrapper test referenced a nonexistent exported helper for the tag text. I fixed that by composing the tag string from the exported `ModeSwitchTag*` constants before running tests.

### What I learned

- The simplest sink behavior for this ticket is "filter and parse, but do not emit a second final switch event." That keeps the structured sink additive without creating duplicate `agent.mode` cards.
- Runtime-config inspection in `cmd/web-chat` is a clean place to derive extractor parse options without coupling generic web-chat packages to agentmode internals.

### What was tricky to build

- The main tricky point was deciding how far the extractor should go. It is easy to overreach and let the sink start owning mode changes or duplicate final events. Keeping the extractor parse-only and filter-focused made the responsibilities much clearer.
- Another subtle point was config propagation. The sanitize toggle had to flow through middleware definitions for authoritative parsing and through the sink wrapper for streaming parsing, but both needed to land on the same `ParseOptions` type.

### What warrants a second pair of eyes

- Whether future UX work should add a distinct streaming proposal event instead of remaining filter-only.
- Whether `agentModeStructuredSinkConfigFromRuntime(...)` should eventually live in a reusable runtime helper if more middleware-specific sink wrappers are added.

### What should be done in the future

- If the team later wants streaming mode proposals in the UI, add a distinct proposal event rather than reusing the final switch event.

### Code review instructions

- Start with:
  - `pinocchio/pkg/middlewares/agentmode/structured_extractor.go`
  - `pinocchio/cmd/web-chat/agentmode_sink_wrapper.go`
  - `pinocchio/cmd/web-chat/middleware_definitions.go`
  - `pinocchio/cmd/web-chat/agentmode_sink_wrapper_test.go`
  - `pinocchio/pkg/webchat/sem_translator.go`
- Validate with:
  - `cd /home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio && go test ./cmd/web-chat ./pkg/webchat ./pkg/middlewares/agentmode -count=1`
- Review commit:
  - `git show ec658b51981c63cba1ffcf044242f4b08fbf70c7`

### Technical details

- Web-chat sink installation path:

```go
webchat.WithEventSinkWrapper(newAgentModeStructuredSinkWrapper())
```

- Runtime-to-sink config rule:

```go
if middleware.name == "agentmode" && enabled {
    cfg.ParseOptions = cfg.ParseOptions.WithSanitizeYAML(...)
    return agentmode.WrapStructuredSink(downstream, cfg)
}
```

- The SEM change in this step was intentionally only:

```go
log.Trace().Msg("received event (SEM)")
```

## Quick Reference

### Ticket workspace

- Ticket: `PI-03-AGENTMODE-STRUCTUREDSINK`
- Root: `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/ttmp/2026/03/29/PI-03-AGENTMODE-STRUCTUREDSINK--switch-agentmode-to-xml-plus-nested-yaml-and-add-structuredsink-extraction`

### Primary code references

- `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/middlewares/agentmode/middleware.go`
- `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/events/structuredsink/filtering_sink.go`
- `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/webchat/router.go`
- `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/webchat/sem_translator.go`
- `/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/sanitize/pkg/yaml/sanitize.go`

## Usage Examples

Use this diary when:

- implementing the migration and needing the exact investigation commands,
- reviewing the resulting PR and wanting to understand why the design chose a shared parser and sink wrapper,
- resuming work later and needing a concise reminder of which docs and files were authoritative.

## Related

- [`../design-doc/01-intern-guide-to-switching-agentmode-to-xml-plus-nested-yaml-and-structuredsink-extraction.md`](../design-doc/01-intern-guide-to-switching-agentmode-to-xml-plus-nested-yaml-and-structuredsink-extraction.md)
