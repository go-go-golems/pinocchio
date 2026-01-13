---
Title: Geppetto/Pinocchio docs gap analysis
Ticket: MO-001-GEPPETTO-PINOCCHIO-DOCS
Status: active
Topics:
    - documentation
    - geppetto
    - pinocchio
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/embeddings/embeddings.go
      Note: Provider interface includes GenerateBatchEmbeddings (doc gap).
    - Path: geppetto/pkg/embeddings/settings_factory.go
      Note: Cache type uses file and default cache directory; docs say disk.
    - Path: geppetto/pkg/events/chat-events.go
      Note: Full event taxonomy beyond docs.
    - Path: geppetto/pkg/inference/toolhelpers/helpers.go
      Note: RunToolCallingLoop now Turn-based.
    - Path: geppetto/pkg/steps/ai/openai/engine_openai.go
      Note: Tools pulled from context and Turn.Data; ConfigureTools not present.
    - Path: geppetto/pkg/steps/ai/settings/settings-chat.go
      Note: WrapWithCache removed; step-based caching docs outdated.
ExternalSources: []
Summary: Gap analysis of geppetto/pkg/doc against source code and developer personas.
LastUpdated: 2026-01-13T08:26:33.161586425-05:00
WhatFor: Plan documentation updates by mapping doc coverage to current APIs and workflows.
WhenToUse: Use when prioritizing doc fixes, onboarding material, or workflow refreshes for Geppetto and Pinocchio.
---


# Geppetto/Pinocchio docs gap analysis

## Scope and sources

This analysis covers the documentation under `geppetto/pkg/doc` (topics and tutorials) and cross-checks it against the current Go source in `geppetto/pkg`, plus entrypoints in `pinocchio/cmd`. The goal is to identify gaps, outdated guidance, and missing workflows, then interpret the result through multiple developer personas.

Docs reviewed:
- `geppetto/pkg/doc/topics/01-profiles.md`
- `geppetto/pkg/doc/topics/02-emrichen-embeddings.md`
- `geppetto/pkg/doc/topics/03-caching.md`
- `geppetto/pkg/doc/topics/04-events.md`
- `geppetto/pkg/doc/topics/05-conversation.md`
- `geppetto/pkg/doc/topics/06-embeddings.md`
- `geppetto/pkg/doc/topics/06-inference-engines.md`
- `geppetto/pkg/doc/topics/07-tools.md`
- `geppetto/pkg/doc/topics/08-turns.md`
- `geppetto/pkg/doc/topics/09-middlewares.md`
- `geppetto/pkg/doc/topics/10-turn-blocks-serialization.md`
- `geppetto/pkg/doc/topics/11-structured-data-event-sinks.md`
- `geppetto/pkg/doc/topics/12-turnsdatalint.md`
- `geppetto/pkg/doc/tutorials/01-streaming-inference-with-tools.md`

Primary source packages referenced:
- `geppetto/pkg/embeddings`
- `geppetto/pkg/inference/engine`, `geppetto/pkg/inference/toolhelpers`, `geppetto/pkg/inference/tools`, `geppetto/pkg/inference/toolcontext`
- `geppetto/pkg/events`
- `geppetto/pkg/turns`
- `geppetto/pkg/steps/ai/settings`
- `geppetto/pkg/layers`
- `glazed/pkg/config`, `glazed/pkg/cli`

## High-priority gaps (summary)

- Tool calling docs reference a non-existent `engine.ToolsConfigurable` and `ConfigureTools` API and use outdated helper signatures; current tool config is per-Turn + context registry. See `geppetto/pkg/doc/topics/06-inference-engines.md`, `geppetto/pkg/doc/topics/07-tools.md`, `geppetto/pkg/doc/tutorials/01-streaming-inference-with-tools.md`, and `geppetto/pkg/inference/toolhelpers.RunToolCallingLoop`.
- Caching docs reference removed APIs (`ChatSettings.WrapWithCache`, `StandardStepFactory`, `chat.NewChatStep`) and the wrong cache type string for embeddings (`disk` vs `file`). See `geppetto/pkg/doc/topics/03-caching.md`, `geppetto/pkg/steps/ai/settings/settings-chat.go`, and `geppetto/pkg/embeddings/settings_factory.go`.
- Embeddings guide omits the `GenerateBatchEmbeddings` method from the `embeddings.Provider` interface, and references a `geppetto/pkg/llm` package that does not exist. See `geppetto/pkg/doc/topics/06-embeddings.md` and `geppetto/pkg/embeddings/embeddings.go`.
- Event docs list only a subset of the event types and omit execution-stage and UI-focused events (tool-call-execute, log/info, citations, search progress). See `geppetto/pkg/doc/topics/04-events.md` and `geppetto/pkg/events/chat-events.go`.
- Turn serialization docs show a `version` field that does not exist in `turns.Turn`. See `geppetto/pkg/doc/topics/10-turn-blocks-serialization.md` and `geppetto/pkg/turns/serde`.

