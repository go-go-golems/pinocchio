---
Title: Scopeddb TUI demo analysis, design, and intern implementation guide
Ticket: GP-032
Status: active
Topics:
    - pinocchio
    - tui
    - sqlite
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/inference/tools/scopeddb/query.go
      Note: |-
        QueryInput and QueryOutput contracts plus validation rules.
        Scopeddb query contracts and validation
    - Path: geppetto/pkg/inference/tools/scopeddb/schema.go
      Note: |-
        Core dataset builder API including DatasetSpec, BuildResult with Meta, and schema setup helpers.
        Core scopeddb dataset builder API
    - Path: geppetto/pkg/inference/tools/scopeddb/tool.go
      Note: |-
        Tool registration API for prebuilt and lazy scopeddb tools.
        Scopeddb tool registration APIs
    - Path: pinocchio/cmd/examples/simple-chat/main.go
      Note: |-
        Existing example surface to compare against; useful for why a TUI demo needs a different host.
        Example surface inspected and rejected as the main TUI host
    - Path: pinocchio/cmd/switch-profiles-tui/main.go
      Note: |-
        Closest current Pinocchio Bubble Tea application that already uses Bobatea chat, overlays, Watermill routing, and a reusable backend.
        Closest current Pinocchio TUI runtime reference
    - Path: pinocchio/cmd/web-chat/main.go
      Note: |-
        Useful contrast; realistic app-owned tool registration, but too heavy for a small scopeddb demo.
        Webchat surface inspected as a contrast case
    - Path: pinocchio/pkg/middlewares/sqlitetool/middleware.go
      Note: |-
        Older generic sqlite tool middleware; useful contrast with the newer scopeddb package.
        Older generic sqlite middleware used as contrast for scopeddb
    - Path: pinocchio/pkg/ui/backends/toolloop/backend.go
      Note: |-
        Current reusable session-backed tool-loop backend for Bobatea chat UIs.
        Reusable backend recommended for the demo
    - Path: temporal-relationships/ttmp/2026/03/11/MEN-TR-055--remove-tui-surfaces-and-review-simplification-opportunities/design-doc/02-tui-removal-and-post-removal-simplification-analysis-and-intern-implementation-guide.md
      Note: Existing investigation explaining why temporal-relationships removed its old TUI surfaces.
ExternalSources: []
Summary: Recommends a new `pinocchio/cmd/examples/scopeddb-tui-demo` Bubble Tea example that reuses Pinocchio's current TUI primitives and Geppetto's new `scopeddb` package, while borrowing only the good ideas from the removed temporal-relationships TUIs.
LastUpdated: 2026-03-15T18:00:00-04:00
WhatFor: Give a new intern enough architectural context and concrete implementation guidance to add a fake-data scopeddb demo to Pinocchio without reviving old temporal-relationships product debt or inventing a one-off TUI stack.
WhenToUse: Use when deciding where and how to demonstrate `geppetto/pkg/inference/tools/scopeddb` in a terminal UI, especially if you want to learn from the removed temporal-relationships TUIs and keep the result small, teachable, and reusable.
---


# Scopeddb TUI demo analysis, design, and intern implementation guide

## Executive Summary

The best place to add a scoped database demo is a new example binary in Pinocchio, not a production command and not a copy of the removed temporal-relationships TUI. The recommended path is a new Bubble Tea example at `pinocchio/cmd/examples/scopeddb-tui-demo/main.go` built from three already-existing pieces:

1. Pinocchio's reusable Bobatea backend in `pinocchio/pkg/ui/backends/toolloop/backend.go`.
2. Pinocchio's current working TUI reference in `pinocchio/cmd/switch-profiles-tui/main.go`.
3. Geppetto's extracted scoped database package in `geppetto/pkg/inference/tools/scopeddb`.

The removed temporal-relationships TUIs are still valuable, but as design input rather than code to resurrect. They prove two things. First, a Bubble Tea chat UI is a good fit for inspecting query tool calls and tool results. Second, command-local registry bootstrapping and renderer hacks can become product-surface debt if they live in a production application for too long. Pinocchio should keep the good part, which is the demo ergonomics, and avoid the bad part, which is command-specific, app-specific plumbing.

