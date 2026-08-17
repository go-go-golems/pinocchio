---
Title: Diary
Ticket: PINOCCHIO-CHAT-ATTACHMENTS-001
Status: active
Topics:
    - chatapp
    - webchat
    - sessionstream
    - geppetto
    - design
DocType: reference
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: repo://pkg/chatapp/attachments.go
      Note: Attachment type + converters (commit 6889069)
    - Path: repo://pkg/chatapp/runtime_inference.go
      Note: echo + multimodal append (commit 6889069)
    - Path: repo://pkg/chatapp/service.go
      Note: PromptRequest.Attachments, validation (commit 6889069)
    - Path: repo://pkg/doc/topics/chatapp-protobuf-plugins.md
      Note: glazed help page update
    - Path: repo://proto/pinocchio/chatapp/v1/chat.proto
      Note: ChatAttachment (commit 6889069)
ExternalSources: []
Summary: Diary of adding attachment references to the chatapp protocol and engine.
LastUpdated: 2026-08-17T14:55:28.625133836-04:00
WhatFor: ""
WhenToUse: ""
---


# Diary

## Goal

Record the implementation of PINOCCHIO-CHAT-ATTACHMENTS-001: carrying image
attachment references through `serverkit.SubmitMessageRequest`,
`chatapp.PromptRequest`, `StartInferenceCommand`, `ChatUserMessageAccepted`
and `ChatMessageEntity`, and building multimodal user turns with geppetto's
new `Session.AppendNewTurnFromUserMessage`. Consumed by CoinVault (COINVAULT-046).

## Step 1: Ticket, design doc, protocol and engine implementation

After finishing the geppetto prerequisites (GEPPETTO-MULTIMODAL-HISTORY-001)
I created this ticket and design doc, extended `chat.proto`, regenerated the
Go and web-chat TypeScript bindings, added the `Attachment` type and
converters, wired attachments through the service, engine, projections and
the web-chat HTTP handler, and covered the behavior with tests. Two mechanical
issues showed up: `buf generate` with the remote plugins needs a Buf API
login that this machine lacks, and the freshly generated TS file failed
Biome's import ordering check.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket in each relevant repository, with a its own design document, and then implement them. commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)

Backfill the diary as necessary. as well. Make sure the diary format from the skill is WELL UNDERSTOOD TO BE MANDATORY."

Mid-task addition (verbatim): "make sure to update relevant glazed documentation when appropripate as well."

**Assistant interpretation:** For pinocchio: own ticket + design doc, implement the protocol/engine part of the CoinVault image-upload design, commit, keep this diary in the mandatory format, and update the embedded (glazed) help pages under `pkg/doc`.

**Inferred user intent:** Reviewable, documented per-repo change so pinocchio's chat engine supports attachments generically, not just for CoinVault.

**Commit (code):** 6889069 — "chatapp: image attachments by reference (protocol, PromptRequest, multimodal user turns)"
**Commit (docs):** 1f5e00a — "PINOCCHIO-CHAT-ATTACHMENTS-001: ticket + design doc"

### What I did

- `proto/pinocchio/chatapp/v1/chat.proto`: `ChatAttachment` (id, kind,
  media_type, url, size_bytes, width, height, filename, detail,
  `map<string,string> metadata`); `attachments` on `StartInferenceCommand`
  (4), `ChatUserMessageAccepted` (7), `ChatMessageEntity` (14).
- Regenerated `pkg/chatapp/pb/.../chat.pb.go` and
  `cmd/web-chat/web/src/generated/chatapp/proto/pinocchio/chatapp/v1/chat_pb.ts`.
- `pkg/chatapp/serverkit/contracts.go`: `AttachmentRef`, `SubmitMessageRequest.Attachments`.
- `pkg/chatapp/attachments.go` (new): `Attachment`, `IsImage`, `TurnURL`,
  `AttachmentsToTurnImages`, `AttachmentsToProto`, `AttachmentsFromProto`,
  constants `AttachmentKindImage`, `AttachmentMetadataTurnURL`.
