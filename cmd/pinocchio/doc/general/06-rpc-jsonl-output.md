---
Title: Protobuf JSONL RPC Output
Slug: rpc-jsonl-output
Short: Use `--rpc` or `--output jsonl` to stream protobuf-defined Pinocchio chat frames for scripts and subprocess clients.
Topics:
- pinocchio
- cli
- rpc
- jsonl
- protobuf
- chatapp
- sessionstream
Commands:
- pinocchio run-command
Flags:
- rpc
- stdin-rpc
- output
- debug-events-jsonl
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Overview

Pinocchio command verbs can emit a script-friendly stream with `--rpc` or `--output jsonl`. In this mode stdout contains one complete JSON object per line. Each line is the protobuf JSON encoding of `pinocchio.chatapp.rpc.v1.RpcLine`.

This mode is intended for subprocess clients, shell pipelines, editors, and tools that need to consume model output while it is still streaming. It is not a pretty-printer. It is a transport format with a stable protobuf contract.

Use it when you need:

- a clean line-delimited stream on stdout,
- machine-readable start, event, snapshot, error, and done frames,
- typed chat payloads with protobuf `Any` `@type` metadata,
- the same projected chat events used by Pinocchio's chatapp/sessionstream layer.

## Basic Usage

For a command loaded from a YAML file:

```bash
pinocchio run-command ./my-command.yaml --output jsonl
```

The `--rpc` flag selects the same protocol:

```bash
pinocchio run-command ./my-command.yaml --rpc
```

Both forms route the command through the chatapp/sessionstream runner and write protobuf JSONL frames to stdout.

Use `--stdin-rpc` with `--rpc` or `--output jsonl` for the long-lived multi-turn stdin/stdout protocol:

```bash
pinocchio run-command ./my-command.yaml --rpc --stdin-rpc
```

`--stdin-rpc` keeps the process alive, reads one protobuf JSON `RpcRequestLine` per stdin line, emits request-scoped `RpcLine` frames on stdout, and updates an in-memory final-turn accumulator per `session_id`.

If you need logs, keep them on stderr or in a log file. Do not enable log-to-stdout for RPC consumers, because stdout is the protocol stream.

## Multi-turn stdin RPC

The stdin protocol uses `RpcRequestLine`:

```protobuf
message RpcRequestLine {
  uint32 version = 1;
  string session_id = 2;
  string request_id = 3;

  oneof request {
    SubmitPromptRequest submit = 10;
    CancelRequest cancel = 11;
    SnapshotRequest snapshot = 12;
    ShutdownRequest shutdown = 13;
  }
}

message SubmitPromptRequest { string prompt = 1; }
message CancelRequest {}
message SnapshotRequest {}
message ShutdownRequest {}
```

Example session:

```jsonl
{"version":1,"sessionId":"demo","requestId":"r1","submit":{"prompt":"first question"}}
{"version":1,"sessionId":"demo","requestId":"r2","submit":{"prompt":"follow-up question"}}
{"version":1,"sessionId":"demo","requestId":"r3","shutdown":{}}
```

Every stdout frame emitted while handling a request is stamped with that request's `requestId`. `submit` requests stream normal `uiEvent` frames, a final `snapshot`, and a `done` frame. `snapshot` requests emit a snapshot and `done`. `shutdown` emits `done.status = "shutdown"` and exits.

The first implementation is process-local: session accumulators are held in memory as final `turns.Turn` values. It does not yet provide external tool-result submission; tool-call lifecycle events can be reported through normal UI event frames when tool plugins are enabled.

## Debug Event Files

Use `--debug-events-jsonl PATH` when you want to keep normal stdout behavior but also capture the projected chatapp/sessionstream events that are entering the RPC/TUI adapter:

```bash
pinocchio run-command ./my-command.yaml \
  --debug-events-jsonl /tmp/pinocchio-events.jsonl
```

In regular text mode this flag does not turn stdout into JSONL. It writes the debug stream to the requested file while stdout remains the command output.

In `--chat` / `--force-interactive` mode it records the same projected UI events that the Bubble Tea adapter receives. This is useful when the terminal UI does not appear to stream in real time and you want to check whether events are arriving incrementally.