The proposed demo should use fake data, a tiny in-memory dataset, and one or two narrow tools. It should teach a reader how a scoped database tool works, what `Meta` is for, how tool registration happens, and how a TUI can make the SQL interaction legible. The demo should be intentionally small enough that an intern can read the whole thing in one sitting.

## Problem Statement And Scope

The user asked for a TUI example if possible, and specifically asked us to check temporal-relationships history because that repository previously had two TUI examples that were later removed. That means this ticket is not just about "pick a demo host". It is about answering a deeper architectural question:

- Which historical ideas are still useful?
- Which current Pinocchio integration point is the cleanest host?
- What should an intern build if the goal is to demonstrate the new `scopeddb` package rather than ship a new full product surface?

This ticket is intentionally limited to analysis and design. It does not implement the demo. Instead, it produces a detailed guide for a future implementation.

### Goals

- Identify the removed temporal-relationships TUI surfaces and explain what they did well.
- Compare those historical surfaces with current Pinocchio TUI and example entry points.
- Recommend one concrete new demo binary and explain why it is the right host.
- Give an intern a file-by-file implementation plan with APIs, pseudocode, diagrams, and test guidance.

### Non-goals

- Reintroduce a TUI into temporal-relationships.
- Turn Pinocchio's demo into a production-grade app shell.
- Replace Pinocchio's current TUI primitives.
- Replace the older `sqlitetool` middleware across the repo in this ticket.

## Current-State Analysis

This section is evidence-first. Every major recommendation below is tied back to concrete files or historical commits.

### 1. Temporal-relationships used to have two TUI surfaces

The first removed surface was a full `tui` command family. In the pre-removal version of `cmd/temporal-relationships/cmds/tui/agent_chat.go`, the command built a Bobatea chat UI, resolved profile settings, created a Geppetto event router, materialized optional scoped ToolDBs, and then ran a tool-loop backend (`agent_chat.go`, lines 56-72 and 152-188 in the historical version from parent of commit `1b05558`). This was a real interactive developer tool, not just a mock.

The second removed surface was a pure mock timeline debug UI in `cmd/temporal-extract-js/debugtui/main.go`. That program did not connect to the real backend at all. Instead, it emitted synthetic timeline events over time using `tea.Tick`, then let the TUI render those fake entities (`debugtui/main.go`, lines 22-29 and 44-70 in the historical version from parent of commit `b8467b7`). That matters because a fake-data demo is exactly what we want in Pinocchio.

These two deleted examples map cleanly to two demo styles:

- "Real tool-loop app with real tools and custom renderers."
- "Pure fake-data visualization that teaches the UI concepts with almost no infrastructure."

For Pinocchio's scopeddb demo, we want something in between those two extremes:

- Real enough to exercise `scopeddb`.
- Small enough to stay understandable.

### 2. The old temporal agent chat contains reusable lessons

The historical `agent_chat.go` showed one strong pattern that still holds up: using Bobatea as a developer-facing inspection UI for tool-driven runs. The command built a tool-loop backend and passed a renderer registration hook through `boba_chat.WithTimelineRegister(registerAgentChatRenderers)` (`agent_chat.go`, lines 180-185). That is the right conceptual model for a scopeddb demo because the user should be able to see:

- the assistant's question,
- the tool call payload,
- the SQL text,
- the rows returned,
- and the final assistant interpretation.

The historical command also showed a weak pattern that we should not reuse directly. Its `buildScopedToolRegistryFromUnifiedDB` helper owned a lot of app-specific preflight validation, materialization, dumping, and registration logic in one place (`tooldb_registry.go`, lines 34-228). That was appropriate before the extraction, but the new `geppetto/pkg/inference/tools/scopeddb` package exists specifically so that applications do not need to rebuild that pattern from scratch.

### 3. The old temporal renderers are good demo inspiration

The deleted `tool_call_sql_highlight_renderer.go` looked for the historical SQL query tools and rendered the `sql` field from the tool input as fenced SQL (`tool_call_sql_highlight_renderer.go`, lines 126-154). The deleted `tool_call_result_markdown_table_renderer.go` recognized a `{columns, rows, count}` payload and rendered it as a markdown table (`tool_call_result_markdown_table_renderer.go`, lines 130-223).