- `pkg/chatapp/service.go`: `PromptRequest.Attachments`; "prompt or attachments" rule; attachments on the wire.
- `pkg/chatapp/runtime_inference.go`: wire fallback for attachments, no demo
  prompt fallback when attachments exist, echo in `ChatUserMessageAccepted`,
  `AppendNewTurnFromUserMessage` when image entries exist.
- `pkg/chatapp/projections.go`: copy attachments into `ChatMessageEntity`.
- `cmd/web-chat/internal/appserver/routes_sessions.go`: prompt-or-attachments gate; pass-through by id.
- Tests: `pkg/chatapp/attachments_test.go` (5 tests).
- Docs (glazed help page): `pkg/doc/topics/chatapp-protobuf-plugins.md`.

```bash
buf generate --template <scratch>/buf.local.go.gen.yaml --path proto/pinocchio    # local protoc-gen-go
buf generate --template <scratch>/buf.local.web.gen.yaml --path proto/pinocchio   # local protoc-gen-es from coinvault/web/node_modules
go test ./pkg/chatapp/... ./cmd/web-chat/... -count=1     # ok
make schema-vet; golangci-lint run ./pkg/chatapp/... ./cmd/web-chat/...   # 0 issues
make web-check                                             # ok after import reorder
```

### Why

- References-only protocol keeps sessionstream events/entities/snapshots small (COINVAULT-046 D1).
- `metadata.turn_url` lets an app put an internal URL into the turn that its own
  runtime resolves, while browsers get the HTTP URL.
- Falling back to wire attachments makes replayed/externally submitted commands work.

### What worked

- Existing `newTestHub` / `recordingHistoryEngine` test helpers made engine
  behavior easy to assert (`TestRuntimeInferenceAppendsMultimodalUserBlockForAttachments`).

### What didn't work

- `make proto-gen-core` failed:
  `Failure: your Buf API token for buf.build is invalid. Run "buf registry login"…`.
  Worked around with local plugin templates (`plugin: go`, `plugin: es` with
  `path: ../coinvault/web/node_modules/.bin/protoc-gen-es`).
- Local `protoc-gen-es` is v2.13.0 and rewrote three unrelated generated files
  (`frontend_tool_pb.ts`, `rpc_pb.ts`, `widget_pb.ts`) with only header/import
  churn; I reverted those and kept only `chat_pb.ts`.
- First commit attempt failed the `web-check` lefthook step: Biome
  `organizeImports` on the generated `chat_pb.ts`
  (`import type { Message }` must come first). Reordered the three import
  lines by hand to match the previously checked-in file; the checked-in file had
  evidently received the same fix before.

### What I learned

- The `web-chat` generated TS is linted by Biome and the generator's import
  order does not satisfy it; keep the manual reorder in mind for future regenerations.

### What was tricky to build

- The engine's demo fallback (`prompt = "Explain evtstream"`) must not fire
  for image-only messages, and `pending.Attachments` may be empty when a
  command arrives without an in-process pending request — both handled in
  `handleStartInference` and covered by
  `TestHandleStartInferenceFallsBackToWireAttachments`.

### What warrants a second pair of eyes

- Proto field numbers (4, 7, 14) and the choice of `map<string,string>` for
  metadata (vs `google.protobuf.Struct`).
- `cmd/web-chat` accepts attachment ids without a store: they are echoed but
  never reach the model — acceptable placeholder, but confirm.

### What should be done in the future

- web-chat frontend upload UI (separate ticket).
- CoinVault TS mirror regeneration is done in COINVAULT-046 (copy of `chat_pb.ts`).

### Code review instructions

- Start with `pkg/chatapp/attachments.go`, then `service.go:SubmitPromptRequest`,
  `runtime_inference.go:handleStartInference` and the history branch of
  `runRuntimeInference`, then `projections.go`.
- Validate: `go test ./pkg/chatapp/... -count=1 && make schema-vet && make web-check`.

### Technical details

- Turn image map: `{"attachment_id","media_type","url": turn_url|url,"detail"?}`.
- Empty prompt + attachments ⇒ user entity with empty content and populated `attachments`.
