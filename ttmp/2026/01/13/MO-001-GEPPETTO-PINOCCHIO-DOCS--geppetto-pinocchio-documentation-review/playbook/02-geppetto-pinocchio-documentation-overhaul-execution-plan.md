---
Title: Geppetto/Pinocchio documentation overhaul execution plan
Ticket: MO-001-GEPPETTO-PINOCCHIO-DOCS
Status: active
Topics:
    - documentation
    - geppetto
    - pinocchio
DocType: playbook
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/ttmp/2026/01/13/MO-001-GEPPETTO-PINOCCHIO-DOCS--geppetto-pinocchio-documentation-review/analysis/01-geppetto-pinocchio-docs-gap-analysis.md
      Note: Gap analysis that this plan operationalizes.
ExternalSources: []
Summary: Concrete, doc-by-doc plan for updating Geppetto/Pinocchio documentation.
LastUpdated: 2026-01-13T09:07:00-05:00
WhatFor: Hand-off plan for a ghostwriter to update docs accurately.
WhenToUse: Use when executing the doc overhaul for geppetto/pkg/doc.
---


# Geppetto/Pinocchio documentation overhaul execution plan

## Purpose

Provide a concrete, doc-by-doc execution plan for rewriting or consolidating Geppetto and Pinocchio documentation. This is meant to be handed to a ghostwriter and executed as a checklist.

## Environment assumptions

- Local repo checkout with `geppetto/` and `pinocchio/` directories.
- Ability to run `rg` and open source files locally.
- Ability to run example commands where validation is required.

## Scope

Documentation under:
- `geppetto/pkg/doc/topics/`
- `geppetto/pkg/doc/tutorials/`

This plan supersedes the earlier general playbook at `playbook/01-documentation-research-and-writing-plan.md`.

## Deliverables

- Updated or consolidated docs listed in this plan.
- One new doc index under `geppetto/pkg/doc/topics/`.
- Optional redirect stubs where a doc is deleted or consolidated.

## Doc set decisions (consolidate/delete)

- Consolidate caching guidance into `geppetto/pkg/doc/topics/06-embeddings.md` and remove or replace `03-caching.md` with a short redirect stub.
- Add a new docs index page in `geppetto/pkg/doc/topics/00-docs-index.md` to route by task.
- Keep `12-turnsdatalint.md` unchanged (no corrections needed).
- Merge `10-turn-blocks-serialization.md` into `08-turns.md` and delete or replace it with a short redirect stub.
- Avoid documenting deprecated APIs or legacy-only flows; remove or minimize conversation-only walkthroughs unless they are still required by current code paths.

## Execution plan by document

### 1) `geppetto/pkg/doc/topics/00-docs-index.md` (new)

Action: Create new doc index.

Content requirements:
- Short intro: what Geppetto docs cover.
- Task-based navigation: streaming, tools, turns, embeddings, events, profiles, middleware.
- Links to example programs in `geppetto/cmd/examples/`.

Sources to read:
- Existing docs list under `geppetto/pkg/doc/topics/`
- Example programs under `geppetto/cmd/examples/`

Validation:
- All links resolve to existing files.

### 2) `geppetto/pkg/doc/topics/01-profiles.md` (update)

Action: Update existing doc.

Required changes:
- Add a "Profile resolution flow" subsection that explains bootstrap parsing (config + env + flags -> profile selection -> profiles loaded).
- Add CLI examples for `pinocchio profiles list`, `pinocchio profiles edit`, and `pinocchio profiles init`.
- Confirm config search order matches `glazed/pkg/config.ResolveAppConfigPath`.

Sources to read:
- `geppetto/pkg/layers/layers.go`
- `glazed/pkg/config/resolve.go`
- `pinocchio/cmd/pinocchio/main.go` (profile init guidance)

Validation:
- Examples use `--profile` (not custom flags).

### 3) `geppetto/pkg/doc/topics/02-emrichen-embeddings.md` (update)

Action: Update existing doc.

Required changes:
- Clarify Ollama `dimensions` defaults to 384 if omitted.
- Remove or qualify `OLLAMA_API_KEY` usage.
- Add a short note linking to embeddings caching and batch embeddings sections in `06-embeddings.md`.

Sources to read:
- `geppetto/pkg/embeddings/ollama.go`
- `geppetto/pkg/embeddings/settings_factory.go`

