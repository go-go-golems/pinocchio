---
Title: "Third-party basic chat POC (local module)"
Ticket: PI-01-REUSABLE-PINOCCHIO-TUI
Status: draft
Topics:
  - tui
  - pinocchio
  - thirdparty
  - bobatea
DocType: script
Intent: throwaway
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: >
  A local experiment that mimics a third-party Go module consuming Pinocchio’s reusable ChatBuilder.
LastUpdated: 2026-03-03T00:00:00Z
---

# Third-party basic chat POC (local module)

## Purpose

Provide a compile-check-style proof that a third-party Go module can wire Pinocchio’s reusable chat TUI runtime (`pinocchio/pkg/ui/runtime.ChatBuilder`) without importing any `pinocchio/cmd/...` packages.

## Usage

This directory contains its own `go.mod`. Because this repository uses a top-level `go.work`, you’ll typically want to disable workspace mode for this POC:

```bash
cd pinocchio/ttmp/2026/03/03/PI-01-REUSABLE-PINOCCHIO-TUI--reusable-pinocchio-tui-third-party-package/scripts/thirdparty-basic-chat-poc

# Mimic “real third-party” builds outside this monorepo:
GOWORK=off go test ./...
```

## Notes

- As of this ticket, in-repo module `bobatea` declares `go >= 1.25.7`. If your local toolchain is older, you will need to upgrade Go to compile against the local `replace` targets in `go.mod`.
- Running `main.go` requires valid provider settings/API keys; this POC is primarily about *wiring* and *import boundaries*.