Those renderers were valuable because they made a tool loop understandable to a human. A raw JSON blob is technically correct but pedagogically weak. For a demo whose entire point is "here is how a scoped query tool works", the UI should present SQL and results in a first-class way.

The recommendation here is not "copy these files verbatim". The recommendation is "reuse the product idea":

- render SQL nicely,
- render query output in a table-like form,
- keep the renderer logic small and demo-local.

### 4. Pinocchio already has the right reusable TUI seam

`pinocchio/cmd/switch-profiles-tui/main.go` is the closest current Pinocchio reference for a TUI scopeddb demo. It already does the real work of wiring Bobatea, Watermill, a reusable backend, and an overlay host. Concretely:

- it creates a Watermill event router and sink (`switch-profiles-tui/main.go`, lines 164-182),
- it constructs a backend for the chat model (`switch-profiles-tui/main.go`, lines 184-198),
- it builds a Bobatea chat model and can intercept slash commands (`switch-profiles-tui/main.go`, lines 199-257),
- it uses overlay and timeline infrastructure from reusable `pkg` code.

The reusable backend underneath that style of UI is `pinocchio/pkg/ui/backends/toolloop/backend.go`. That backend stores a `session.Session`, appends a user turn, starts inference, and emits `BackendFinishedMsg` when the inference run completes (`backend.go`, lines 23-45 and 48-72). This is exactly the primitive a scopeddb demo needs.

### 5. Pinocchio's other surfaces are worse fits

`pinocchio/cmd/examples/simple-chat/main.go` is a useful existing example, but it is not the best host. It demonstrates a simple CLI-oriented chat step and turn printing, not an interactive TUI (`simple-chat/main.go`, lines 78-169). A scopeddb demo in that style would show functionality, but it would not teach the interactive flow of tool calls and results.

`pinocchio/cmd/web-chat/main.go` is also not the right host. It is a realistic app-owned integration surface and it does show an important idea, which is application-owned tool registration, but the command is much larger and pulls in unrelated concerns like HTTP serving, frontend assets, and runtime config (`web-chat/main.go`, lines 1-240 alone already show that breadth). That is too much conceptual load for a scopeddb demo meant for onboarding.

### 6. The new Geppetto scopeddb package is the core abstraction to teach

The extracted package in `geppetto/pkg/inference/tools/scopeddb` now provides the reusable building blocks that the old temporal ToolDB code did not have.

At the schema and materialization level:

- `DatasetSpec[Scope, Meta]` defines the schema SQL, allowed objects, tool metadata, default query options, and a `Materialize` callback (`schema.go`, lines 31-39).
- `BuildResult[Meta]` returns the materialized `*sql.DB`, a `Meta` value, and `Cleanup` (`schema.go`, lines 41-45).
- `BuildInMemory` opens an in-memory SQLite database, applies schema, calls `Materialize`, and returns the handle (`schema.go`, lines 91-115).

At the tool-registration level:

- `RegisterPrebuilt` registers a query tool from an already-built database (`tool.go`, lines 11-35).
- `NewLazyRegistrar` registers a tool whose scoped database is rebuilt per invocation based on a resolved scope (`tool.go`, lines 37-79).

At the query contract level:

- `QueryInput` exposes `sql` and optional `params` (`query.go`, lines 148-151).
- `QueryOutput` returns `columns`, `rows`, `count`, `truncated`, and optional `error` (`query.go`, lines 153-159).
- `validateQuery` enforces read-only query constraints such as single-statement `SELECT` or `WITH` queries (`query.go`, lines 305-320).

This is the package the demo should center around. The demo is not "SQLite in a TUI". The demo is "how an application defines a scoped dataset spec and exposes it as a safe query tool".

### 7. The older Pinocchio sqlite middleware is useful as a contrast case

`pinocchio/pkg/middlewares/sqlitetool/middleware.go` registers a generic `sql_query` tool by reading schema and optional prompts out of an attached database (`middleware.go`, lines 44-101 and 126-190). That middleware is useful context, but it demonstrates a different philosophy:

- the database is attached from outside,
- schema help is dumped from the runtime DB,
- tool registration happens inside middleware,
- and the query contract is more generic.

The new `scopeddb` package is more structured:

