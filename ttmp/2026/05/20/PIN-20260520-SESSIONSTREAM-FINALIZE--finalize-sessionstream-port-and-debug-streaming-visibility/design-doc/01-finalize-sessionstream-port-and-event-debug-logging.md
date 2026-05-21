---
Title: Finalize sessionstream port and event debug logging
Ticket: ""
Status: active
Topics:
    - rpc
    - sessionstream
    - debugging
    - tui
    - structured-output
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/pinocchio/doc/general/06-rpc-jsonl-output.md
      Note: Documents --debug-events-jsonl alongside RPC JSONL protocol
    - Path: pkg/chatapp/rpc/jsonl/fanout.go
      Note: Existing protobuf JSONL fanout reused for debug event files
    - Path: pkg/cmds/cmd.go
      Note: Run-mode selection plus RPC/TUI debug JSONL fanout lifecycle
    - Path: pkg/cmds/cmdlayers/helpers.go
      Note: Defines the --debug-events-jsonl helper flag
    - Path: pkg/cmds/event_printer.go
      Note: Restores human-readable streaming output with thinking markers
    - Path: pkg/cmds/event_printer_test.go
      Note: Regression coverage for reasoning-summary pretty printing
    - Path: pkg/cmds/run/context.go
      Note: Carries debug event log path in UI settings
    - Path: pkg/cmds/run_status_fanout.go
      Note: Records ChatRunFinished/Stopped/Failed terminal state for RPC and debug done/error frames
    - Path: pkg/ui/chatapp_fanout.go
      Note: Adapts chatapp UI events to Bubble Tea and now accumulates append patches
    - Path: pkg/ui/multi_fanout.go
      Note: Tees projected UI events to Bubble Tea and debug JSONL fanouts
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---





# Finalize sessionstream port and event debug logging

## Executive summary

The previous migration moved command RPC and command TUI onto `chatapp` + `sessionstream`, but two user-visible regressions remain:

1. Normal command output can be swallowed by the TUI path because the default `interactive` setting now selects the chatapp Bubble Tea run mode before users explicitly ask for chat.
2. Streaming is hard to diagnose because projected sessionstream UI events are only visible through stdout in RPC mode or through Bubble Tea in TUI mode.

This ticket finalizes the port by restoring stdout-first behavior for normal command runs, keeping TUI opt-in via `--chat` / `--force-interactive`, improving Bubble Tea streaming patch handling, and adding a debug JSONL event log flag that writes incoming chatapp/sessionstream UI events to disk.

## Problem statement

The sessionstream migration correctly removed duplicated raw Geppetto event mapping, but it changed the default command experience. Since `interactive` defaults to true, the command can enter `runChat` even when the user expected regular stdout output. That makes scripts and terminal inspection worse: the data goes to the TUI instead of stdout.

A second problem is observability. When the TUI does not visibly stream in real time, there is no easy durable trace of incoming projected UI events. We need a debug flag that records exactly what the chatapp/sessionstream fanout receives, independent of the terminal renderer.

## Proposed solution

### 1. Restore stdout-first default execution

Use blocking stdout mode unless the user explicitly asks for a TUI/RPC transport:

- `--rpc` or `--output jsonl` selects RPC JSONL stdout.
- `--chat` selects chat TUI.
- `--force-interactive` selects interactive TUI for explicit smoke/debug scenarios.
- Plain default execution stays blocking and writes model output to stdout.

This keeps the new chatapp/sessionstream TUI implementation, but no longer routes ordinary command output into Bubble Tea just because `interactive` defaults to true.

### 2. Add `--debug-events-jsonl PATH`

Add a helper-layer flag that opens/truncates `PATH` and stores protobuf JSONL `RpcLine` frames for projected sessionstream UI events. The debug file should not replace stdout. It is a tee for diagnosis.

In RPC mode the file receives the same family of frames as stdout: hello, snapshots, ui events, done/error.

In TUI mode the file receives hello, snapshots, ui events, done/error while Bubble Tea receives the same live UI events through its fanout adapter.

### 3. Make Bubble Tea text updates cumulative

`ChatTextPatch` and `ChatReasoningPatch` may arrive as append deltas. Bubble Tea timeline updates are easier to render reliably if the patch sent to the timeline contains the current accumulated text. The TUI fanout should accumulate per message/stream id and send cumulative text while still honoring snapshot/replace patch modes.

## Implementation plan

1. Add `DebugEventsJSONL` to helper settings and run UI settings.
2. Add `--debug-events-jsonl` to the helpers parameter layer.
3. Add a small multi-fanout adapter so TUI fanout can tee to Bubble Tea and the JSONL file fanout.
4. In `runRPCJSONL`, when the debug path is set, write hello/snapshot/done/error to both stdout and the debug file and tee live UI events through both fanouts.
5. In `runChat`, when the debug path is set, create a JSONL fanout to the file, write session lifecycle frames, and tee live UI events to Bubble Tea plus the file.
6. Change run-mode selection so the default path remains blocking stdout; only `--chat`, `--force-interactive`, `--rpc`, or `--output jsonl` leave blocking mode.
7. Add focused tests for run mode selection, multi fanout, event debug flag plumbing, and cumulative text patches.
8. Run targeted tests and update this ticket diary/changelog.

## Open questions

- Whether append-vs-snapshot TUI behavior should also be documented in the RPC help page.
- Whether backend/raw event frames should be optionally logged in addition to projected UI events. The first implementation should stay with projected UI events because that is the stable client-facing contract.
