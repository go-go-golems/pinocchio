Title: Migrating Pinocchio to Turns and the New Bobatea Timeline Chat UI
Slug: report-porting-to-turns-and-new-bobatea-ui
Short: A detailed report of the migration from conversation/step to engine/turn with timeline-based UI, plus guidance to port the simple-chat-agent
Topics:
- pinocchio
- migration
- turns
- engine
- chat
- bobatea
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic

## Purpose and scope

This report documents the end-to-end migration of Pinocchio from the legacy conversation + chat step model to Geppetto’s Engine/Turn architecture, along with adoption of the Bobatea timeline chat UI. It captures all notable code changes under `pinocchio/pkg`, explains rationale and design decisions, and provides a concrete checklist and next steps to perform the same migration for `cmd/agents/simple-chat-agent/main.go`.

To ground this report in facts, we generated and reviewed the full diff against `origin/main` for the `pinocchio/pkg` tree and attached it here: `ttmp/2025-08-13/pkg-diff.txt`.

## High-level summary of changes

- Replaced step-based chat execution with Engine/Turn flows.
- Introduced a new `EngineBackend` for the chat UI that consumes engine events and seeds timeline entities.
- Removed runtime dependence on `conversation.Manager` in the command pipeline; everything now runs on `turns.Turn` and converts back to conversation only at output boundaries.
- Implemented prompt templating using `glazed` templating for `system`, `messages` (now simplified to strings mapped to user blocks), and `prompt`.
- Switched YAML `messages` from complex conversation objects to a simple `[]string` that is converted into user blocks during load.
- Updated the migration documentation and refactored supporting docs to reflect Engine/Turn terminology.

## Notable diffs (overview)

The complete diff is saved to `ttmp/2025-08-13/pkg-diff.txt`. Key areas:

- Chat runner: migration from step factory to engine factory and Turn-first orchestration
- UI backend: new engine-backed UI integration, timeline entity seeding, dedup
- Command: Turn construction, rendering, and chat seeding; removal of conversation manager runtime dependency
- Loader: YAML schema changes (`messages: []string`) and conversion to user blocks
- Run context: options refactor (engine factory, variables, output conversation), removal of manager
- Documentation: refreshed to Engine/Turn concepts
- Cleanup: removed obsolete codegen/config logic

## Detailed changes

### 1) Chat runner (`pkg/chatrunner/chat_runner.go`)

- Replace step factory with `engine/factory.EngineFactory` and `settings.StepSettings`.
- Wire a Watermill sink into the engine for UI streaming.
- Use Turn-first seeding for blocking/interactive flows.
- Forward UI events via the router to the Bubble Tea program (unchanged handler name, updated payload semantics).

Example excerpt:
```1:26:pinocchio/pkg/chatrunner/chat_runner.go
package chatrunner

import (
    "context"
    "fmt"
    "os"
    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/turns"
    "io"
    ...
)
```

### 2) UI backend (`pkg/ui/backend.go`)

- Introduce `EngineBackend` that implements the Bobatea chat backend using Engines and Turns.
- Maintain a `history []*turns.Turn` and a reducer to build a seed Turn for each run.
- Emit timeline entities for prior user/assistant text blocks when entering chat (system omitted to prevent duplication) and deduplicate by block ID.
- Forward engine streaming events to UI via `StepChatForwardFunc` using timeline `Created/Updated/Completed` messages.

Example excerpt:
```21:40:pinocchio/pkg/ui/backend.go
type EngineBackend struct {
    engine    engine.Engine
    isRunning bool
    cancel    context.CancelFunc
    historyMu sync.RWMutex
    history   []*turns.Turn
    program   *tea.Program
    emittedMu sync.Mutex
    emitted   map[string]struct{}
}
```

### 3) Command (`pkg/cmds/cmd.go`)

- Add rendering utilities using `glazed` templating to render `system`, block texts, and `prompt`.
- Convert to a Turn-first pipeline: build a rendered seed Turn and call `engine.RunInference`.
- In chat mode, attach a UI engine sink, create `EngineBackend`, attach the program, seed from the rendered Turn (so prior history shows), and optionally auto-submit rendered prompt.
- PrintPrompt path builds a Turn and converts to conversation for output only.
- Introduced `Blocks []turns.Block` on `PinocchioCommand`; removed dependence on `[]*conversation.Message` for configuration.

Example excerpt:
```124:149:pinocchio/pkg/cmds/cmd.go
type PinocchioCommand struct {
    *glazedcmds.CommandDescription `yaml:",inline"`
    Prompt  string        `yaml:"prompt,omitempty"`
    Blocks  []turns.Block `yaml:"-"`
    SystemPrompt string   `yaml:"system-prompt,omitempty"`
}
```

### 4) Run context (`pkg/cmds/run/context.go`)

- Replace step factory and conversation manager with `EngineFactory`, `Variables`, `ResultConversation`.
- Provide `WithVariables(...)` so callers can pass parsed layer variables for templating.
- Keep router and UI settings for chat.