## Doc-to-code alignment by file

### `geppetto/pkg/doc/topics/01-profiles.md`

This doc mostly matches the current Pinocchio profile flow and config search order, but it skips a few implementation details that are important for debugging profile selection.

- Doc sections touched: "Configuring Profiles", "Selecting a Profile", "Using the Environment Variable", "Using the Command Line Flag", "Setting in config.yaml", "Debugging".
- Related APIs: `glazed/pkg/config.ResolveAppConfigPath`, `glazed/pkg/cli.ProfileSettings`, `geppetto/pkg/layers.GetCobraCommandGeppettoMiddlewares`, `middlewares.GatherFlagsFromProfiles`.
- Missing or out of date:
  - The bootstrap profile selection flow (config + env + flags to resolve profile file/name before `GatherFlagsFromProfiles`) is not described. This is key for explaining why profile selection works and where it can fail.
  - The doc does not mention the `pinocchio profiles` CLI helpers (list, get, set, edit). These are the fastest onboarding path for new users.
- Improvements:
  - Add a short "Profile resolution flow" section describing the two-phase parse and precedence.
  - Add a CLI snippet showing `pinocchio profiles list` and `pinocchio profiles edit` for onboarding.

### `geppetto/pkg/doc/topics/02-emrichen-embeddings.md`

The tag function description is close to the current implementation, but a few defaults and environment details are out of sync.

- Doc sections touched: "Basic Usage", "Configuration Options", "Provider Types", "Configuration Parameters", "Environment Variables".
- Related APIs: `embeddings.SettingsFactory.GetEmbeddingTagFunc`, `embeddings.SettingsFactory.NewProvider`, `embeddings.WithType`, `embeddings.WithEngine`, `embeddings.WithDimensions`, `embeddings.WithBaseURL`.
- Missing or out of date:
  - The doc says Ollama requires explicit `dimensions`; the code provides a default (384) when missing.
  - The doc suggests `OLLAMA_API_KEY`, but the Ollama provider does not read any API key in `geppetto/pkg/embeddings/ollama.go`.
- Improvements:
  - Clarify that `dimensions` is optional for Ollama and defaults to 384 unless overridden.
  - Remove or qualify `OLLAMA_API_KEY` unless a future provider update uses it.
  - Cross-link to `geppetto/pkg/doc/topics/06-embeddings.md` for caching and batch behavior.

### `geppetto/pkg/doc/topics/03-caching.md`

This file is materially out of date relative to the removal of the old step-based caching helpers.

- Doc sections touched: "Programmatic Usage", "Chat Caching", "Using with StandardStepFactory".
- Related APIs: `embeddings.NewCachedProvider`, `embeddings.NewDiskCacheProvider`, `embeddings.SettingsFactory.NewProvider`, `geppetto/pkg/embeddings/settings_factory.go`, `geppetto/pkg/steps/ai/settings/settings-chat.go`.
- Missing or out of date:
  - `ChatSettings.WrapWithCache` is removed (commented out in code).
  - `chat.NewChatStep` and `ai.StandardStepFactory` are no longer present.
  - Embeddings cache type is `file` in `embeddings.SettingsFactory` and flags, not `disk`.
  - Disk cache default directory is `~/.geppetto/cache/embeddings/<model>`, not system temp.
  - There is no engine-level caching middleware documented for chat; the doc suggests it exists.
- Improvements:
  - Split caching docs into "Embeddings caching (current)" and "Chat caching (deprecated or planned)".
  - Update the cache type string and defaults to match `embeddings/settings_factory.go` and `embeddings/config/flags/embeddings.yaml`.

### `geppetto/pkg/doc/topics/04-events.md`

Event flow description is strong, but the event taxonomy is incomplete for current consumers.

- Doc sections touched: "Event Model", "Event Type Cheat Sheet", "Publishing Events", "Running the Event Router", "Client-side Consumption Patterns".
- Related APIs: `events.EventType*`, `events.NewEventFromJson`, `events.EventRouter`, `events.NewStructuredPrinter`, `events.WithEventSinks`, `events.PublishEventToContext`, `middleware.NewWatermillSink`.
- Missing or out of date:
  - Event list does not mention `tool-call-execute` / `tool-call-execution-result` (tool execution stage), `log`/`info`, and built-in progress events (web search, file search, image gen, citations).
  - No brief mention of `events.EventSink` interface and where sinks are used beyond the router.