- the app defines the schema contract up front,
- the app defines a scope type,
- the app owns materialization,
- the tool description is generated from a dataset spec,
- and the query runner enforces tighter scope and object boundaries.

For an intern, this contrast is instructive. The demo should mention the older middleware only to explain why the extracted package exists.

## Recommendation

Add a new example binary:

```text
pinocchio/cmd/examples/scopeddb-tui-demo/main.go
```

Use Pinocchio's current TUI primitives and Geppetto's `scopeddb` package. Do not add the demo to `cmd/web-chat`. Do not revive a top-level production `tui` command family. Do not copy the deleted temporal files into Pinocchio.

### Why this is the best choice

- It is close to the real TUI path used in Pinocchio today.
- It keeps demo code out of production commands.
- It is easy to read, run, and delete or evolve later.
- It can safely borrow the nice renderer ideas from the removed temporal TUI without taking on that application's historical baggage.

## Proposed Demo Architecture

### High-level flow

```text
User question in TUI
  |
  v
Bobatea chat model
  |
  v
Pinocchio reusable tool-loop backend
  |
  v
Geppetto engine + tool registry
  |
  +--> scopeddb query tool
          |
          v
      BuildInMemory(spec, scope)
          |
          v
      materialize fake dataset into SQLite
          |
          v
      QueryRunner validates and runs SELECT query
          |
          v
      QueryOutput {columns, rows, count, truncated}
  |
  v
Event router forwards tool call + tool result to TUI
  |
  v
Custom renderers show SQL and tabular rows
```

### Recommended demo shape

The demo should have one narrow teaching goal: show how to define a scoped dataset and make it queryable by an LLM inside a TUI.

A good fake-data domain would be something with obvious scoping, such as:

- project issue tracker snapshots,
- bookstore inventory by store,
- support tickets by customer account,
- meetings and action items by workspace.

The easiest option is probably "customer support tickets by account". It gives us:

- a clear scope key like `AccountID`,
- obvious tables,
- intuitive starter questions,
- and useful `Meta` like row counts and resolved account label.

### Recommended data model

Example SQLite schema for the demo:

```sql
CREATE TABLE accounts (
  account_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  plan TEXT NOT NULL
);

CREATE TABLE tickets (
  ticket_id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  priority TEXT NOT NULL,
  opened_at TEXT NOT NULL
);

CREATE TABLE ticket_events (
  event_id TEXT PRIMARY KEY,
  ticket_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at TEXT NOT NULL
);
```

Allowed objects for the tool should likely be:

- `accounts`
- `tickets`
- `ticket_events`

### Why keep `Meta` in the demo

The user explicitly asked to keep `Meta`, and the demo is a good place to justify that decision. `Meta` is not needed to execute a query, but it is useful for everything around the query:

- status messages in the TUI,
- debugging output,
- preload summaries,
- scope labels,
- counts shown to the reader,
- and optional "seed info" panels.

For the demo, `Meta` could be:

```go
type DemoMeta struct {
    AccountID      string
    AccountName    string
    TicketCount    int
    EventCount     int
    GeneratedSeed  string
}
```

That gives the application something meaningful to log or display when the scoped database is built.

## API Reference For The Intern

This section names the important types and explains them in plain language.

### `scopeddb.DatasetSpec[Scope, Meta]`

Defined in `geppetto/pkg/inference/tools/scopeddb/schema.go`.

Purpose:

- describes one logical scoped dataset,
- names the tool,
- defines the schema contract,
- tells the package how to materialize rows.

Fields to understand:

- `InMemoryPrefix`: prefix for the SQLite in-memory DSN.
- `SchemaLabel`: human-readable name used in schema setup errors.
- `SchemaSQL`: SQL statements used to initialize the database.
- `AllowedObjects`: tables/views the query runner will allow.
- `Tool`: user/model-facing tool definition.
- `DefaultQuery`: limits such as timeout and row cap.
- `Materialize`: callback that inserts rows for a particular scope.

### `scopeddb.BuildResult[Meta]`

Defined in `geppetto/pkg/inference/tools/scopeddb/schema.go`.

Purpose:

- returns the built SQLite handle,
- returns `Meta`,
- returns a cleanup function.

Why the intern should care:

- this is how app-owned code gets both the database and extra build information,
- and it shows why `Meta` exists at all.

