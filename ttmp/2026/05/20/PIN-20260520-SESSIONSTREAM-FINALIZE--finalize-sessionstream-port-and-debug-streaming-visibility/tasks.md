# Tasks

## TODO

- [x] Create docmgr ticket and initial design/diary docs.
- [x] Restore stdout-first default command execution instead of routing default `interactive` runs into the TUI.
- [x] Add `--debug-events-jsonl PATH` helper flag and settings plumbing.
- [x] Tee projected chatapp/sessionstream UI events to a protobuf JSONL debug file.
- [x] Add debug JSONL lifecycle frames for hello, snapshots, done, and terminal errors.
- [x] Accumulate append-mode TUI text/reasoning patches so live updates carry current text.
- [x] Add focused unit tests for run-mode selection, debug event files, multi-fanout, and accumulated patches.
- [x] Run targeted validation for `pkg/chatapp`, `pkg/ui`, `pkg/cmds`, and `cmd/pinocchio`.
- [x] Run optional real command/TUI smoke with `--debug-events-jsonl` if API profile/time permits.
- [ ] Commit and push the sessionstream-finalize changes.
- [x] Run reasoning-capable TUI smoke to confirm visible ChatReasoningPatch rendering