- Improvements:
  - Add a "Full event catalog" or link to `geppetto/pkg/events/chat-events.go` with a table of event types and payload shapes.
  - Include short guidance on which events are UI-facing vs internal.

### `geppetto/pkg/doc/topics/05-conversation.md`

Core concepts are accurate, but the tutorial focus can be improved for Turn-based workflows.

- Doc sections touched: "Core Concepts", "Managing Conversations with the Manager", "Saving and Loading".
- Related APIs: `conversation.NewManager`, `conversation.Manager.AppendMessages`, `conversation.WithAutosave`, `conversation.SaveToFile`, `conversation.NewChatMessage`, `conversation.NewImageContentFromFile`.
- Missing or out of date:
  - The docs do not mention `conversation/builder` (used in Turn-based examples) or how to bridge conversations to Turns.
  - There is little guidance on tree traversal or how to access non-left-most branches.
- Improvements:
  - Add a short "Conversation <-> Turn bridging" section referencing `turns.BlocksFromConversationDelta` and `turns.BuildConversationFromTurn`.

### `geppetto/pkg/doc/topics/06-embeddings.md`

A comprehensive guide, but it lags the current `Provider` interface and includes a dead package reference.

- Doc sections touched: "Provider Interface", "Implementing Caching Strategies", "Settings and Configuration", "Using Embeddings with LLM Agents".
- Related APIs: `embeddings.Provider`, `embeddings.NewCachedProvider`, `embeddings.NewDiskCacheProvider`, `embeddings.NewSettingsFactoryFromParsedLayers`, `embeddings.NewSettingsFactoryFromStepSettings`.
- Missing or out of date:
  - The `Provider` interface now includes `GenerateBatchEmbeddings`.
  - The section "Using Embeddings with LLM Agents" references `geppetto/pkg/llm`, which is not in the repo.
  - Cache type and default directory mismatch (see caching doc notes).
- Improvements:
  - Update the interface definition and add a "Batch embeddings" section covering `GenerateBatchEmbeddings`, `DefaultGenerateBatchEmbeddings`, `ParallelGenerateBatchEmbeddings`.
  - Remove or update the LLM integration section with actual current interfaces.

### `geppetto/pkg/doc/topics/06-inference-engines.md`

The architectural narrative is solid, but several code examples use APIs that no longer exist.

- Doc sections touched: "Basic Inference Without Tools", "Tool Calling with Helpers", "Manual Tool Calling", "Complete Tool Calling Example".
- Related APIs: `engine.Engine.RunInference`, `turns.Turn`, `turns.BlocksFromConversationDelta`, `turns.BuildConversationFromTurn`, `toolhelpers.RunToolCallingLoop`, `toolcontext.WithRegistry`, `turns.DataKeyToolConfig`.
- Missing or out of date:
  - Examples call `engine.RunInference(ctx, conversation)` but the engine only accepts `*turns.Turn`.
  - The helper loop now takes a `*turns.Turn` and returns `*turns.Turn`, not a conversation slice.
  - The doc references `engine.ToolsConfigurable` and `ConfigureTools`, which do not exist in code; OpenAI engine uses context registry + Turn.Data config.
- Improvements:
  - Rewrite tool calling sections to be fully Turn-based and show the per-Turn tool config workflow.

### `geppetto/pkg/doc/topics/07-tools.md`

The intent aligns with Turn-based tools, but the helper-route example is outdated.

- Doc sections touched: "Quickstart", "Helper route", "Guided walkthrough", "Tool executors and lifecycle hooks".
- Related APIs: `toolcontext.WithRegistry`, `tools.NewToolFromFunc`, `toolhelpers.RunToolCallingLoop`, `turns.DataKeyToolConfig`, `tools.BaseToolExecutor`.
- Missing or out of date:
  - The helper route still shows `toolhelpers.RunToolCallingLoop(ctx, e, initialConversation, ...)` rather than `*turns.Turn`.
- Improvements:
  - Add a Turn-based helper example and point at `toolblocks.ExtractPendingToolCalls` for lower-level usage.

### `geppetto/pkg/doc/topics/08-turns.md`

This topic is mostly correct but omits key block kinds and payload constants.

- Doc sections touched: "Types", "Helpers", "Engine mapping", "Tool workflow".
- Related APIs: `turns.BlockKindReasoning`, `turns.PayloadKeyEncryptedContent`, `turns.PayloadKeyItemID`, `turns.DataKeyResponsesServerTools`.
- Missing or out of date:
  - The type list omits `reasoning` blocks and related payload keys.
  - The example that references `tools.NewInMemoryToolRegistry()` is missing the import.
