---
Title: Webchat Runner Migration Guide
Slug: webchat-runner-migration-guide
Short: How to migrate from legacy SubmitPrompt-based webchat startup to the runner-based startup model.
Topics:
- webchat
- backend
- migration
- api
- docs
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Goal

Explain how to migrate existing webchat embeddings from the legacy chat-start path:

- `ChatService.SubmitPrompt(...)`
- `ConversationService.SubmitPrompt(...)`
- `webhttp.NewChatHandler(...)`

to the runner-based startup model:

- `ConversationService.PrepareRunnerStart(...)`
- `ChatService.StartPromptWithRunner(...)`
- app-owned `POST /...` handlers that select a `Runner`

The new model keeps `Conversation` as the transport identity but moves execution behind a `Runner`.

## What Changed

`pkg/webchat` now has a runner-oriented startup seam with these core types:

- `webchat.Runner`
- `webchat.StartRequest`
- `webchat.StartResult`
- `webchat.RunHandle`
- `webchat.LLMLoopRunner`

The important boundary changes are:

- `StartRequest` is transport-safe and does not expose raw `*Conversation` state
- `StartResult` exposes a generic completion handle instead of a Geppetto execution handle
- generic conversation ensuring no longer creates LLM session state eagerly
- prompt queue/idempotency stays in `ChatService`

The legacy path still works:

- `ChatService.SubmitPrompt(...)`
- `ConversationService.SubmitPrompt(...)`
- `webhttp.NewChatHandler(...)`

But those should now be treated as convenience adapters over the older chat-start behavior, not as the preferred extension point for new embeddings.

## Recommended Migration Paths

### Option 1: Stay on the legacy path for now

If your embedding only needs the old behavior, do nothing.

This still works:

```go
resp, err := srv.ChatService().SubmitPrompt(ctx, webchat.SubmitPromptInput{
  ConvID:     convID,
  RuntimeKey: "default",
  Prompt:     prompt,
})
```

Use this when you do not need app-owned start semantics yet.

### Option 2: Migrate prompt-driven chat to app-owned runners

This is the recommended migration for new work.

Your app:

1. parses the HTTP request
2. resolves runtime/profile policy
3. selects a runner, usually `LLMLoopRunner`
4. calls `ChatService.StartPromptWithRunner(...)`
5. keeps using generic `/ws` and `/api/timeline`

This preserves queue/idempotency semantics while moving runner selection into app code.

### Option 3: Migrate non-chat SEM producers

If your feature is not prompt-driven, skip `ChatService.StartPromptWithRunner(...)`.

Instead:

1. ensure the conversation with `PrepareRunnerStart(...)`
2. pass the returned `StartRequest` to your own `Runner`
3. emit SEM and/or timeline entities through the provided surfaces

Use this for fake runners, extraction-style jobs, or other non-LLM flows.

## Old And New Composition

### Before: legacy convenience path

```go
chatHandler := webhttp.NewChatHandler(srv.ChatService(), resolver)
```

The handler owns:

- request parsing
- prompt validation
- queue/idempotency
- LLM startup

### After: app-owned prompt runner path

```go
handler := func(w http.ResponseWriter, req *http.Request) {
  plan, err := resolver.Resolve(req)
  if err != nil {
    http.Error(w, "failed to resolve request", http.StatusInternalServerError)
    return
  }

  result, err := srv.ChatService().StartPromptWithRunner(
    req.Context(),
    srv.ChatService().NewLLMLoopRunner(),
    webchat.StartPromptWithRunnerInput{
      Runtime:        plan.RuntimeRequest(),
      IdempotencyKey: webhttp.IdempotencyKeyFromRequest(req, nil),
      Payload: webchat.LLMLoopStartPayload{
        Prompt:    plan.Prompt,
        Overrides: plan.Overrides,
      },
      Metadata: map[string]any{"route": "chat-runner"},
    },
  )
  if err != nil {
    http.Error(w, "runner start failed", http.StatusInternalServerError)
    return
  }

  if result.HTTPStatus > 0 {
    w.WriteHeader(result.HTTPStatus)
  }
  _ = json.NewEncoder(w).Encode(result.Response)
}
```

This is the preferred prompt-driven migration because:

- the app chooses the runner
- queue/idempotency behavior is preserved
- generic transport stays reusable

### After: non-chat runner path

```go
_, startReq, err := srv.ChatService().PrepareRunnerStart(
  req.Context(),
  webchat.PrepareRunnerStartInput{
    Runtime:  plan.RuntimeRequest(),
    Metadata: map[string]any{"route": "fake-runner"},
  },
)
if err != nil {
  return err
}

result, err := myRunner.Start(req.Context(), startReq)
```

Use this when the thing being started is not a prompt-submission workflow.

## API Mapping

| Legacy | New preferred path | Notes |
|---|---|---|
| `ConversationService.SubmitPrompt(...)` | `ChatService.StartPromptWithRunner(...)` | Legacy still works; new path keeps queue semantics but lets the app choose the runner |
| `ChatService.SubmitPrompt(...)` | `ChatService.StartPromptWithRunner(...)` | Use `LLMLoopRunner` for the built-in LLM flow |
| `webhttp.NewChatHandler(...)` | app-owned `POST /...` handler | Keep `NewChatHandler(...)` only as a convenience |
| direct LLM startup hidden inside service | `Runner.Start(...)` | New path makes execution explicit |
| raw LLM session state created during ensure | lazy LLM state creation on first LLM run | Websocket-first attach no longer creates `session.Session` eagerly |

## Behavioral Notes

- Queue/idempotency is still chat-specific.
  Use `ChatService.StartPromptWithRunner(...)` for prompt-driven flows if you want the old `202 queued` / replay behavior.

- `PrepareRunnerStart(...)` is generic.
  It should not be expected to apply prompt queue semantics.

- `GET /ws` and `GET /api/timeline` stay generic.
  The startup route changes. The transport contract does not.

- `LLMLoopRunner` resolves execution state by `conv_id`.
  It does not require raw `*Conversation` access in the public runner API.

## Migration Checklist

- Decide whether your feature is prompt-driven or not.
- For prompt-driven chat:
  - keep using `ChatService.SubmitPrompt(...)` temporarily, or
  - switch to app-owned handler + `StartPromptWithRunner(...)`
- For non-chat runners:
  - use `PrepareRunnerStart(...)`
  - call your own `Runner.Start(...)`
- Keep websocket and timeline routes unchanged.
- Verify queue/replay behavior if you migrated prompt-driven chat.
- Verify timeline hydration still works after runner startup.

## Verification

Suggested commands after migrating:

1. `go test ./pkg/webchat/... ./cmd/web-chat -count=1`
2. `go test ./... -count=1`
3. `make lintmax`

If your app has an integration harness, verify:

- app-owned `POST /...` starts a conversation-backed run
- `GET /ws` still receives SEM
- `GET /api/timeline` hydrates the same `conv_id`

## See Also

- [Webchat Framework Guide](webchat-framework-guide.md)
- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
- [Webchat Values Separation Migration Guide](webchat-values-separation-migration-guide.md)
