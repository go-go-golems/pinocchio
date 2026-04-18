---
Title: Fix Pinocchio profile env and explicit selection resolution
Ticket: PIN-20260418-PROFILE-ENV-RESOLUTION
Status: active
Topics:
    - pinocchio
    - profiles
    - cli
    - bootstrap
    - configuration
    - runtime
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/cli/bootstrap/bootstrap_test.go
      Note: Current shared test that rejects implicit registry fallback
    - Path: geppetto/pkg/cli/bootstrap/engine_settings.go
      Note: Profile-to-engine merge path that consumes selection and registry chain
    - Path: geppetto/pkg/cli/bootstrap/profile_registry.go
      Note: Strict validation point for empty registry sources when profile is set
    - Path: glazed/pkg/config/plan.go
      Note: Generic layered config plan infrastructure relevant to the ecosystem-wide override pattern
    - Path: pinocchio/cmd/pinocchio/cmds/js.go
      Note: JS runtime bootstrap path that also resolves profile registries
    - Path: pinocchio/cmd/web-chat/main.go
      Note: Command-level guard that mirrors profile-registries validation
    - Path: pinocchio/pkg/cmds/helpers/parse-helpers.go
      Note: Secondary helper path that reads PINOCCHIO_PROFILE directly
    - Path: pinocchio/pkg/cmds/profilebootstrap/profile_selection.go
      Note: Shared Pinocchio profile bootstrap wrapper and app bootstrap config
ExternalSources: []
Summary: ""
LastUpdated: 2026-04-18T13:28:00.451193783-04:00
WhatFor: ""
WhenToUse: ""
---



# Fix Pinocchio profile env and explicit selection resolution

## Overview

Pinocchio’s profile bootstrap currently rejects `--profile` and `PINOCCHIO_PROFILE` unless `profile-settings.profile-registries` is already populated. That breaks the documented contract that Pinocchio should discover the default `profiles.yaml` registry when present.

This ticket documents the architecture, the observed failure, the expected behavior, and the implementation path for restoring default registry discovery in the shared Pinocchio bootstrap layer.

## Key Links

- **Design doc 1**: `design-doc/01-pinocchio-profile-env-and-explicit-profile-resolution-design.md`
- **Design doc 2**: `design-doc/02-shared-assessment-centralize-profile-registry-discovery-and-loading-in-geppetto-bootstrap.md`
- **Investigation diary**: `reference/01-investigation-diary.md`
- **Primary code references**: see the design doc references sections

## Status

Current status: **active**

## Topics

- pinocchio
- profiles
- cli
- bootstrap
- configuration
- runtime

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design-doc/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
