---
Title: Chatapp Protobuf Schemas and Shared Plugins
Slug: chatapp-protobuf-plugins
Short: Reference for Pinocchio chatapp protobuf contracts, segment-aware message entities, and shared reasoning/tool-call plugins.
Topics:
- webchat
- chatapp
- protobuf
- sessionstream
- plugins
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Overview

`pinocchio/pkg/chatapp` is the reusable chat application layer on top of `sessionstream`. It owns the base chat command, event, UI-event, and timeline schemas, while application packages can add more behavior through `ChatPlugin` implementations.

The current chatapp contract is protobuf-first:

- source schema: `proto/pinocchio/chatapp/v1/chat.proto`
- generated Go package: `pkg/chatapp/pb/proto/pinocchio/chatapp/v1`
- generator config: `buf.chatapp.gen.yaml`
- runtime registration: `chatapp.RegisterSchemas(reg, plugins...)`

Sessionstream still delivers browser frames as JSON over WebSocket, but those frames are the JSON form of registered protobuf messages. Backend code should publish `proto.Message` payloads, not ad-hoc maps, unless the feature intentionally uses `google.protobuf.Struct` for flexible metadata.

## Base chatapp schema

The base chatapp registers these command messages:

| Name | Protobuf message | Purpose |
|---|---|---|
| `ChatStartInference` | `StartInferenceCommand` | Start an assistant run for a prompt. |
| `ChatStopInference` | `StopInferenceCommand` | Stop the active assistant run for a session. |

It registers these backend events and UI events with `ChatMessageUpdate` payloads:

| Backend event | UI event | Purpose |
|---|---|---|
| `ChatUserMessageAccepted` | `ChatMessageAccepted` | User prompt was accepted and projected into the timeline. |
| `ChatInferenceStarted` | `ChatMessageStarted` | Assistant run started. |
| `ChatTokensDelta` | `ChatMessageAppended` | Assistant text changed during streaming. |
| `ChatInferenceFinished` | `ChatMessageFinished` | Assistant text segment or final response finished. |
| `ChatInferenceStopped` | `ChatMessageStopped` | Assistant run stopped or failed. |

It registers one base timeline entity kind:

| Kind | Protobuf message | Purpose |
|---|---|---|
| `ChatMessage` | `ChatMessageEntity` | User, assistant, thinking, and warning transcript rows. |

## Segment-aware transcript rows

`ChatMessageUpdate` and `ChatMessageEntity` include segment metadata:

| Field | Meaning |
|---|---|
| `message_id` / JSON `messageId` | Stable entity ID for this concrete transcript row. |
| `parent_message_id` / JSON `parentMessageId` | Assistant run ID that owns this row, when the row is a segment. |
| `segment` | One-based segment number within the parent assistant run. |
| `segment_type` / JSON `segmentType` | Logical segment kind, for example `text` or `thinking`. |
| `final` | True only for the final assistant text row of the run. |

This is important for tool loops. A single assistant run can produce:

```text
chat-msg-1:thinking:1
chat-msg-1:text:2
ChatToolCall / ChatToolResult rows
chat-msg-1:thinking:3
chat-msg-1:text:4
```

Timeline stores and Redux reducers upsert by entity ID. Therefore every distinct transcript row must have a distinct `messageId`. Do not reuse the parent assistant ID for multiple thinking blocks or for multiple interleaved assistant text blocks.

## Shared ReasoningPlugin

`pkg/chatapp/plugins.NewReasoningPlugin()` translates Geppetto reasoning events into chatapp/sessionstream events.

It handles:

- `*events.EventThinkingPartial`
- `*events.EventInfo` with `thinking-started`
- `*events.EventInfo` with `thinking-ended`
- reasoning summary info payloads when available

It registers:

