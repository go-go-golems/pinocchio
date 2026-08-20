---
Title: Make chat message identities restart-safe and replica-safe
Ticket: PINOCCHIO-202
Status: active
Topics:
    - chat
    - backend
    - sessionstream
    - persistence
    - runtime
    - design
    - debugging
DocType: index
Intent: long-term
Owners:
    - manuel
RelatedFiles: []
ExternalSources: []
Summary: "Analysis and implementation plan for replacing process-local chat-msg-N identifiers with injectable, globally unique message identities."
LastUpdated: 2026-08-20T11:07:21.08567224-04:00
WhatFor: "Prevent persisted chat entities from being overwritten when a server restarts or multiple replicas allocate messages concurrently."
WhenToUse: "Use this ticket when implementing, reviewing, testing, or releasing restart-safe root message identity in Pinocchio."
---

# Make chat message identities restart-safe and replica-safe

## Overview

Pinocchio currently allocates root chat message identifiers from an in-memory
counter. After a process restart, the counter returns to its initial value even
though the session store still contains earlier `chat-msg-N` entities. A new
turn can therefore reuse an existing root identifier, and projections keyed by
entity ID merge the new user message and response stream into an earlier turn.

This ticket records the verified failure mechanism and specifies a focused
replacement: globally unique root message identifiers generated through an
injectable service-level API. The default generator uses UUIDv4; tests can
inject deterministic generators. The design deliberately leaves event ordinal
allocation and run identity unchanged.

The research and design package is complete. Production implementation remains
to be performed in the phases recorded in [tasks.md](./tasks.md).

## Key Links

- [Analysis, design, and implementation guide](./design-doc/01-restart-safe-chat-message-identity-analysis-design-and-implementation-guide.md)
- [Investigation diary](./reference/01-investigation-diary.md)
- [Implementation tasks](./tasks.md)
- [Ticket changelog](./changelog.md)
- [GitHub issue #202](https://github.com/go-go-golems/pinocchio/issues/202)

## Status

Current status: **active**

## Topics

- chat
- backend
- sessionstream
- persistence
- runtime
- design
- debugging

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