### `scopeddb.QueryInput`

Defined in `geppetto/pkg/inference/tools/scopeddb/query.go`.

Contract:

```go
type QueryInput struct {
    SQL    string `json:"sql"`
    Params []any  `json:"params,omitempty"`
}
```

Teaching point:

- the tool input schema is intentionally small,
- the model gets one SQL string plus optional bind parameters,
- query safety comes from the validation and authorizer layer, not from an oversized prompt schema.

### `scopeddb.QueryOutput`

Defined in `geppetto/pkg/inference/tools/scopeddb/query.go`.

Contract:

```go
type QueryOutput struct {
    Columns   []string
    Rows      []map[string]any
    Count     int
    Truncated bool
    Error     string
}
```

Teaching point:

- this shape is ideal for a demo renderer because it is already table-like.

### `scopeddb.RegisterPrebuilt` vs `scopeddb.NewLazyRegistrar`

Defined in `geppetto/pkg/inference/tools/scopeddb/tool.go`.

Use `RegisterPrebuilt` when:

- the app already built the database,
- the scope is fixed for the life of the app,
- or you want to access `Meta` at startup.

Use `NewLazyRegistrar` when:

- the scope depends on request or session context,
- the tool should rebuild its scoped snapshot on each call,
- or you want the narrowest possible lifetime for the in-memory database.

For the first version of the demo, `RegisterPrebuilt` is simpler and better for teaching. The demo can build the database once at startup, log or display its `Meta`, and then register the tool.

## Proposed File Layout

The new example should stay small and explicit. A reasonable structure is:

```text
pinocchio/cmd/examples/scopeddb-tui-demo/
  main.go
  fake_data.go
  dataset.go
  renderers.go
  README.md
```

### Responsibility split

- `main.go`
  - CLI flags
  - build fake dataset
  - register tool
  - build event router and TUI
  - run program

- `dataset.go`
  - demo `Scope` type
  - demo `Meta` type
  - `scopeddb.DatasetSpec`
  - `Materialize` callback

- `fake_data.go`
  - deterministic seed generator or literal fixtures
  - helper functions that produce rows

- `renderers.go`
  - SQL-focused tool call renderer
  - markdown-table or simple table result renderer

- `README.md`
  - how to run the demo
  - example prompts
  - architecture summary

## Detailed Design

### 1. Scope definition

The demo needs a small scope type that is easy to understand.

Example:

```go
type DemoScope struct {
    AccountID string
}
```

The demo can accept `--account acme-co` as a CLI flag and use that to choose which fake dataset to materialize.

### 2. Materialization strategy

The `Materialize` callback should not read from an external DB. It should insert fake rows into the provided SQLite handle. That keeps the example reproducible and avoids setup work for the intern or the reader.

Pseudocode:

```go
func materializeDemo(ctx context.Context, dst *sql.DB, scope DemoScope) (DemoMeta, error) {
    account, tickets, events := buildFakeFixtures(scope.AccountID)

    insert account row
    insert each ticket row
    insert each ticket event row

    return DemoMeta{
        AccountID: account.ID,
        AccountName: account.Name,
        TicketCount: len(tickets),
        EventCount: len(events),
        GeneratedSeed: "literal-fixtures-v1",
    }, nil
}
```

### 3. Tool definition strategy

The tool should have a narrow name and a helpful summary. The model-facing description should explain:

- what the dataset contains,
- which tables are available,
- what kinds of questions it is good at answering,
- and one or two starter queries.

Example sketch:

```go
Tool: scopeddb.ToolDefinitionSpec{
    Name: "query_support_history",
    Description: scopeddb.ToolDescription{
        Summary: "Query a scoped read-only SQLite snapshot of support tickets and events for one customer account.",
        StarterQueries: []string{
            "SELECT ticket_id, title, status FROM tickets ORDER BY opened_at DESC LIMIT 5",
            "SELECT event_type, created_at, body FROM ticket_events WHERE ticket_id = ? ORDER BY created_at",
        },
        Notes: []string{
            "Use joins between tickets and ticket_events when you need ticket context.",
            "Use ORDER BY for stable result ordering.",
        },
    },
    Tags: []string{"sqlite", "scopeddb", "demo"},
    Version: "1.0.0",
}
```

