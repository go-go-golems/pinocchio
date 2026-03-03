---
Title: 'Glazed help pages: TUI integration'
Ticket: PI-02
Status: complete
Topics:
    - pinocchio
    - tui
    - refactor
    - thirdparty
    - bobatea
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/doc/topics/pinocchio-tui-integration-playbook.md
      Note: Glazed help playbook (debugging + ops)
    - Path: pinocchio/pkg/doc/tutorials/06-tui-integration-guide.md
      Note: Glazed help tutorial (intern-first integration guide)
ExternalSources: []
Summary: Pointers to the PI-02 Glazed help pages (intern tutorial + ops playbook) and how to view them locally.
LastUpdated: 2026-03-03T11:01:31.157464019-05:00
WhatFor: ""
WhenToUse: ""
---


# Glazed help pages: TUI integration

## Goal

Make the PI-02 deliverable discoverable via the Pinocchio/Glazed help system: a detailed intern-first integration guide plus an ops/debugging playbook.

## Context

The primary content lives in `pinocchio/pkg/doc/...` so it is embedded into the `pinocchio` binary via `pinocchio/pkg/doc/doc.go` (`go:embed *`) and is accessible through the CLI help system (`help_cmd.SetupCobraRootCommand` wiring).

## Quick Reference

### Help slugs (canonical)

- User guide (intern-first tutorial): `pinocchio-tui-integration-guide`
- Playbook (debugging + ops): `pinocchio-tui-integration-playbook`

### Source files

- Guide: `pinocchio/pkg/doc/tutorials/06-tui-integration-guide.md`
- Playbook: `pinocchio/pkg/doc/topics/pinocchio-tui-integration-playbook.md`

### View locally (from `pinocchio/`)

```bash
go run ./cmd/pinocchio help pinocchio-tui-integration-guide
go run ./cmd/pinocchio help pinocchio-tui-integration-playbook
```

## Usage Examples

### Intern onboarding “first read”

1. Read the guide:

```bash
go run ./cmd/pinocchio help pinocchio-tui-integration-guide
```

2. Keep the playbook open while debugging:

```bash
go run ./cmd/pinocchio help pinocchio-tui-integration-playbook
```

## Related

- Original design docs:
  - `pinocchio/ttmp/2026/03/03/PI-01-REUSABLE-PINOCCHIO-TUI--reusable-pinocchio-tui-third-party-package/design-doc/01-reusable-pinocchio-tui-analysis-extraction-guide.md`
- Extracted backend: `pinocchio/pkg/ui/backends/toolloop/backend.go`
- Extracted forwarder: `pinocchio/pkg/ui/forwarders/agent/forwarder.go`
