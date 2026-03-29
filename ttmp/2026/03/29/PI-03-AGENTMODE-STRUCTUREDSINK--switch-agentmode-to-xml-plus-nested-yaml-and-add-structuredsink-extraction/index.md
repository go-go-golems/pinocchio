---
Title: Switch agentmode to XML plus nested YAML and add structuredsink extraction
Ticket: PI-03-AGENTMODE-STRUCTUREDSINK
Status: active
Topics:
    - pinocchio
    - structured-sinks
    - webchat
    - yaml
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-29T16:28:55.537625176-04:00
WhatFor: "Plan the migration of Pinocchio agentmode to a structured XML-like tag with nested YAML, shared sanitize-backed parsing, structuredsink extraction, and web-chat integration."
WhenToUse: "Use when implementing or reviewing PI-03 or when orienting a new engineer on the current agentmode and structuredsink architecture."
---

# Switch agentmode to XML plus nested YAML and add structuredsink extraction

## Overview

This ticket documents how to migrate Pinocchio `agentmode` from free-form fenced YAML parsing to a structured XML-like tag with nested YAML, shared sanitize-backed parsing, and a dedicated structured extractor. The main guide also explains where Pinocchio web-chat already has the necessary sink-wrapper and SEM seams and what still needs to be changed.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field
- **Primary design doc**: `design-doc/01-intern-guide-to-switching-agentmode-to-xml-plus-nested-yaml-and-structuredsink-extraction.md`
- **Diary**: `reference/01-investigation-diary.md`

## Status

Current status: **active**

Implementation status:

- Shared sanitize-backed parser landed in `agentmode`
- Middleware migrated to the shared parser
- Agentmode structuredsink extractor added
- Web-chat sink wrapper integration added
- Existing `agent.mode` SEM mapping retained
- SEM ingress log reduced from debug to trace

## Topics

- pinocchio
- structured-sinks
- webchat
- yaml

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

## Current conclusion

The investigation found that:

- `agentmode` currently parses fenced YAML directly in the middleware.
- Geppetto already provides the correct structured streaming abstraction through `FilteringSink`.
- Pinocchio web-chat already exposes an event-sink wrapper seam and already maps `EventAgentModeSwitch` into `agent.mode` SEM frames.
- The missing work is the protocol migration, shared parser extraction, `sanitize/pkg/yaml` integration, and concrete structured extractor wiring.