### 4. TUI host strategy

Base the demo on the `switch-profiles-tui` pattern, but strip it down aggressively. The demo does not need:

- profile switching,
- persistent chat stores,
- overlay forms unless they directly help the scopeddb story,
- multiple app modes.

It does need:

- a Watermill router,
- an event sink,
- a reusable tool-loop backend,
- a Bobatea chat model,
- renderer registration,
- a small system prompt.

### 5. Renderer strategy

The renderer goal is explanation, not production polish.

Recommended renderers:

- tool call renderer:
  - if the tool is the scopeddb demo tool, extract `sql` from the JSON input and render it as fenced SQL;
  - otherwise fall back to plain text or Bobatea defaults.

- tool result renderer:
  - if the payload decodes as `QueryOutput`, render a compact table or markdown table;
  - if there is an `Error`, render that clearly;
  - otherwise fall back to raw output.

This is where the deleted temporal renderers are valuable inspiration.

### 6. Prompting strategy

Keep the system prompt very small. The demo should not rely on giant prompt scaffolding.

Example:

```text
You are a support-ops assistant. Use the query_support_history tool when the user asks
for account-specific ticket or event history. Prefer short SQL queries, explicit ORDER BY,
and explain your findings after reading the tool output.
```

### 7. Startup UX

Use `Meta` during startup so the intern can see why it exists.

For example, on startup the app can show a plain timeline item or a status line like:

```text
Loaded scoped snapshot for account Acme Co: 6 tickets, 18 events.
```

That is concrete, useful, and hard to do elegantly if the builder returns only `*sql.DB`.

## Implementation Guide

This section is written for an intern who may not know the codebase yet.

### Phase 1. Read the reference code

Before writing anything, read these files in this order:

1. `geppetto/pkg/inference/tools/scopeddb/schema.go`
2. `geppetto/pkg/inference/tools/scopeddb/tool.go`
3. `geppetto/pkg/inference/tools/scopeddb/query.go`
4. `pinocchio/pkg/ui/backends/toolloop/backend.go`
5. `pinocchio/cmd/switch-profiles-tui/main.go`
6. Historical reference only:
   - the deleted `temporal-relationships` `agent_chat.go`
   - the deleted `tool_call_sql_highlight_renderer.go`
   - the deleted `tool_call_result_markdown_table_renderer.go`
   - the deleted `debugtui/main.go`

Goal of this reading pass:

- understand the scopeddb package API,
- understand how a Pinocchio TUI starts and listens to events,
- understand how the old temporal demo made tool calls readable.

### Phase 2. Create the demo dataset

Create `dataset.go` and define:

- `DemoScope`
- `DemoMeta`
- SQL schema string
- dataset spec
- materializer

Checklist:

- keep the schema tiny,
- use deterministic fake data,
- return meaningful `Meta`,
- keep `AllowedObjects` explicit.

### Phase 3. Build the DB and register the tool

In `main.go`:

1. Parse `--account`.
2. Build the scoped DB with `scopeddb.BuildInMemory(...)`.
3. Save `Meta` for startup display.
4. Create an in-memory tool registry.
5. Register the tool with `scopeddb.RegisterPrebuilt(...)`.

Pseudocode:

```go
scope := DemoScope{AccountID: accountFlag}
build, err := scopeddb.BuildInMemory(ctx, demoDatasetSpec(), scope)
if err != nil { return err }
defer build.Cleanup()

reg := tools.NewInMemoryToolRegistry()
if err := scopeddb.RegisterPrebuilt(reg, demoDatasetSpec(), build.DB, demoDatasetSpec().DefaultQuery); err != nil {
    return err
}
```

### Phase 4. Build the TUI runtime

In `main.go`:

1. create a Watermill-backed event router,
2. create a sink,
3. build the engine from step settings,
4. create the reusable backend,
5. build the Bobatea model,
6. register forwarders,
7. run the Bubble Tea program.

Pseudocode:

```go
router := events.NewEventRouter(...)
sink := middleware.NewWatermillSink(router.Publisher, "chat")
backend := toolloopbackend.NewToolLoopBackend(engine, middlewares, reg, sink, nil)

model := boba_chat.InitialModel(
    backend,
    boba_chat.WithTitle("scopeddb demo"),
    boba_chat.WithTimelineRegister(registerDemoRenderers),
)

p := tea.NewProgram(model, tea.WithAltScreen())
router.AddHandler("ui-forward", "chat", agentforwarder.MakeUIForwarder(p))
```

