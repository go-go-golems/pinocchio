---
Title: Profile-backed OAuth credentials and browser login
Ticket: PINOCCHIO-OAUTH-PROFILE-LIFECYCLE
Status: active
Topics:
    - auth
    - security
    - storage
    - llm
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: 'Detailed implementation plan for Pinocchio profile-backed OAuth credentials, secure persistence, browser login, and Geppetto source injection.'
LastUpdated: 2026-07-10T19:08:01.566772412-04:00
WhatFor: 'Safely implement Pinocchio-owned OAuth profile lifecycle behavior.'
WhenToUse: 'Use when planning or reviewing OAuth profile state, browser login, persistence, or source-aware runtime integration.'
---

# Profile-backed OAuth credentials and browser login

## Overview

This ticket plans Pinocchio’s host side of renewable OAuth credentials: a typed secret-bearing profile extension, owner-only atomic persistence, browser Authorization Code + PKCE login, and injection of Geppetto’s renewable bearer source. Geppetto protocol mechanics are already implemented in the sibling worktree; llm-proxy vault support remains separate follow-on work.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- auth
- security
- storage
- llm

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