- Improvements:
  - Update the block kind list, add payload constants, and link to `turns/keys.go`.

### `geppetto/pkg/doc/topics/09-middlewares.md`

Mostly aligned. The main improvement is clarification on the two different tool config types.

- Doc sections touched: "Core interfaces", "Example: Tool middleware".
- Related APIs: `middleware.ToolConfig`, `tools.ToolConfig`.
- Missing or out of date:
  - The doc does not clarify that `middleware.ToolConfig` is separate from `tools.ToolConfig` used by tool executors.
- Improvements:
  - Add a short callout showing where each config lives and which layers consume them.

### `geppetto/pkg/doc/topics/10-turn-blocks-serialization.md`

Serde guidance is useful, but the example schema includes a field that does not exist.

- Doc sections touched: "Data Model", "Examples", "Using the serde helpers".
- Related APIs: `turns.Turn`, `turns.Block`, `turns/serde.ToYAML`, `turns/serde.FromYAML`.
- Missing or out of date:
  - Examples include `version: 1`, but `turns.Turn` has no `Version` field.
- Improvements:
  - Either add `Version` to the Turn schema or remove it from examples and explain compatibility expectations.

### `geppetto/pkg/doc/topics/11-structured-data-event-sinks.md`

Aligned with code, and in good shape. It could use a short note on how to attach a filtering sink to engine event sinks.

- Doc sections touched: "Public API and Types", "Wiring Everything Together".
- Related APIs: `structuredsink.NewFilteringSink`, `structuredsink.NewFilteringSinkWithContext`, `events.EventSink`.
- Improvement:
  - Add a short snippet showing `engine.WithSink(structuredsink.NewFilteringSink(...))` to close the loop with the event system.

### `geppetto/pkg/doc/topics/12-turnsdatalint.md`

Aligned with code. No immediate accuracy problems found.

- Doc sections touched: "What it enforces", "How it works".
- Related APIs: `turnsdatalint.Analyzer`.

### `geppetto/pkg/doc/tutorials/01-streaming-inference-with-tools.md`

The workflow intent is strong but the example APIs are out of date.

- Doc sections touched: "Key APIs You'll Use", "Step 3 - Create the Engine", "Step 4 - Register a Tool", "Step 6 - Run the Router and Tool-Calling Loop".
- Related APIs: `engine.Engine`, `events.WithEventSinks`, `toolcontext.WithRegistry`, `toolhelpers.RunToolCallingLoop`.
- Missing or out of date:
  - `engine.ToolsConfigurable` and `ConfigureTools` are referenced but not implemented; OpenAI engine reads tools from context + Turn.Data.
  - The helper loop takes a `*turns.Turn`, not a conversation slice.
  - The tutorial introduces `pinocchio-profile` as a custom flag; the actual built-in flag is `--profile` from `glazed/pkg/cli`.
- Improvements:
  - Update the tutorial to build a `turns.Turn`, attach tool config via `turns.DataKeyToolConfig`, and use the real `--profile` flag.

## Persona analysis - new developer onboarding

### Session 1: "How do I run a streaming tool call?"

The developer starts with `geppetto/pkg/doc/topics/06-inference-engines.md` and sees the overall architecture. The doc suggests running `engine.RunInference(ctx, conversation)` in the "Manual Tool Calling" section, so they try to wire a `conversation.Conversation` directly into the engine. The compiler rejects this since `RunInference` accepts `*turns.Turn`. They then search for `RunToolCallingLoop` and find it expects a `*turns.Turn` rather than a conversation slice.

- Docs consulted: "Manual Tool Calling", "Automated Tool Calling Loop".
- What they expected: a copy-pasteable Turn-based example, or an explicit conversion step.
- Current friction: mismatch between examples and signatures; missing `ToolsConfigurable` interface in code.
- Needed doc fix: a minimal, correct Turn-based tool calling example with `toolcontext.WithRegistry` and `turns.DataKeyToolConfig`.

### Session 2: "How do caching settings work?"

They attempt to reduce API costs by enabling caching. `geppetto/pkg/doc/topics/03-caching.md` instructs them to call `ChatSettings.WrapWithCache` and use `StandardStepFactory`, but those APIs are gone. They then try to set `--embeddings-cache-type=disk` and see that the CLI expects `file` for embeddings caching.

- Docs consulted: "Programmatic Usage", "Chat Caching".
- What they expected: a working example using current engines or a clear deprecation note.
- Current friction: non-existent APIs and wrong cache type string.
- Needed doc fix: update to embeddings-only caching, clarify cache types and default directories.

