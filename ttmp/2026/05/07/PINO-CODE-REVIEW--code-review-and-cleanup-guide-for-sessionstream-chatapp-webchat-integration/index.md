---
Title: Code review and cleanup guide for Pinocchio sessionstream, chatapp, and web-chat integration
Ticket: PINO-CODE-REVIEW
Status: active
Topics:
  - code-review
  - cleanup
  - sessionstream
  - chatapp
  - web-chat
  - observability
DocType: index
Intent: long-term
Owners:
  - manuel
RelatedFiles: []
ExternalSources: []
Summary: Intern-facing architecture review and cleanup plan for Pinocchio's sessionstream/chatapp/web-chat integration, with emphasis on recent debug, protobuf, and observability work.
LastUpdated: 2026-05-07T00:00:00-04:00
WhatFor: Use this ticket to onboard a new contributor and plan cleanup/refactor work around the web-chat runtime pipeline.
WhenToUse: When changing sessionstream transport, chatapp plugins, web-chat server/debug routes, frontend websocket state, or release validation.
---

# PINO-CODE-REVIEW

This ticket contains a code review and cleanup guide for `pinocchio/`, focused on the interaction between:

- Sessionstream event/session transport.
- `pkg/chatapp` domain runtime and plugins.
- `cmd/web-chat` backend server, debug APIs, and React frontend.

## Documents

- [Design guide](design-doc/01-pinocchio-sessionstream-chatapp-webchat-code-review-and-intern-guide.md)
- [Diary](reference/01-diary.md)
- [Tasks](tasks.md)
- [Changelog](changelog.md)
