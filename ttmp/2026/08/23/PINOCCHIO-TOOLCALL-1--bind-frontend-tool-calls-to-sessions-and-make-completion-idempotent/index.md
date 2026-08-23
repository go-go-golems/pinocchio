---
Title: 'Bind frontend tool calls to sessions and make completion idempotent'
Ticket: PINOCCHIO-TOOLCALL-1
Status: active
Topics: [security, chatapp, backend, runtime]
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: 'Implementation ticket for hardening Pinocchio frontend-tool pending identity, result validation, terminal completion, manifests, and bridge observability.'
LastUpdated: 2026-08-23T17:30:00-04:00
WhatFor: 'Landing page for the Pinocchio frontend-tool bridge hardening guide, diary, tasks, and delivery.'
WhenToUse: 'Before changing pkg/chatapp/frontendtools or the frontend-tool protobuf.'
---

# Bind frontend tool calls to sessions and make completion idempotent

## Start here

1. [Frontend-tool bridge hardening design and implementation guide](./design-doc/01-pinocchio-frontend-tool-bridge-hardening-invocation-identity-result-validation-implementation-guide.md)
2. [Diary](./reference/01-diary.md)
3. [Tasks](./tasks.md)
4. [Changelog](./changelog.md)

## Scope

This ticket owns the Go server half of frontend browser tools: composite invocation keys, exact session/tool result binding, duplicate insertion protection, terminal idempotency, cancellation, manifest identity, protobuf evolution, bridge context, timeline projection, and race/security tests.

PBUI route/policy/effect work belongs to `PBUI-TOOLCALL-1`. Browser runtime execution belongs to `REACT-CHAT-TOOL-RUNTIME-1`.
