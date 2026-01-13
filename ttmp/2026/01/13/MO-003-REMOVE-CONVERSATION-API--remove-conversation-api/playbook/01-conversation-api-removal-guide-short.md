---
Title: Conversation API removal guide (short)
Ticket: MO-003-REMOVE-CONVERSATION-API
Status: active
Topics:
    - geppetto
    - pinocchio
    - refactor
    - documentation
DocType: playbook
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/inference/toolhelpers/helpers.go
      Note: Conversation-based helpers still present.
    - Path: geppetto/pkg/js/conversation-js.go
      Note: JS bindings depend on conversation API.
    - Path: geppetto/pkg/turns/conv_conversation.go
      Note: Turn bridge to be removed after migration.
    - Path: pinocchio/pkg/cmds/images.go
      Note: Pinocchio usage of conversation image helpers.
ExternalSources: []
Summary: Short, actionable steps to remove the conversation API and migrate callers.
LastUpdated: 2026-01-13T09:15:00-05:00
WhatFor: Guide the removal of geppetto/pkg/conversation usage and related docs.
WhenToUse: Use when planning the migration to Turn-based APIs.
---


# Conversation API removal guide (short)

## Purpose

Identify and remove remaining uses of `geppetto/pkg/conversation`, migrate call sites to Turn-based APIs, and remove the associated docs.

## Environment assumptions

- Local repo checkout for `geppetto/` and `pinocchio/`.
- Ability to run ripgrep and update Go imports.

## Commands

```bash
# 1) Locate all remaining conversation usages
rg "geppetto/pkg/conversation" -n geppetto pinocchio

# 2) Find conversation-based helpers and bridge code
rg "conversation\." geppetto/pkg/inference geppetto/pkg/turns geppetto/pkg/js pinocchio/pkg

# 3) Find docs referencing conversation API
rg "conversation" geppetto/pkg/doc
```

## Migration checklist (short)

1) **Inventory call sites**
   - Classify each usage as: app logic, examples, tests, JS bindings, or helper utilities.

2) **Replace or remove**
   - Prefer Turn-based structures (`turns.Turn`, blocks, tool registry in context).
   - Replace conversation-based tool helpers with Turn-based equivalents.
   - If a call site only needs image handling, move the helper to a non-conversation utility.

3) **Bridge removal**
   - Remove `geppetto/pkg/turns/conv_conversation.go` once all call sites are migrated.

4) **Docs cleanup**
   - Remove `geppetto/pkg/doc/topics/05-conversation.md` from the public set.
   - Ensure the docs index does not link to the conversation doc.

## Known current usage (start here)

- `pinocchio/pkg/cmds/images.go` (image helper usage)
- `geppetto/pkg/inference/toolhelpers/helpers.go` (conversation helpers)
- `geppetto/pkg/js/conversation-js.go` (JS bindings)
- `geppetto/pkg/turns/conv_conversation.go` (Turn bridge)

## Exit criteria

- No imports of `geppetto/pkg/conversation` remain in `geppetto/` or `pinocchio/`.
- All examples and tests compile with Turn-based APIs.
- Conversation docs removed or replaced by Turn-based guidance.

## Failure modes

- If a migration path is unclear, document the gap in `tasks.md` and flag for design review.