Validation:
- Example config matches `GetEmbeddingTagFunc` options (`type`, `engine`, `dimensions`, `base_url`, `api_key`).

### 4) `geppetto/pkg/doc/topics/03-caching.md` (replace or retire)

Action: Replace with a short redirect stub or remove if the doc index does not need it.

Redirect stub contents (if kept):
- One paragraph: "Caching for embeddings lives in the embeddings guide" with link to `06-embeddings.md`.
- Note that chat caching is deprecated or not implemented.

Sources to read:
- `geppetto/pkg/embeddings/settings_factory.go`
- `geppetto/pkg/steps/ai/settings/settings-chat.go` (WrapWithCache removed)

Validation:
- No references to `ChatSettings.WrapWithCache`, `StandardStepFactory`, or `chat.NewChatStep` remain.

### 5) `geppetto/pkg/doc/topics/04-events.md` (update)

Action: Update event catalog and usage examples.

Required changes:
- Add a "Full event catalog" table or bullet list with current types from `events/chat-events.go`.
- Include execution-stage events (`tool-call-execute`, `tool-call-execution-result`) and log/info events.
- Treat all emitted events as chat events; do not split into UI vs internal categories.
- Add a short "EventSink usage" paragraph (engine.WithSink + context sinks).

Sources to read:
- `geppetto/pkg/events/chat-events.go`
- `geppetto/pkg/events/printer.go`
- `geppetto/pkg/events/event-router.go`

Validation:
- Event names and payload fields match code.

### 6) `geppetto/pkg/doc/topics/05-conversation.md` (decision point)

Action: Decide whether to retire or sharply reduce this doc.

Required changes (if kept):
- Remove legacy/step-based workflows and any deprecated APIs.
- Keep only the minimum required API surface that is still used in current code (for example image helpers or conversion utilities).
- Add a short "Conversation <-> Turn bridging" subsection that points to the canonical Turn-based flow.

Sources to read:
- `pinocchio/pkg/cmds/images.go` (image helper usage)
- `geppetto/pkg/inference/toolhelpers/helpers.go` (conversation-based helpers still present)
- `geppetto/pkg/turns/conv_conversation.go` (bridge helpers)

Validation:
- If the doc is retired, ensure the new index does not link to it.
- If kept, examples must reflect only current, non-deprecated usage.

### 7) `geppetto/pkg/doc/topics/06-embeddings.md` (update + consolidate)

Action: Update and include caching section from `03-caching.md`.

Required changes:
- Update `Provider` interface to include `GenerateBatchEmbeddings`.
- Add a "Batch embeddings" section with `DefaultGenerateBatchEmbeddings` and `ParallelGenerateBatchEmbeddings` usage.
- Include embeddings caching (memory + disk) with cache type `file`, and default cache directory location.
- Remove references to `geppetto/pkg/llm` (non-existent).

Sources to read:
- `geppetto/pkg/embeddings/embeddings.go`
- `geppetto/pkg/embeddings/batch.go`
- `geppetto/pkg/embeddings/cache.go`
- `geppetto/pkg/embeddings/disk-cache.go`
- `geppetto/pkg/embeddings/settings_factory.go`

Validation:
- Cache type string `file` matches `settings_factory.go`.
- Default cache dir uses `~/.geppetto/cache/embeddings/<model>`.

### 8) `geppetto/pkg/doc/topics/06-inference-engines.md` (rewrite sections)

Action: Rewrite examples to Turn-based APIs.

Required changes:
- Replace conversation-based `RunInference` examples with Turn-based examples.
- Update tool-calling sections to use `toolcontext.WithRegistry` + `turns.DataKeyToolConfig` + `toolhelpers.RunToolCallingLoop` (Turn-based).
- Remove `engine.ToolsConfigurable` and `ConfigureTools` references.

Sources to read:
- `geppetto/pkg/inference/engine/*`
- `geppetto/pkg/inference/toolhelpers/helpers.go`
- `geppetto/pkg/turns/*`
- `geppetto/pkg/steps/ai/openai/engine_openai.go`

Validation:
- Examples compile against `engine.Engine.RunInference(ctx, *turns.Turn)`.

### 9) `geppetto/pkg/doc/topics/07-tools.md` (update)

