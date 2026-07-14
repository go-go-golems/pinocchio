---
Title: JavaScript OAuth runtime injection and local credential management
Ticket: PINOCCHIO-JS-OAUTH-RUNTIME-MANAGEMENT
Status: active
Topics:
    - oauth
    - javascript
    - credentials
    - inference
DocType: index
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: repo://cmd/pinocchio/cmds/auth/login.go
      Note: Existing Glazed local PKCE login command
    - Path: repo://cmd/pinocchio/cmds/js.go
      Note: Registers both JavaScript modules without the resolved bearer source
    - Path: repo://pkg/cmds/profilebootstrap/oauth.go
      Note: Trusted selected-profile source construction
    - Path: repo://pkg/js/modules/pinocchio/module.go
      Note: Pinocchio JS default-engine factory bypasses source options
    - Path: repo://pkg/oauthprofiles/store.go
      Note: Owner-only atomic credential persistence and proposed tuple deletion
    - Path: ws://geppetto/pkg/js/modules/geppetto/module.go
      Note: Released Go-only bearer source injection API
ExternalSources: []
Summary: ""
LastUpdated: 2026-07-14T12:41:38.093893645-04:00
WhatFor: ""
WhenToUse: ""
---


# JavaScript OAuth runtime injection and local credential management

## Overview

<!-- Provide a brief overview of the ticket, its goals, and current status -->

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- oauth
- javascript
- credentials
- inference

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