### Session 3: "Where do profiles and configs live?"

They start with the profiles doc and it describes config search order correctly, but does not mention the `pinocchio profiles` commands that would help bootstrap a `profiles.yaml`. They also do not understand why their profile is not being applied until they discover the two-phase parsing in `geppetto/pkg/layers`.

- Docs consulted: "Configuring Profiles", "Selecting a Profile".
- What they expected: a quick CLI-first entrypoint and a short diagram of precedence.
- Needed doc fix: add CLI helpers and a short profile-resolution flow diagram.

## Persona analysis - active programmer using Pinocchio and Geppetto

### Session 1: "I need to refresh tool execution hooks"

They want to inject auth into tool calls and look for the executor hooks. `geppetto/pkg/doc/topics/07-tools.md` has a good hook example, but it does not mention where tool execution emits events (`tool-call-execute` and `tool-call-execution-result`). They end up reading `tools/base_executor.go` to find the exact hook names and event types.

- Docs consulted: "Tool executors and lifecycle hooks".
- Needed doc fix: add a small hook-to-event mapping and explain how to use `events.WithEventSinks` for execution events.

### Session 2: "My UI is missing events"

They implement a UI that listens to `EventTypeToolCall` and `EventTypeToolResult` as documented. Later they see no UI updates for execution or progress events such as web search or citations. The event taxonomy in `geppetto/pkg/doc/topics/04-events.md` is incomplete.

- Docs consulted: "Event Type Cheat Sheet".
- Needed doc fix: provide a full event catalog or a table of public vs internal event types.

### Session 3: "I need a test fixture for Turns"

They open `geppetto/pkg/doc/topics/10-turn-blocks-serialization.md` and use the YAML example with `version: 1`. Their test loader ignores `version`, which makes them question whether the schema is correct. They then inspect `turns/serde` to confirm the actual structure.

- Docs consulted: "Examples", "Using the serde helpers".
- Needed doc fix: remove or implement a version field and clearly document serde options.

### Session 4: "Batch embeddings for semantic search"

They want to build a semantic search loop and look for batch embeddings. The embeddings guide does not mention `GenerateBatchEmbeddings` or `ParallelGenerateBatchEmbeddings`. They end up reading `geppetto/pkg/embeddings/batch.go` directly.

- Docs consulted: "Provider Interface", "Practical Applications".
- Needed doc fix: add a batch embeddings section and guidance on when to use `GenerateBatchEmbeddings`.

## Technical writer perspective

- Information architecture: topics and tutorials are mixed without a clear landing index. There are two topic files with the same numeric prefix (`06-embeddings.md` and `06-inference-engines.md`), which weakens navigation and order.
- Consistency: several docs claim Turn-based architecture but include conversation-based code. This is an obvious drift signal that should be fixed and likely requires a doc-wide audit for Turn vs conversation references.
- Taxonomy: some docs are labeled "Tutorial" but read more like reference (for example `05-conversation.md`). Consider normalizing SectionType or splitting reference vs walkthrough content.
- Cross-linking: direct links to `geppetto/cmd/examples/*` are rare. Those examples are the best source of truth and should be linked from each topic.
- Accuracy debt: caching, tool calling, and embeddings are particularly stale. These should be treated as P0 fixes because they lead to compile errors and user confusion.

## Suggested documentation upgrade plan (actionable)

P0 - correctness fixes (blocking):
- Rewrite tool calling sections in `06-inference-engines.md`, `07-tools.md`, and `01-streaming-inference-with-tools.md` to be fully Turn-based and remove `ToolsConfigurable`/`ConfigureTools`.
- Update `03-caching.md` to remove step-based APIs and to use the correct cache type (`file`) and default directory for embeddings caching.
- Update `06-embeddings.md` to include `GenerateBatchEmbeddings` and remove references to `geppetto/pkg/llm`.
- Expand `04-events.md` to include execution and progress events, or link to a full event catalog.
- Fix `10-turn-blocks-serialization.md` to align the YAML examples with the `turns.Turn` schema.

P1 - onboarding and workflow clarity:
- Add a Pinocchio profile CLI section in `01-profiles.md` and a short profile-resolution flow diagram.
- Add a "Turn <-> Conversation" bridge section in `05-conversation.md` and cross-link to Turns docs.
- Add a short "Tool registry and Turn.Data" callout in `07-tools.md` with a minimal recipe.

P2 - structure and polish:
- Create a top-level index topic that routes readers by task: streaming, tools, embeddings, events, turns, middleware.
- Normalize numbering and SectionType metadata.
- Add a short "current API status" or "deprecated" callout for removed Step-based APIs to prevent drift.
