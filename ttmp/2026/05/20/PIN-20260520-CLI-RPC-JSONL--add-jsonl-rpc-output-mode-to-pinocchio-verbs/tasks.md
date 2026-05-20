# Tasks

## Completed planning and research

- [x] Create docmgr ticket workspace for JSONL/RPC Pinocchio CLI output mode.
- [x] Inspect Pinocchio command loading, helper flag, run context, and output handler architecture.
- [x] Inspect Geppetto event router, event codec, text printer, and structured printer architecture.
- [x] Write intern-ready design and implementation guide with diagrams, pseudocode, API sketches, and file references.
- [x] Write chronological investigation diary.
- [x] Analyze `sessionstream/` and `pkg/chatapp/` as the canonical stream foundation for CLI, TUI, and RPC.
- [x] Write follow-up design proposing JSONL and Bubble Tea adapters over `sessionstream.UIFanout`.
- [x] Update the follow-up design so the JSONL line format is protobuf-defined via a proposed `pinocchio.chatapp.rpc.v1.RpcLine` envelope.
- [x] Relate key Pinocchio, Geppetto, sessionstream, and chatapp source files to the design and diary.
- [x] Validate ticket with `docmgr doctor`.
- [x] Upload final design bundle to reMarkable.

## Phase 1 — Protobuf contract and generated bindings

Goal: create the explicit generated boundary for JSONL/RPC lines before writing adapters.

- [x] Add `proto/pinocchio/chatapp/rpc/v1/rpc.proto`.
  - [x] Define `RpcLine` with `version`, `session_id`, `request_id`, and a `oneof frame`.
  - [x] Define `HelloFrame` with protocol/server/capabilities.
  - [x] Define `SnapshotFrame` and `SnapshotEntity` for sessionstream hydration snapshots.
  - [x] Define `UiEventFrame` with ordinal/name/`google.protobuf.Any` payload.
  - [x] Define `BackendEventFrame` with ordinal/name/`google.protobuf.Any` payload for future debug/advanced modes.
  - [x] Define `ErrorFrame` and `DoneFrame`.
  - [x] Reserve or document field-number ranges for future bidirectional stdin frames if not included now.
- [x] Regenerate Go protobuf bindings for chatapp protos.
  - [x] Verify generated Go package path under `pkg/chatapp/pb/proto/pinocchio/chatapp/rpc/v1` or the final chosen path.
  - [x] Verify existing `chatapp/v1` generated files are not unexpectedly reformatted or churned.
- [x] Regenerate TypeScript bindings if the repository's existing chatapp web generation supports the new proto path.
  - [x] Existing chatapp web generation supports the new proto path; generated `rpc_pb.ts`.
- [x] Add a minimal compile-time test that imports the generated `chatapprpcv1` package.
- [x] Run protobuf validation commands.
  - [x] `buf lint --path proto/pinocchio/chatapp/rpc/v1/rpc.proto`
  - [x] targeted Go tests for generated package import.
- [ ] Commit Phase 1 as a focused protobuf-contract commit. (pending after diary update)

## Phase 2 — ProtoJSON JSONL writer package

Goal: provide a small, tested writer that guarantees one protobuf JSON `RpcLine` per line.

- [x] Add a package such as `pkg/chatapp/rpc/jsonl`.
- [x] Implement `Writer` with:
  - [x] `protojson.MarshalOptions{EmitUnpopulated:false, UseProtoNames:false}`.
  - [x] mutex-protected writes to keep concurrent fanout output line-safe.
  - [x] `WriteLine(*chatapprpcv1.RpcLine) error`.
- [x] Implement helper constructors/writers:
  - [x] `NewHelloLine(sessionID string, capabilities []string) *RpcLine`.
  - [x] `NewErrorLine(sessionID string, code string, err error, terminal bool) *RpcLine`.
  - [x] `NewDoneLine(sessionID string, status string) *RpcLine`.