| Backend event | UI event | Payload |
|---|---|---|
| `ChatReasoningStarted` | `ChatReasoningStarted` | `google.protobuf.Struct` |
| `ChatReasoningDelta` | `ChatReasoningAppended` | `google.protobuf.Struct` |
| `ChatReasoningFinished` | `ChatReasoningFinished` | `google.protobuf.Struct` |

The payload contains chat-message-shaped fields such as `messageId`, `parentMessageId`, `segment`, `role: "thinking"`, `chunk`, `content`, `status`, and `streaming`. Each contiguous thinking phase gets a segment ID such as `chat-msg-5:thinking:1`.

Use this shared plugin instead of defining app-local runtime-debug/reasoning projection code.

## Shared ToolCallPlugin

`pkg/chatapp/plugins.NewToolCallPlugin()` translates Geppetto tool lifecycle events into typed protobuf payloads.

It handles:

- `*events.EventToolCall`
- `*events.EventToolCallExecute`
- `*events.EventToolResult`
- `*events.EventToolCallExecutionResult`

It registers:

| Backend event / UI event | Payload |
|---|---|
| `ChatToolCallStarted` | `ToolCallUpdate` |
| `ChatToolCallUpdated` | `ToolCallUpdate` |
| `ChatToolCallFinished` | `ToolCallUpdate` |
| `ChatToolResultReady` | `ToolResultUpdate` |

It also registers timeline entity kinds:

| Kind | Payload |
|---|---|
| `ChatToolCall` | `ToolCallEntity` |
| `ChatToolResult` | `ToolResultEntity` |

Use this shared plugin for apps that want durable, hydrated tool-call and tool-result rows. Product-specific tools can still add their own widgets, but they should not duplicate the generic Geppetto tool lifecycle projection.

## Wiring pattern

A web-chat style application wires the base schemas and plugins at server assembly time:

```go
import (
    chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
    chatplugins "github.com/go-go-golems/pinocchio/pkg/chatapp/plugins"
    sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

reg := sessionstream.NewSchemaRegistry()
chatPlugins := []chatapp.ChatPlugin{
    myAppSpecificPlugin(),
    chatplugins.NewReasoningPlugin(),
    chatplugins.NewToolCallPlugin(),
}

if err := chatapp.RegisterSchemas(reg, chatPlugins...); err != nil {
    return err
}

engine := chatapp.NewEngine(chatapp.WithPlugins(chatPlugins...))
```

The reference app uses this pattern from `cmd/web-chat/main.go` and `cmd/web-chat/app/server.go`. `agentmode` remains app-owned under `cmd/web-chat`; reasoning and tool calls are reusable chatapp plugins under `pkg/chatapp/plugins`.

## Frontend implications

The Pinocchio web frontend receives canonical sessionstream frames from `/api/chat/ws` and maps them into local renderer keys:

- `ChatMessage` snapshot entities become local `message` entities.
- thinking rows are represented as `message` entities with `role: "thinking"`.
- app-specific entities such as `AgentMode` keep their own renderer path.

Downstream applications may choose different frontend mappings. The stable contract is the sessionstream event/entity name plus the protobuf payload, not the local React component name.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| Reasoning appears as one overwritten block | Multiple thinking phases reused the same entity ID | Use `ReasoningPlugin` and preserve segment-aware `messageId` values. |
| Tool calls stream live but disappear after reload | Tool calls were only local UI state | Register and use `ToolCallPlugin` so tool calls project into timeline entities. |
| Protobuf payload cannot be decoded | Schema was not registered with the sessionstream registry | Register base chatapp schemas and every plugin before creating the hub. |
| Frontend sees unknown entity kinds | Backend registered new timeline kinds without frontend renderers | Add a renderer or normalize the entity into an existing local renderer kind. |

## See Also

- [Webchat Frontend Integration](webchat-frontend-integration.md)
- [Webchat Frontend Architecture](webchat-frontend-architecture.md)
- [Building Sessionstream React Chat Apps](../tutorials/09-building-sessionstream-react-chat-apps.md)
