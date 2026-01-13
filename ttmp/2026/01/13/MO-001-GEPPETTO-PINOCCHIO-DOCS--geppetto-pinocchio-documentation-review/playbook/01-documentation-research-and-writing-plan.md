---
Title: Documentation research and writing plan
Ticket: MO-001-GEPPETTO-PINOCCHIO-DOCS
Status: active
Topics:
    - documentation
    - geppetto
    - pinocchio
DocType: playbook
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/ttmp/2026/01/13/MO-001-GEPPETTO-PINOCCHIO-DOCS--geppetto-pinocchio-documentation-review/analysis/01-geppetto-pinocchio-docs-gap-analysis.md
      Note: Context for doc audit and prioritized fixes this plan should support.
ExternalSources: []
Summary: Technical-writer playbook for researching and writing new documentation.
LastUpdated: 2026-01-13T08:42:00-05:00
WhatFor: Provide a repeatable plan for creating accurate, task-focused docs.
WhenToUse: Use when starting new docs or rebuilding stale sections.
---


# Documentation research and writing plan

## Purpose

Provide a repeatable, technical-writer playbook for researching, planning, and writing new documentation that stays aligned with current code and user workflows.

## Environment assumptions

- Local repo checkout is available and builds.
- You can search files and run commands in the workspace.
- Subject matter experts (SMEs) are reachable for targeted questions.

## Plan overview (phases and outcomes)

Phase 0 - Intake and scope
- Outcome: a scoped doc request with clear audiences, tasks, and success criteria.

Phase 1 - Research and discovery
- Outcome: an evidence-backed knowledge base: API mapping, examples, gotchas, and gaps.

Phase 2 - Information architecture
- Outcome: a doc map with the right doc types and an outline for each.

Phase 3 - Drafting and example design
- Outcome: drafts with runnable examples and explicit workflows.

Phase 4 - Validation and review
- Outcome: verified examples, SME sign-off, and a release checklist.

Phase 5 - Release and maintenance
- Outcome: published docs with ownership and drift checks.

## Phase 0 - Intake and scope

### Goals
- Translate the request into a precise doc mission (who, what task, where the doc lives).

### Questions to ask
- Who is the reader (new developer, active maintainer, integrator, ops)?
- What tasks must the doc enable in under 15 minutes?
- What code paths or APIs are in scope?
- What does "done" look like (examples compile, CLI works, workflows validated)?

### Outputs
- A short doc brief (audience, tasks, scope, constraints, success criteria).
- A list of "must cover" workflows and terms.

## Phase 1 - Research and discovery

### Research sources (ordered by truthfulness)
1. Source code (public APIs, current signatures, types, examples).
2. Tests and fixtures (actual usage patterns, edge cases).
3. Example programs and sample configs.
4. Existing docs (identify drift and reuse where valid).
5. SME interviews (clarify intent, roadmap, or deprecations).

### Research checklist
- Map every doc section to a real package, type, or function.
- Identify default values from config layers and flags.
- Note deprecations and removed APIs (mark for callouts).
- Capture example-level constraints (required ordering, env vars, config precedence).
- Capture terminology and type names exactly as code defines them.

### Evidence log (template)
- Source file: <path>
- Symbols: <type/func/const>
- Notes: <behavior, defaults, constraints>
- Risks: <where docs might drift>

## Geppetto/Pinocchio-focused research checklist

Use this section to tailor the plan to the Geppetto and Pinocchio codebases.

### Core workflows to verify
- Profiles and config resolution (Pinocchio CLI + Glazed layers)
  - Source: `geppetto/pkg/layers/layers.go`, `glazed/pkg/config/resolve.go`, `pinocchio/cmd/pinocchio/main.go`
  - Verify: precedence order, profile selection bootstrap, default file locations
- Streaming events and sinks
  - Source: `geppetto/pkg/events/*`, `geppetto/pkg/inference/engine/*`
  - Verify: event types, metadata fields, sink wiring, router usage
- Turn-based tool calling
  - Source: `geppetto/pkg/turns/*`, `geppetto/pkg/inference/toolhelpers/*`, `geppetto/pkg/inference/toolcontext/*`
  - Verify: tool registry via context, per-Turn config via `turns.DataKeyToolConfig`
- Embeddings and caching
  - Source: `geppetto/pkg/embeddings/*`, `geppetto/pkg/embeddings/config/*`
  - Verify: provider defaults, cache types, cache directories, batch embeddings