If the demo can reuse an existing forwarder directly, do that. If not, create the smallest possible demo-local forwarder or renderer additions instead of inventing a new framework.

### Phase 5. Make the SQL visible

Add a small renderer file that:

- detects the demo tool name,
- parses `QueryInput`,
- renders SQL in a readable code block.

Also add a result renderer that:

- detects `QueryOutput`,
- renders a compact table,
- highlights `Error` if present.

This phase matters because without it the demo will technically work but will not teach much.

### Phase 6. Add a tiny README

The example should be runnable with one command and explain what the reader is looking at.

Minimum README sections:

- what the demo is,
- how to run it,
- example prompts,
- which files matter,
- how `Meta` is used.

## Suggested File-Level API Sketch

```go
// dataset.go
type DemoScope struct {
    AccountID string
}

type DemoMeta struct {
    AccountID     string
    AccountName   string
    TicketCount   int
    EventCount    int
    GeneratedSeed string
}

func DemoDatasetSpec() scopeddb.DatasetSpec[DemoScope, DemoMeta]
```

```go
// fake_data.go
type Account struct { ... }
type Ticket struct { ... }
type TicketEvent struct { ... }

func BuildFixtures(accountID string) (Account, []Ticket, []TicketEvent)
```

```go
// renderers.go
func RegisterDemoRenderers(r *timeline.Registry)
func NewSQLToolCallFactory(toolName string) timeline.ModelFactory
func NewQueryResultTableFactory() timeline.ModelFactory
```

```go
// main.go
func buildDemoRegistry(ctx context.Context, scope DemoScope) (*tools.InMemoryToolRegistry, DemoMeta, func() error, error)
func newSystemPrompt() string
func main()
```

## Example End-To-End Sequence

The demo should make the following sequence easy to observe:

1. User types: "Show me the most recent open tickets for Acme."
2. Assistant decides to use the scopeddb tool.
3. TUI shows the tool call with SQL:

```sql
SELECT ticket_id, title, priority, opened_at
FROM tickets
WHERE status = 'open'
ORDER BY opened_at DESC
LIMIT 5
```

4. Tool returns rows.
5. TUI shows the rows in a table.
6. Assistant answers in plain English.

That sequence is the whole point of the demo.

## Detailed Task Breakdown

This ticket is analysis-only, but the future implementation should be broken down into small reviewable tasks.

### Task group A. Demo scaffolding

1. Create `pinocchio/cmd/examples/scopeddb-tui-demo/`.
2. Add `main.go`, `dataset.go`, `fake_data.go`, `renderers.go`, and `README.md`.
3. Verify the example builds with `go build ./cmd/examples/scopeddb-tui-demo`.

### Task group B. Dataset definition

1. Choose a fake-data domain and document it.
2. Define `DemoScope`.
3. Define `DemoMeta`.
4. Write `SchemaSQL`.
5. Define `AllowedObjects`.
6. Define `DefaultQuery` limits.
7. Implement `Materialize`.
8. Add unit tests for the materializer if practical.

### Task group C. Tool registration

1. Build the in-memory DB at startup.
2. Capture `Meta`.
3. Register the scopeddb tool with `RegisterPrebuilt`.
4. Add one startup message or status indicator that uses `Meta`.
5. Verify the tool appears in the registry and can execute a simple query.

### Task group D. TUI runtime

1. Create Watermill router and sink.
2. Build engine and middleware list.
3. Create `ToolLoopBackend`.
4. Build Bobatea model with title and renderer registration.
5. Register the UI forwarder.
6. Run the program and ensure graceful shutdown.

### Task group E. Renderer usability

1. Add SQL renderer for the demo tool call.
2. Add query result renderer for `QueryOutput`.
3. Test long SQL strings and truncated row outputs.
4. Confirm fallback rendering still works for non-query events.

### Task group F. Documentation and validation

1. Add README with run instructions and example prompts.
2. Add comments only where the code is not self-explanatory.
3. Run `gofmt`.
4. Run the example manually.
5. Add or update tests where useful.

