# Tasks

## Investigation

- [x] Read the GP-031 playbook and confirm which runner-first webchat architecture decisions must be preserved.
- [x] Inspect `web-agent-example`, `go-go-os-chat`, `wesen-os`, and linked app modules for current `pinocchio` integration assumptions.
- [x] Reproduce the compile failure modes with concrete `go` commands and capture the exact errors in the diary.

## Fix

- [x] Restore a root workspace overlay so sibling modules resolve against the local `pinocchio`, `geppetto`, and app modules again.
- [x] Align `wesen-os/go.work` with the `go 1.26.1` requirement introduced by the updated `pinocchio` checkout.
- [x] Mirror the local `go-go-os-chat v0.0.0` workspace replacement semantics needed by `wesen-os` and linked app modules.

## Validation

- [x] Verify `web-agent-example` builds against the restored workspace.
- [x] Verify `go-go-os-chat/pkg/...` builds against the restored workspace.
- [x] Verify `wesen-os/pkg/assistantbackendmodule` builds against the restored workspace.
- [x] Verify linked downstream apps (`go-go-app-inventory`, `go-go-app-arc-agi-3`) still build.
- [x] Run `docmgr doctor --ticket APP-12-WEBCHAT-RUNNER-FIRST-REBASE --stale-after 30`.
- [x] Upload the ticket bundle to reMarkable and verify the remote listing.
