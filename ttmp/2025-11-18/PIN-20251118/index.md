---
Title: Pinocchio config + profile migration
Ticket: PIN-20251118
Status: active
Topics:
    - pinocchio
    - glazed
    - profiles
DocType: index
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/geppetto/pkg/layers/layers.go
      Note: |-
        Geppetto middleware chain - replaced Viper with UpdateFromEnv and LoadParametersFromFiles
        Profile middleware chain entry point referenced
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/glazed/pkg/cli/cli.go
      Note: Profile/command settings layers described
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/.ttmp.yaml
      Note: Configured docmgr root for this repo
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/pinocchio/main.go
      Note: Main CLI entry point - modernized to use InitGlazed and InitLoggerFromCobra, removed Viper dependency
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025-11-18/PIN-20251118/analysis/01-config-and-profile-migration-analysis.md
      Note: Analysis source of lint + profile findings
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025-11-18/PIN-20251118/analysis/02-profile-preparse-options.md
      Note: Profile pre-parse options doc
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025-11-18/PIN-20251118/design/01-profile-loading-plan.md
      Note: Detailed plan for profile parsing fix
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025-11-18/PIN-20251118/playbooks/01-migrating-from-viper-to-glazed-config.md
      Note: Step-by-step playbook for migrating applications from Viper to Glazed config system, based on Pinocchio migration experience
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025-11-18/PIN-20251118/various/02-implementation-diary-config-migration.md
      Note: Detailed implementation diary documenting the migration work, lessons learned, and what would be done differently
    - Path: /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/vocabulary.yaml
      Note: Initial vocabulary for pinocchio tickets
ExternalSources: []
Summary: ""
LastUpdated: 2025-11-18T22:01:50.400616627-05:00
---








---
Title: Pinocchio config + profile migration
Ticket: PIN-20251118
Status: draft
Topics:
  - pinocchio
  - glazed
  - profiles
DocType: index
Intent: short-term
Owners:
  - manuel
RelatedFiles: []
ExternalSources: []
Summary: >
  
LastUpdated: 2025-11-18
---

# Pinocchio config + profile migration

## Overview

<!-- Provide a brief overview of the ticket, its goals, and current status -->

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- pinocchio
- glazed
- profiles

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