## Testing And Validation Strategy

### Unit-level checks

- `Materialize` inserts the expected number of rows.
- `Meta` reflects the generated data.
- `QueryRunner` can query only allowed objects.
- `QueryRunner` rejects invalid SQL shapes.

### Manual checks

Run the example, then try prompts like:

- "List all tickets ordered by opened date."
- "Show the latest events for ticket T-100."
- "Which tickets are still open?"
- "Summarize the highest-priority issues."

Observe:

- tool call appears,
- SQL is readable,
- results are readable,
- assistant produces a final response.

### Failure-mode checks

Try:

- invalid `--account`,
- empty fixture set,
- a prompt that asks for data not present,
- a query that would exceed `MaxRows`.

The demo should fail clearly and teach the reader what happened.

## Risks, Alternatives, And Tradeoffs

### Risk: too much app scaffolding

If the demo copies too much from `switch-profiles-tui`, it will stop being a demo and become a second app. The fix is to keep only the essential runtime wiring and avoid profile or persistence features.

### Risk: demo hides the point of `Meta`

If `Meta` is returned but never displayed or logged, a new intern will reasonably ask why it exists. The fix is to use it in the startup UX and README.

### Risk: renderer code becomes over-engineered

The deleted temporal renderers were useful, but a demo does not need production-grade generalized renderers. Keep them small and local.

### Alternative: CLI-only example

Rejected because it demonstrates the tool but not the TUI value. The user explicitly asked for a TUI if possible, and Pinocchio already has the right primitives.

### Alternative: web-chat example

Rejected for first cut because it introduces HTTP server and frontend complexity unrelated to understanding scopeddb.

### Alternative: pure mock debug TUI

Rejected because it would teach the TUI but not the `scopeddb` package itself. The demo needs to execute real scopeddb queries.

## Open Questions

These are the only material questions still worth deciding before implementation:

1. Should the demo use `RegisterPrebuilt` for simplicity, or also include a commented example of `NewLazyRegistrar` for request-scoped apps?
2. Should the demo use a new demo-local result renderer, or can it reuse an existing renderer from another Pinocchio TUI package without pulling in unrelated UI concerns?
3. Should the fake data be fully literal fixtures or generated deterministically from a seed string?

My recommendation:

- start with `RegisterPrebuilt`,
- write demo-local renderers,
- use literal fixtures first.

## References

### Primary current code

- `pinocchio/cmd/switch-profiles-tui/main.go`
- `pinocchio/pkg/ui/backends/toolloop/backend.go`
- `pinocchio/cmd/examples/simple-chat/main.go`
- `pinocchio/cmd/web-chat/main.go`
- `pinocchio/pkg/middlewares/sqlitetool/middleware.go`
- `geppetto/pkg/inference/tools/scopeddb/schema.go`
- `geppetto/pkg/inference/tools/scopeddb/tool.go`
- `geppetto/pkg/inference/tools/scopeddb/query.go`

### Historical reference code

- historical `temporal-relationships` `cmd/temporal-relationships/cmds/tui/agent_chat.go` from parent of commit `1b05558`
- historical `temporal-relationships` `cmd/temporal-relationships/cmds/tui/tooldb_registry.go` from parent of commit `1b05558`
- historical `temporal-relationships` `cmd/temporal-relationships/cmds/tui/tool_call_sql_highlight_renderer.go` from parent of commit `1b05558`
- historical `temporal-relationships` `cmd/temporal-relationships/cmds/tui/tool_call_result_markdown_table_renderer.go` from parent of commit `1b05558`
- historical `temporal-relationships` `cmd/temporal-extract-js/debugtui/main.go` from parent of commit `b8467b7`

### Related ticket documentation

- `temporal-relationships/ttmp/2026/03/11/MEN-TR-055--remove-tui-surfaces-and-review-simplification-opportunities/design-doc/02-tui-removal-and-post-removal-simplification-analysis-and-intern-implementation-guide.md`
- `pinocchio/ttmp/2026/03/03/PI-01-REUSABLE-PINOCCHIO-TUI--reusable-pinocchio-tui-third-party-package/design-doc/02-unified-pinocchio-tui-simple-chat-agent-tool-loop-as-reusable-primitives.md`