Action: Update helper route to Turn-based tool loop.

Required changes:
- Replace `RunToolCallingLoop` signature to accept `*turns.Turn`.
- Add explicit note: tool registry in `context.Context`, tool config in `Turn.Data`.

Sources to read:
- `geppetto/pkg/inference/toolhelpers/helpers.go`
- `geppetto/pkg/inference/toolcontext/toolcontext.go`
- `geppetto/pkg/turns/keys.go`

Validation:
- Example uses `turns.DataKeyToolConfig`.

### 10) `geppetto/pkg/doc/topics/08-turns.md` (update)

Action: Expand the data model section.

Required changes:
- Add `reasoning` block kind and payload keys (`encrypted_content`, `item_id`).
- Link to `turns/keys.go` for typed constants.
- Add a "Serialization (YAML)" subsection merged from `10-turn-blocks-serialization.md` (data model, serde helpers, fixtures).

Sources to read:
- `geppetto/pkg/turns/types.go`
- `geppetto/pkg/turns/keys.go`
- `geppetto/pkg/turns/serde/serde.go`

Validation:
- Examples match current constants and payload keys.

### 11) `geppetto/pkg/doc/topics/09-middlewares.md` (update)

Action: Clarify tool config types and usage.

Required changes:
- Add a callout differentiating `middleware.ToolConfig` vs `tools.ToolConfig`.
- Note that tool execution config for Turn-based helpers uses `turns.DataKeyToolConfig`.

Sources to read:
- `geppetto/pkg/inference/middleware/tool_middleware.go`
- `geppetto/pkg/inference/tools/config.go`

Validation:
- Config types and fields match code.

### 12) `geppetto/pkg/doc/topics/10-turn-blocks-serialization.md` (merge/retire)

Action: Merge content into `08-turns.md`, then delete or replace with a short redirect stub.

Required changes:
- Move serde examples and YAML schema into the "Serialization (YAML)" subsection of `08-turns.md`.
- If a stub remains, remove `version` from YAML examples and link to the merged section.

Sources to read:
- `geppetto/pkg/turns/serde/serde.go`

Validation:
- No duplicate serialization content across two docs.

### 13) `geppetto/pkg/doc/topics/11-structured-data-event-sinks.md` (update)

Action: Add sink wiring snippet.

Required changes:
- Add a short example using `engine.WithSink(structuredsink.NewFilteringSink(...))`.

Sources to read:
- `geppetto/pkg/events/structuredsink/filtering_sink.go`
- `geppetto/pkg/inference/engine/options.go`

Validation:
- Example uses correct imports and sink type.

### 14) `geppetto/pkg/doc/topics/12-turnsdatalint.md` (no change)

Action: Leave as-is.

Validation:
- N/A

### 15) `geppetto/pkg/doc/tutorials/01-streaming-inference-with-tools.md` (rewrite sections)

Action: Rewrite to Turn-based workflow.

Required changes:
- Replace conversation-based loop with Turn-based `toolhelpers.RunToolCallingLoop`.
- Remove `engine.ToolsConfigurable` references.
- Ensure profile usage uses `--profile` from `glazed/pkg/cli`.

Sources to read:
- `geppetto/pkg/inference/toolhelpers/helpers.go`
- `geppetto/pkg/turns/*`
- `geppetto/pkg/steps/ai/openai/engine_openai.go`

Validation:
- Example compiles and uses current types.

## Commands

Use these commands during the doc updates.

```bash
# Find references to obsolete APIs
rg "WrapWithCache|StandardStepFactory|ToolsConfigurable|ConfigureTools" geppetto/pkg/doc

# Verify Turn-based helpers
rg "RunToolCallingLoop" geppetto/pkg/inference/toolhelpers

# Locate event types
rg "EventType" geppetto/pkg/events/chat-events.go

# Locate examples
rg --files geppetto/cmd/examples
```

## Exit criteria

- All docs in the plan are updated or replaced as specified.
- No doc references removed APIs (WrapWithCache, StandardStepFactory, ToolsConfigurable).
- New doc index exists and links resolve.
- All updated examples map to real code paths and can be verified.

## Failure modes and mitigations

- If an example cannot be made to compile, mark it explicitly as pseudocode and link to a working example program.
- If a doc is removed, leave a redirect stub or ensure the index does not reference it.