Example excerpt:
```1:22:pinocchio/pkg/cmds/run/context.go
import (
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    ...
)

type RunContext struct {
    StepSettings   *settings.StepSettings
    EngineFactory  factory.EngineFactory
    Router         *events.EventRouter
    Variables      map[string]interface{}
    ResultConversation []*geppetto_conversation.Message
    ...
}
```

### 5) Loader (`pkg/cmds/loader.go`)

- YAML messages are now `[]string`. Each entry is mapped to a user text block during load.
- Pass these blocks down via `WithBlocks(blocks)`.
- Wrap the default geppetto layers and append the Pinocchio helper layer.

Example excerpt:
```90:108:pinocchio/pkg/cmds/loader.go
blocks := make([]turns.Block, 0, len(scd.Messages))
for _, text := range scd.Messages {
    if strings.TrimSpace(text) == "" { continue }
    blocks = append(blocks, turns.NewUserTextBlock(text))
}

sq, err := NewPinocchioCommand(
    description,
    WithPrompt(scd.Prompt),
    WithBlocks(blocks),
    WithSystemPrompt(scd.SystemPrompt),
)
```

### 6) Documentation

- `pkg/doc/topics/01-chat-runner-events.md` updated from steps to engines/turns terminology, and examples adjusted accordingly.
- `bobatea/ttmp/2025-08-12/03-porting-pinocchio-to-bobatea-timeline-chat.md` updated (see that page) to reflect the completed changes (templating, Turn-first, loader messages as strings, backend entity seeding/dedup, auto-start behavior).

### 7) Cleanup

- Removed `pkg/codegen/codegen.go` and `pkg/cmds/config.go` which were tied to the old step/manager patterns.
- Simplified `pkg/cmds/cobra.go` to rely on upstream `GetCobraCommandGeppettoMiddlewares`.

## Prompt rendering pipeline

- Variables are passed as `run.WithVariables(...)` into the run context from parsed layers.
- At run time, `PinocchioCommand.buildInitialTurn(vars)` renders:
  - `systemPrompt`
  - text payloads of each pre-seeded user block (derived from YAML `messages`)
  - `prompt`
- The rendered Turn is used for blocking runs, chat seeding, and auto-start submission (so the UI shows rendered text).

## Timeline seeding and dedup strategy

- On entering chat, the backend seeds the timeline by emitting `UIEntityCreated/Completed` for prior user/assistant blocks in the seed Turn.
- System blocks are intentionally not emitted to avoid duplication with pre-run previews.
- Block IDs are used as local entity IDs and tracked in an `emitted` set to prevent duplicates.

## YAML schema change

- `messages:` is now a list of strings:
  ```yaml
  messages:
    - "You are a helpful assistant"
    - "Use short, direct answers"
  ```
- The loader converts each string into a `turns.NewUserTextBlock(text)`; the `system-prompt:` remains a single string.

## Migration checklist status (pkg)

- Engine/Turn migration in command and chat paths: done
- Timeline UI backend and event forwarding: done
- Prompt templating and variable passing: done
- YAML messages → strings mapped to user blocks: done
- Conversation manager removed from runtime paths: done (still available at edges where needed for output conversion)
- Docs updated to Engine/Turn: done
- Cleanups (codegen/config): done

## Next steps: port `cmd/agents/simple-chat-agent/main.go`

Goal: apply the same Engine/Turn + timeline UI model to the simple chat agent.

Recommended steps:

1) Replace step/manager centric wiring with Engine/Turn
- Create an engine via `factory.NewEngineFromParsedLayers(parsed)`.
- Build a seed `*turns.Turn` from your current conversation state or initial prompts (system + user), rendering templates as needed.
- Call `engine.RunInference(ctx, seed)`; use middleware for tools/agent-mode. Persist snapshots as you already do.

2) Streaming/events
- Use `middleware.NewWatermillSink(router.Publisher, "ui")` to receive engine events and feed the UI.
- Register `ui.StepChatForwardFunc(p)` as a handler for the `ui` topic to convert events into timeline entities.

3) UI backend
- Instantiate `ui.NewEngineBackend(engine)`, attach the Bubble Tea program with `AttachProgram(p)`, and call `SetSeedTurn(seed)` so prior history appears.
- For REPL-style UX, you can keep your REPL evaluator but ensure turns are the unit of inference and that tool middlewares operate on blocks.

4) Prompt templating
- If the agent supports templated prompts or message patterns, render them via `glazed` templating into the Turn before calling the engine.

5) Conversation export (optional)
- When you need to output text to stdout or logs, convert the latest Turn to `conversation.Conversation` with `turns.BuildConversationFromTurn` and print only the tail messages.

6) Verify tool flows
- Ensure `tool_call` / `tool_use` blocks are carried across turns. If you want to visualize tools in the timeline, mirror tool events to dedicated entities.

## References

- Full code diff: `ttmp/2025-08-13/pkg-diff.txt`
- Updated migration doc: `bobatea/ttmp/2025-08-12/03-porting-pinocchio-to-bobatea-timeline-chat.md`
- Geppetto: Engine/Turn and middleware APIs