In `--rpc` / `--output jsonl` mode the debug file receives the same protobuf `RpcLine` family as stdout: `hello`, `snapshot`, live `uiEvent` frames, terminal `error` frames, and `done`.

The file is created or truncated on each run. Parent directories are created automatically.

## Frame Shape

Every line is a `RpcLine` message:

```protobuf
message RpcLine {
  uint32 version = 1;
  string session_id = 2;
  string request_id = 3;

  oneof frame {
    HelloFrame hello = 10;
    SnapshotFrame snapshot = 11;
    UiEventFrame ui_event = 12;
    BackendEventFrame backend_event = 13;
    ErrorFrame error = 14;
    DoneFrame done = 15;
  }
}
```

A normal stream starts with `hello`, then usually an initial `snapshot`, then zero or more `uiEvent` frames, then a final `snapshot`, then `done`.

Errors are represented as `error` frames. Terminal setup errors, submit errors, wait errors, and snapshot errors are emitted as terminal error frames when the JSONL transport has already been initialized.

## Typed Payloads and `google.protobuf.Any`

UI event and snapshot payloads are protobuf messages packed into `google.protobuf.Any`. In JSON, an `Any` payload contains an `@type` field.

Example text patch frame:

```json
{
  "version": 1,
  "sessionId": "session-123",
  "uiEvent": {
    "ordinal": "7",
    "name": "ChatTextPatch",
    "payload": {
      "@type": "type.googleapis.com/pinocchio.chatapp.v1.ChatTextPatch",
      "messageId": "chat-msg-1:text:segment-1",
      "role": "assistant",
      "text": "partial answer",
      "mode": "CHAT_STREAM_PATCH_MODE_SNAPSHOT",
      "status": "streaming"
    }
  }
}
```

The event name is useful for quick shell filters. The `@type` is the stronger contract for clients that unpack the payload into generated protobuf types.

Common payload types include:

- `pinocchio.chatapp.v1.ChatUserMessageAccepted`
- `pinocchio.chatapp.v1.ChatRunStarted`
- `pinocchio.chatapp.v1.ChatTextSegmentStarted`
- `pinocchio.chatapp.v1.ChatTextPatch`
- `pinocchio.chatapp.v1.ChatTextSegmentFinished`
- `pinocchio.chatapp.v1.ChatRunFinished`
- `pinocchio.chatapp.v1.ChatRunFailed`
- `pinocchio.chatapp.v1.ChatMessageEntity`

## Protobuf JSON Numbers

Protobuf JSON encodes `uint64` fields as strings. Ordinals therefore look like this:

```json
{"ordinal":"42"}
```

Use `tonumber` in `jq` when you need numeric comparisons or sorting:

```bash
pinocchio run-command ./my-command.yaml --output jsonl \
  | jq 'select(.uiEvent) | .uiEvent.ordinal | tonumber'
```

This is normal protobuf JSON behavior. Do not treat quoted ordinals as a Pinocchio-specific workaround.

## Extract Streaming Text Patches

To print the accumulated assistant text snapshots as they stream:

```bash
pinocchio run-command ./my-command.yaml --output jsonl \
  | jq -r '
      select(.uiEvent.name == "ChatTextPatch")
      | .uiEvent.payload.text
    '
```

If you only want payloads that are explicitly typed as `ChatTextPatch`, filter by `@type`:

```bash
pinocchio run-command ./my-command.yaml --output jsonl \
  | jq -r '
      select(.uiEvent.payload["@type"] == "type.googleapis.com/pinocchio.chatapp.v1.ChatTextPatch")
      | .uiEvent.payload.text
    '
```

## Extract Final Assistant Text

To print final assistant text segments:

```bash
pinocchio run-command ./my-command.yaml --output jsonl \
  | jq -r '
      select(.uiEvent.name == "ChatTextSegmentFinished")
      | select(.uiEvent.payload.role == "assistant")
      | .uiEvent.payload.content // .uiEvent.payload.text
    '
```

This is usually the best shell-level equivalent of "give me the final answer".