- [x] Add tests:
  - [x] one call writes exactly one newline-terminated line.
  - [x] line unmarshals back into `chatapprpcv1.RpcLine` with `protojson.UnmarshalOptions`.
  - [x] concurrent writes produce complete JSON lines without interleaving.
  - [x] no empty `{}` lines for nil/invalid input; invalid input returns an error.
- [x] Run targeted tests for the package.
- [x] Commit Phase 2 as a focused writer commit (`18c1513`).

## Phase 3 — sessionstream JSONL fanout and snapshot helpers

Goal: adapt projected chatapp/sessionstream UI events and snapshots to protobuf-defined JSONL.

- [x] Implement `JSONLUIFanout` that satisfies `sessionstream.UIFanout`.
  - [x] For each `sessionstream.UIEvent`, pack `ev.Payload` with `anypb.New`.
  - [x] Emit `RpcLine_UiEvent` frames with session ID, ordinal, event name, and `Any` payload.
  - [x] Return errors for nil/non-packable payloads.
- [x] Implement snapshot emission.
  - [x] Convert `sessionstream.Snapshot` to `SnapshotFrame`.
  - [x] Pack `TimelineEntity.Payload` with `anypb.New`.
  - [x] Preserve `kind`, `id`, `created_ordinal`, `last_event_ordinal`, and `tombstone`.
- [x] Add optional backend-event writer support using the same `RpcLine_BackendEvent` shape, but keep it disabled by default.
- [x] Add tests:
  - [x] `ChatTextPatch` UI event emits a valid `uiEvent` line.
  - [x] `ChatTextSegmentFinished` UI event emits a valid `uiEvent` line.
  - [x] `ChatRunFinished` UI event emits a valid `uiEvent` line.
  - [x] emitted `Any` payloads unpack into concrete `chatappv1` messages.
  - [x] snapshot entities unpack into concrete `chatappv1.ChatMessageEntity` messages.
- [x] Run targeted tests.
- [x] Commit Phase 3 as a focused fanout commit (`b6ab166`).

## Phase 4 — Reusable non-web chatapp runner

Goal: make CLI/TUI able to use the same sessionstream/chatapp stack currently wired by web-chat.

- [x] Add `pkg/chatapp/runner.go` or an equivalent package-level helper.
  - [x] Accept `HydrationStore`, `UIFanout`, `TurnStore`, plugins, and chunk-delay options.
  - [x] Create a `SchemaRegistry` and call `chatapp.RegisterSchemas`.
  - [x] Create default in-memory hydration store when none is provided.
  - [x] Create `chatapp.Engine` with plugins and turn-store support.
  - [x] Create `sessionstream.Hub` with schema registry, hydration store, and fanout.
  - [x] Call `chatapp.Install` and return `chatapp.Service` plus useful handles.
- [x] Add tests mirroring web-chat setup without HTTP.
  - [x] runner can submit a demo prompt and wait idle.
  - [x] runner can return a snapshot.
  - [x] plugins register reasoning/tool schemas through the runner.
- [x] Commit Phase 4 as a focused runner commit (`4f36ae0`).

## Phase 5 — Support rich Pinocchio verb input in chatapp

Goal: avoid losing existing Pinocchio verb semantics when routing through chatapp.

- [x] Extend `chatapp.PromptRequest` with an optional initial turn or equivalent typed input.
  - [x] Preferred first step: `InitialTurn *turns.Turn` for in-process CLI/TUI use.
  - [x] Preserve `Prompt string` for display/user-message content and simple chat calls.
- [x] Update `chatapp.runRuntimeInference` to seed `geppetto` session from `InitialTurn` when provided.
- [x] Preserve existing prompt-only behavior and tests.
- [x] Add tests:
  - [x] prompt-only requests still append user prompt as today.
  - [x] initial-turn requests pass system/user/image/block context into runtime.
  - [x] prior turn-store history behavior remains correct or is explicitly ordered relative to initial turn.
