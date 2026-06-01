---
Title: Fix timeline JS consume contract mismatches and runtime error propagation
Ticket: GEPA-07-TIMELINE-JS-CONSUME-CONTRACT
Status: active
Topics:
    - gepa
    - pinocchio
    - sem
    - goja
    - bug
    - architecture
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-01T06:35:47.495750602-05:00
WhatFor: "Track detailed analysis and bug-fix planning for timeline JS consume contract mismatches."
WhenToUse: "Use when implementing or reviewing consume semantics, runtime ordering, and error propagation in timeline projection."
---

# Fix timeline JS consume contract mismatches and runtime error propagation

Document workspace for GEPA-07-TIMELINE-JS-CONSUME-CONTRACT.

## Primary Deliverables

1. `design-doc/01-timeline-js-consume-contract-mismatch-analysis-and-bug-fix-design.md`
2. `reference/01-investigation-diary.md`

## Scope

1. Analyze three flagged issues in:
   - `pkg/webchat/timeline_js_runtime.go`
   - `pkg/webchat/timeline_registry.go`
   - `pkg/webchat/timeline_projector.go`
2. Provide a concrete, intern-friendly bug-fix design and testing plan.