- Turn serialization and linting
  - Source: `geppetto/pkg/turns/serde/*`, `geppetto/pkg/analysis/turnsdatalint/*`
  - Verify: YAML schema, typed-key rules, fixtures/examples
- Middleware composition
  - Source: `geppetto/pkg/inference/middleware/*`
  - Verify: tool middleware behavior, config types, ordering rules

### Example-first sources to mine
- Geppetto example commands: `geppetto/cmd/examples/*`
- Pinocchio CLI entrypoints: `pinocchio/cmd/*`
- Tests for edge cases: `geppetto/pkg/**/**/*_test.go`

### Geppetto/Pinocchio doc outputs (minimum set)
- Profiles: config search order, CLI helpers, precedence diagram
- Events: full event catalog and UI-facing subset
- Tools: Turn-based workflow and execution hooks
- Embeddings: provider + cache + batch section
- Turns: data model + serde fixtures

## Phase 2 - Information architecture

### Choose doc types by intent
- Tutorial: step-by-step task completion (teaches by doing).
- Topic/guide: conceptual overview with minimal examples.
- Reference: copy-pasteable API or CLI facts.
- Playbook: repeatable procedures (this document).

### Outline rules
- Lead with the outcome and the minimal path to success.
- Expand to advanced or optional paths after the primary workflow.
- Place constraints and pitfalls next to the step that triggers them.
- Link to example programs as the "source of truth" for full code.

### Outputs
- Doc map: list of new or updated docs by type.
- Outline per doc with top-level headings and example slots.

## Phase 3 - Drafting and example design

### Writing approach
- Start with the "happy path" workflow that is minimal yet complete.
- Use consistent naming with code (types, flags, file paths, env vars).
- Show required input before command lines or code snippets.
- Keep examples runnable or explicitly marked as pseudocode.

### Example quality bar
- Must compile or run if presented as runnable.
- Must show realistic, non-trivial input.
- Must demonstrate defaults and how to override them.
- Must state where the output goes and what to expect.

### Structural template (use as a baseline)
1. What you will build/learn
2. Prerequisites
3. Minimal working example
4. Step-by-step walkthrough
5. Common errors and fixes
6. Variants and advanced usage
7. References to related APIs and docs

## Phase 4 - Validation and review

### Validation checklist
- Compile or run all runnable examples.
- Confirm CLI flags and defaults against source.
- Confirm config precedence and file search order.
- Confirm tool or API names are current.
- Confirm terminology matches type names.

### SME review checklist
- Ask SMEs to verify correctness and intended use.
- Ask for expected failure modes and future changes.
- Capture any "tribal knowledge" and convert into doc notes.

## Phase 5 - Release and maintenance

### Release checklist
- Ensure doc links resolve and point to current paths.
- Add changelog entry for doc updates.
- Assign an owner or responsible area.

### Drift prevention
- Add a short "Last verified" note in long-lived tutorials.
- Add a lightweight doc audit cadence (quarterly or per release).
- Link docs to code owners or tests when possible.

## Commands

Use or adapt these commands during research and validation.

```bash
# Discover docs and related code
rg --files | rg "geppetto/pkg/doc|pinocchio/pkg/doc"

# Find API references and validate names
rg "ToolConfig|RunInference|RunToolCallingLoop" geppetto/pkg

# Find defaults and flags
rg "cache-type|profile|profile-settings" geppetto/pkg pinocchio/pkg

# Locate examples
rg --files geppetto/cmd/examples pinocchio/cmd

# Optional: run example or tests when validating
# go test ./geppetto/... -count=1
# go run ./geppetto/cmd/examples/simple-streaming-inference/main.go --help
```

## Exit criteria

- The doc brief is complete and approved.
- All doc sections map to current APIs or workflows.
- Examples are validated (or explicitly marked as pseudocode).
- Known pitfalls and deprecations are called out.
- Release checklist is satisfied (links, changelog, ownership).

## Failure modes and mitigations

- API drift discovered late: pause drafting, update mapping, add deprecation callout.
- Missing SME availability: use code and tests as source of truth, record open questions.
- Example does not run: downgrade to pseudocode, add a TODO to replace.

## Deliverables

- Doc brief
- Research evidence log
- Doc map and outlines
- Drafts with validated examples
- Release checklist
