---
Title: Adopt a profile-first unified config format for pinocchio
Ticket: PI-PROFILE-FIRST-CONFIG
Status: active
Topics:
    - config
    - pinocchio
    - profiles
    - design
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: >
  Research and design ticket for replacing Pinocchio's split between top-level
  runtime config and external engine-profile registries with one layered unified
  config document containing app settings, profile selection, and inline
  profiles, while keeping external registries as optional imported catalogs.
LastUpdated: 2026-04-14T22:55:00-04:00
WhatFor: >
  Plan a profile-first config format that is much easier to explain, easier to
  override locally, and easier for new contributors to implement safely.
WhenToUse: >
  Use this ticket when designing or implementing the next-generation Pinocchio
  config document, migrating away from top-level ai-chat style runtime config,
  or explaining how inline profiles and external registries should coexist.
---

# Adopt a profile-first unified config format for pinocchio

## Overview

This ticket captures the proposed redesign of Pinocchio configuration after the recent declarative config-plan cleanup.

The current system already has a good *loading* story:

- layered discovery through Glazed config plans,
- provenance-aware loading,
- repo and cwd local overrides,
- explicit-file precedence,
- and a reusable Geppetto bootstrap contract.

What is still awkward is the *document model* that sits on top of that loader.

Today, Pinocchio runtime behavior is split across two different concepts:

1. top-level Geppetto runtime sections in config files, such as `ai-chat`, and
2. external engine-profile registries selected through `profile-settings.profile` and `profile-settings.profile-registries`.

That split is powerful, but it is not easy to explain to a newcomer. It also creates unnecessary friction for local workflows because local override files look like “profile” files but are actually only partial overlays on a broader config model.

This ticket proposes a clearer future model:

- one layered config document format,
- one local override story,
- one place for app settings,
- one place for runtime profiles,
- and external registries treated as optional imported catalogs rather than as the primary everyday config mechanism.

## Current Status

This ticket is a research/design ticket, not an implementation ticket. The design and implementation guidance are ready for a future coding pass.

## Key Documents

- [Current architecture analysis](./analysis/01-current-profile-config-and-registry-architecture-analysis.md)
- [Primary design document](./design-doc/01-profile-first-unified-config-format-and-migration-design.md)
- [Intern-oriented implementation guide](./reference/01-implementation-guide-for-the-profile-first-config-format.md)
- [Investigation diary](./reference/02-investigation-diary.md)
- [Tasks](./tasks.md)
- [Changelog](./changelog.md)

## Core Recommendation

Adopt one unified Pinocchio config document with three semantic blocks:

- `app` for non-runtime application settings,
- `profile` for selection and imports,
- `profiles` for inline runtime profiles.

Illustrative shape:

```yaml
app:
  repositories:
    - ~/prompts

profile:
  active: assistant
  registries:
    - ~/.pinocchio/profiles.yaml

profiles:
  default:
    display_name: Default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini

  assistant:
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5
```

## Why This Ticket Exists

This redesign is worth a dedicated ticket because it touches all of the following:

- Pinocchio config file naming and shape,
- Geppetto bootstrap responsibilities,
- engine-profile registry composition,
- runtime profile switching,
- web-chat and JS command profile consumers,
- current docs and migration examples,
- and the distinction between app settings and runtime settings.

## Structure

- `analysis/` — evidence-based current-state architecture and constraints
- `design-doc/` — proposed target architecture and migration plan
- `reference/` — implementation guide and diary for future coding work
- `playbooks/` — reserved for future validation sequences if implementation starts
- `scripts/` — reserved for temporary migration tooling if implementation starts

## Success Criteria For The Future Implementation

The implementation that follows this ticket should make the following statement true:

> A newcomer can understand Pinocchio config as “one layered document with app settings plus profiles,” without having to first understand a separate hidden top-level runtime config model and a separate registry-only profile model.