## Extract Tool Results

Tool payload names depend on the enabled chatapp tool plugins. When tool-call payloads are present, filter by the protobuf `@type` rather than guessing from text:

```bash
pinocchio run-command ./my-command.yaml --output jsonl \
  | jq '
      select(.uiEvent.payload["@type"]? | test("pinocchio.chatapp.v1.ChatTool"))
      | .uiEvent
    '
```

For a specific tool result payload, replace the predicate with the exact generated type URL used by that payload.

## Done and Error Frames

A successful run ends with a `done` frame:

```bash
pinocchio run-command ./my-command.yaml --output jsonl \
  | jq 'select(.done)'
```

A terminal error frame has `error.terminal == true`:

```bash
pinocchio run-command ./my-command.yaml --output jsonl \
  | jq 'select(.error.terminal == true) | .error'
```

A robust subprocess client should watch for both:

- `done` means the adapter reached normal completion;
- terminal `error` means the adapter reached a non-recoverable error after it could write JSONL frames.

The process exit status still matters. Treat a non-zero exit as failure even if you saw some frames before the process exited.

## Relationship To Text, JSON, and YAML Output

`--output text`, `--output json`, and `--output yaml` are command output modes for existing users. They are not the RPC protocol.

`--output jsonl` and `--rpc` are transport modes. They use chatapp/sessionstream projections and protobuf JSON lines. Use them for automation.

## TUI Chat Persistence

Command TUI mode can persist both the model-context turn history and the visible sessionstream timeline:

```bash
pinocchio run-command ./my-command.yaml \
  --chat \
  --turns-db ~/.local/share/pinocchio/chat/turns.db \
  --timeline-db ~/.local/share/pinocchio/chat/timeline.db
```

`--turns-db` / `--turns-dsn` stores successful final `turns.Turn` snapshots after each TUI inference run. This is the model-context accumulator that should seed future turns.

`--timeline-db` / `--timeline-dsn` stores the live `sessionstream` hydration timeline: projected UI entities, ordinals, and snapshots. This is the visible UI/debug/export state, not the model-context source.

The two databases intentionally serve different purposes:

| Flag | Stores | Used for |
|---|---|---|
| `--turns-db` / `--turns-dsn` | Final Geppetto `turns.Turn` YAML payloads | durable model-context turns, export, future resume support |
| `--timeline-db` / `--timeline-dsn` | `sessionstream` timeline entities and snapshots | UI hydration, debug inspection, export |

To persist under an explicit stable key, pass `--session-id`:

```bash
pinocchio run-command ./my-command.yaml \
  --chat \
  --session-id project-notes \
  --turns-db ~/.local/share/pinocchio/chat/turns.db \
  --timeline-db ~/.local/share/pinocchio/chat/timeline.db
```

To resume model context from the latest persisted final turn for that session, add `--resume`:

```bash
pinocchio run-command ./my-command.yaml \
  --chat \
  --session-id project-notes \
  --resume \
  --turns-db ~/.local/share/pinocchio/chat/turns.db \
  --timeline-db ~/.local/share/pinocchio/chat/timeline.db
```

`--resume` requires `--session-id` and a configured turns DB/DSN. The first implementation uses the simple keying rule `conv_id = session_id = --session-id`.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `jq` fails before the first frame | Logs or other text were written to stdout | Keep logs on stderr or use `--log-file`; do not use `--log-to-stdout` with RPC mode. |
| Ordinals compare incorrectly | Protobuf JSON encodes `uint64` values as strings | Use `tonumber` in `jq`. |
| Payload shape is not what the script expected | The script filtered only by event name | Filter by `payload["@type"]` or unpack with generated protobuf code. |
| The stream has events but no `done` frame | The process terminated before normal adapter completion | Check exit status and stderr; treat missing `done` as incomplete. |
| The first frames appear and then an error appears | Runtime initialization or submit failed after transport startup | Read the terminal `error` frame and process exit status. |

## See Also

- `pinocchio help profile-resolution-runtime-switching`
- `pinocchio help tui-integration-guide`
- `pinocchio help chatapp-protobuf-plugins`
