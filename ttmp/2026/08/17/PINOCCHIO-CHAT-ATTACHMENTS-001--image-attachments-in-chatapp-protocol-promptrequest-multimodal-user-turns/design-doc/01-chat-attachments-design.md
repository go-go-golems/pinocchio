---
Title: Chat attachments design
Ticket: PINOCCHIO-CHAT-ATTACHMENTS-001
Status: active
Topics:
    - chatapp
    - webchat
    - sessionstream
    - geppetto
    - design
DocType: design-doc
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: repo://pkg/chatapp/projections.go
      Note: projection
    - Path: repo://pkg/chatapp/runtime_inference.go
      Note: engine
    - Path: repo://pkg/chatapp/serverkit/contracts.go
      Note: HTTP contract
    - Path: repo://pkg/chatapp/service.go
      Note: PromptRequest
    - Path: repo://proto/pinocchio/chatapp/v1/chat.proto
      Note: protocol
ExternalSources: []
Summary: Add image attachments (by reference) to the chatapp protocol, PromptRequest and HTTP contract, echo them to clients, and build multimodal user turns via geppetto.
LastUpdated: 2026-08-17T14:55:28.405225119-04:00
WhatFor: ""
WhenToUse: ""
---


# Chat attachments design

## Executive Summary

`pkg/chatapp` today moves a single `prompt` string from the HTTP contract
(`serverkit.SubmitMessageRequest`) through `PromptRequest` and the sessionstream
command `StartInferenceCommand` into a text-only geppetto user block
(`sess.AppendNewTurnFromUserPrompt`). Consumers such as CoinVault want users to
attach images. This ticket adds an **attachment reference** concept end to end:

- `ChatAttachment` proto message; `repeated ChatAttachment attachments` on
  `StartInferenceCommand`, `ChatUserMessageAccepted`, and `ChatMessageEntity`;
- `serverkit.AttachmentRef` on `SubmitMessageRequest`, `chatapp.Attachment` on
  `PromptRequest`;
- "prompt **or** attachments" validation instead of "prompt required";
- the engine echoes attachments in `ChatUserMessageAccepted`, projects them
  into `ChatMessageEntity`, and builds the user block with geppetto's
  `Session.AppendNewTurnFromUserMessage(text, images)` (GEPPETTO-MULTIMODAL-HISTORY-001);
- `chatapp.AttachmentsToTurnImages` converts attachments to the geppetto image
  map shape.

Bytes never travel through chatapp: an application (CoinVault) stores them and
supplies URLs. `cmd/web-chat` gains the request field but no upload UI (out of scope).

## Problem Statement

Evidence:

- `proto/pinocchio/chatapp/v1/chat.proto:7-11, 253-276` — string-only messages.
- `pkg/chatapp/serverkit/contracts.go:21-27` — `SubmitMessageRequest{Prompt,…}`.
- `pkg/chatapp/service.go:17-35, 59-84` — `PromptRequest`; empty prompt rejected.
- `pkg/chatapp/runtime_inference.go:21-47` — echo `ChatUserMessageAccepted`
  text only; `:107-138` — `AppendNewTurnFromUserPrompt`; `InitialTurn` bypasses history.
- `pkg/chatapp/projections.go:29-43` — user message projection.
- `pkg/cmds/images.go` — CLI-only image → payload conversion (unexported).

## Proposed Solution

### Protocol

```proto
message ChatAttachment {
  string attachment_id = 1;
  string kind = 2;             // "image"
  string media_type = 3;
  string url = 4;              // browser-fetchable (app-relative or absolute)
  uint64 size_bytes = 5;
  uint32 width = 6;
  uint32 height = 7;
  string filename = 8;
  string detail = 9;           // low|high|auto|original
  map<string, string> metadata = 10;   // app-owned extras; e.g. turn_url
}
StartInferenceCommand.attachments = 4
ChatUserMessageAccepted.attachments = 7
ChatMessageEntity.attachments = 14
```

### Go API

```go
// serverkit
type AttachmentRef struct { AttachmentID string `json:"attachment_id"` }
SubmitMessageRequest.Attachments []AttachmentRef `json:"attachments,omitempty"`

// chatapp
type Attachment struct {
    ID, Kind, MediaType, URL, Filename, Detail string
    SizeBytes uint64; Width, Height uint32
    Metadata map[string]string   // "turn_url" overrides URL in the turn image map
}
PromptRequest.Attachments []Attachment
func AttachmentsToTurnImages(atts []Attachment) []map[string]any
func AttachmentsToProto(atts []Attachment) []*chatappv1.ChatAttachment
func AttachmentsFromProto(pb []*chatappv1.ChatAttachment) []Attachment
```

Turn image map per attachment: `{"attachment_id", "media_type", "url": turn_url|url,
"detail"?}` — the geppetto `imageparts` normalizer reads `url`/`media_type`/`detail`
and ignores unknown keys.

### Engine flow

1. `SubmitPromptRequest`: reject only when prompt is empty **and** no
   attachments; put attachments on the command.
2. `handleStartInference`: attachments = pending.Attachments, else from the
   command payload; the "Explain evtstream" fallback only applies when there
   are no attachments either; publish `ChatUserMessageAccepted{…, Attachments}`.
3. `runRuntimeInference` (history branch): if attachments →
   `sess.AppendNewTurnFromUserMessage(prompt, images)` else the existing call.
4. `baseTimelineProjection`: copy attachments into `ChatMessageEntity`.
5. Demo inference path: unchanged (text only).

## Design Decisions

- **References only in the protocol.** Everything is persisted as protojson
  and re-sent in snapshots; bytes would multiply storage (see COINVAULT-046 D1).
- **`map<string,string> metadata`** rather than `google.protobuf.Struct`:
  keeps `schema-vet` trivially happy and is enough for app hints (`turn_url`).
- **Turn URL vs browser URL.** The turn image map may use an internal URL that
  only the app runtime can resolve (CoinVault middleware); the proto `url` is
  what browsers fetch. `Metadata["turn_url"]` carries the former.
- **No `InitialTurn` usage.** History semantics stay in one code path.

## Alternatives Considered

- App-owned attachment entities via `ChatPlugin` — keeps pinocchio untouched
  but splits the user message across two entities; rejected for the first-class case.
- Encoding references in the prompt string — rejected (pollutes prompt, breaks idempotency).

## Implementation Plan

1. `chat.proto` + `make proto-gen-core` (Go + web-chat TS).
2. `serverkit/contracts.go`, `pkg/chatapp/attachments.go` (new), `service.go`,
   `runtime_inference.go`, `projections.go`.
3. `cmd/web-chat/internal/appserver/routes_sessions.go`: pass attachments through
   (`Attachments` from `AttachmentRef` — web-chat has no store, so it maps
   ids to `Attachment{ID}` only; apps with a store resolve first).
4. Tests: `attachments_test.go`, `service_test.go` (validation),
   `runtime_inference` echo + block payload, `projections_protocol_test.go`.
5. `README`/doc note.

## Open Questions

- Should `cmd/web-chat` reject attachment refs it cannot resolve? For now it
  passes them through as bare ids (no URL → geppetto adapters skip them).

## References

- COINVAULT-046 (coinvault) design doc §8–§10; GEPPETTO-MULTIMODAL-HISTORY-001.