- [ ] Commit Phase 5 as a focused chatapp-input commit. (pending after diary update)

## Phase 6 — CLI RPC integration for Pinocchio verbs

Goal: route `--rpc` / `--output jsonl` through chatapp/sessionstream and protobuf JSONL.

- [ ] Add or finalize helper flags/settings.
  - [ ] Add `--rpc` to `pkg/cmds/cmdlayers/helpers.go`.
  - [ ] Add `jsonl` to `--output` choices.
  - [ ] Add `RPC bool` to `run.UISettings`.
  - [ ] Make `--rpc` imply non-interactive behavior.
- [ ] Add `runRPCViaChatApp` or equivalent path in `pkg/cmds/cmd.go`.
  - [ ] Build/render the initial `turns.Turn` from the Pinocchio command.
  - [ ] Build or adapt `infruntime.ComposedRuntime` from existing CLI inference settings/profile resolution.
  - [ ] Create JSONL writer/fanout and chatapp runner.
  - [ ] Emit hello and snapshot frames before submitting prompt.
  - [ ] Submit `chatapp.PromptRequest` and wait idle.
  - [ ] Emit done or error frame.
- [ ] Keep `--output text|json|yaml` on the existing raw Geppetto path for compatibility.
- [ ] Add integration-style tests using fake runtime engines.
  - [ ] stdout contains only protobuf JSONL lines.
  - [ ] every line unmarshals as `RpcLine`.
  - [ ] `ChatTextPatch` and `ChatRunFinished` appear.
  - [ ] process errors still surface via exit status and/or terminal error frame.
- [ ] Commit Phase 6 as a focused CLI-RPC integration commit.

## Phase 7 — Bubble Tea/TUI adapter over sessionstream

Goal: migrate TUI streaming from raw Geppetto event decoding to projected chatapp UI events.

- [ ] Implement a `sessionstream.UIFanout` adapter that sends Bubble Tea timeline messages.
- [ ] Implement snapshot hydration for Bubble Tea timeline from `sessionstream.Snapshot`.
- [ ] Add parity tests against current `StepChatForwardFunc` behavior for:
  - [ ] assistant text streaming.
  - [ ] final assistant completion.
  - [ ] errors and interrupts.
  - [ ] reasoning/thinking segments.
  - [ ] tool calls and tool results if renderers exist.
- [ ] Wire TUI chat mode to chatapp runner behind an internal feature branch or low-risk switch.
- [ ] Keep old raw handlers until parity is proven.
- [ ] Commit Phase 7 as one or more focused TUI adapter commits.

## Phase 8 — Documentation, cleanup, and de-duplication

Goal: finish the migration and remove avoidable duplicate stream mappings.

- [ ] Add CLI help docs for `--rpc` / `--output jsonl`.
  - [ ] Explain protobuf JSONL.
  - [ ] Explain `google.protobuf.Any` `@type` payloads.
  - [ ] Explain protobuf JSON `uint64` strings and `jq tonumber`.
  - [ ] Include jq examples for `ChatTextPatch`, final text, tool results, and done/error frames.
- [ ] Update design docs if implementation decisions differ from the ticket plan.
- [ ] Evaluate whether `StepChatForwardFunc` can be deprecated or removed.
- [ ] Evaluate whether `StepTimelinePersistFunc` can be deprecated in favor of sessionstream hydration stores.
- [ ] Run broad validation:
  - [ ] `go test ./pkg/chatapp/... ./pkg/cmds/... ./cmd/pinocchio/...`
  - [ ] `go test ./cmd/web-chat/...` if runtime composer movement touched web-chat.
  - [ ] `make schema-vet` if schema changes need vet coverage.
- [ ] Commit documentation/cleanup changes.

## Current implementation checkpoint

- Active phase: Phase 5 validation/commit.
- Current source changes: `PromptRequest.InitialTurn`, runtime seeding changes, and chatapp tests.
- Next concrete action: commit Phase 5, then start Phase 6 CLI RPC integration.
