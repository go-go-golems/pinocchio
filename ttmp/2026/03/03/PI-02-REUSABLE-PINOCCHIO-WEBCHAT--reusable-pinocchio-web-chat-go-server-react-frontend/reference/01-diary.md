---
Title: Diary
Ticket: PI-02-REUSABLE-PINOCCHIO-WEBCHAT
Status: active
Topics:
    - webchat
    - react
    - frontend
    - pinocchio
    - refactor
    - thirdparty
    - websocket
    - http-api
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Chronological investigation diary for PI-02 (commands run, findings, decisions).
LastUpdated: 2026-03-03T08:19:11.628874297-05:00
WhatFor: ""
WhenToUse: ""
---

# Diary

## Goal

Keep a chronological, continuation-friendly record of PI-02 work:

- what was investigated (with file anchors),
- what decisions were made,
- what docs/bookkeeping were updated,
- and how the final bundle was validated and uploaded to reMarkable.

## Step 1: Create ticket workspace + seed docs

This step creates the PI-02 ticket workspace and establishes the deliverable structure (design doc, diary, recipes) so subsequent investigation can accumulate in a predictable place. The intended audience is a new intern who needs a single “source of truth” for how to reuse Pinocchio’s webchat backend + React UI from outside `pinocchio/cmd/...`.

### Prompt Context

**User prompt (verbatim):** "Once you are done, create a PI-02 ticket where you study exactly the same thing, but to reuse the web-chat part of pinocchio (so that we can reuse the react and all that as well). reate a detailed analysis / design / implementation guide that is very detailed for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file
  references.
  It should be very clear and detailed. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create a PI-02 docmgr ticket analogous to PI-01, focused on making Pinocchio’s web-chat reusable end-to-end (Go server + React UI), then produce an intern-ready guide in the ticket and bundle-upload it to reMarkable.

**Inferred user intent:** Enable a third-party package/app to reuse webchat without importing `pinocchio/cmd/...`, with clear guidance for extraction/refactor work.

**Commit (code):** N/A

### What I did
- Located/created the PI-02 ticket folder under `pinocchio/ttmp/.../PI-02-REUSABLE-PINOCCHIO-WEBCHAT--...`
- Added three docs:
  - primary design doc: `design-doc/01-reusable-pinocchio-web-chat-analysis-extraction-guide.md`
  - diary: `reference/01-diary.md`
  - recipes: `reference/02-third-party-web-chat-reuse-copy-paste-recipes.md`
- Updated ticket index + tasks + changelog

### Why
- Establish a stable workspace and doc structure before investigation, so later evidence and decisions can be tracked and delivered.

### What worked
- Ticket workspace structure was created with expected folders and files.

### What didn't work
- N/A

### What I learned
- The PI-02 deliverable needs to cover both “reuse today” (backend mostly reusable) and “reuse after refactor” (React package extraction + UI asset export).

### What was tricky to build
- N/A (administrative setup step)

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- Continue with evidence gathering and produce the main analysis content.

### Code review instructions
- Review the generated ticket structure starting at `pinocchio/ttmp/2026/03/03/PI-02-.../index.md`.

### Technical details
- N/A

## Step 2: Evidence gathering (backend + frontend)

This step collects concrete, line-anchored evidence for how the webchat system works today, focusing on the boundaries that impact reuse: app-owned `/chat` and `/ws`, UI embedding, runtime config injection, timeline hydration, and SEM event flows.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Map the codebase (Pinocchio webchat backend + React frontend) and identify what needs refactoring to be reusable.

**Inferred user intent:** Produce a design that is grounded in how the system actually works, not speculation.

**Commit (code):** N/A

### What I did
- Inspected key backend files:
  - `pinocchio/pkg/webchat/doc.go` (ownership model)
  - `pinocchio/pkg/webchat/server.go` / `router.go` / `types.go` (server/router boundaries)
  - `pinocchio/pkg/webchat/conversation.go`, `conversation_service.go`, `stream_coordinator.go`, `stream_hub.go` (lifecycle + streaming)
  - `pinocchio/pkg/webchat/timeline_projector.go`, `timeline_registry.go`, `timeline_js_runtime.go` (projection + JS runtime)
  - `pinocchio/pkg/webchat/http/api.go` and `pinocchio/pkg/webchat/http/profile_api.go` (HTTP handlers)
- Inspected example app entrypoint + glue:
  - `pinocchio/cmd/web-chat/main.go` (go:embed static, app-config.js, root mounting)
  - `pinocchio/cmd/web-chat/timeline_js_runtime_loader.go` (JS loader glue)
- Inspected frontend core:
  - `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx`
  - `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
  - `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
  - `pinocchio/cmd/web-chat/web/src/utils/basePrefix.ts`
  - `pinocchio/cmd/web-chat/web/src/store/*`
- Reviewed upstream internal docs to align terminology:
  - `pinocchio/pkg/doc/topics/webchat-framework-guide.md`
  - `pinocchio/pkg/doc/topics/webchat-http-chat-setup.md`
  - `pinocchio/pkg/doc/topics/webchat-frontend-architecture.md`
  - `pinocchio/pkg/doc/topics/webchat-frontend-integration.md`

### Why
- The PI-02 guide must cite concrete files/lines so a new engineer can verify behavior quickly.

### What worked
- The codebase already has a strong reusable backend core under `pinocchio/pkg/webchat/*`.
- The frontend code has clear extension points (`ChatWidgetProps`, renderer registry, SEM registry).

### What didn't work
- N/A (investigation step)

### What I learned
- “Reuse gap” is primarily packaging:
  - UI assets are embedded from `cmd/web-chat` today.
  - React UI source lives under `cmd/web-chat/web`, which is the wrong place to depend on from third-party code.
  - app-config.js generation and timeline JS script loading glue are still cmd-owned.

### What was tricky to build
- Distinguishing which routes are “core API utilities” vs “app-owned transport” requires reading both `pkg/webchat/doc.go` and the cmd/web-chat wiring.

### What warrants a second pair of eyes
- Confirm whether we want to unify the duplicated timeline HTTP handler implementations (`pkg/webchat/router_timeline_api.go` vs `pkg/webchat/http/api.go`).

### What should be done in the future
- Write the full design/refactor plan document and recipes.

### Code review instructions
- Start with `pinocchio/pkg/webchat/doc.go` and `pinocchio/pkg/webchat/server.go` to understand ownership boundaries.
- Then inspect `pinocchio/cmd/web-chat/main.go` to see how the example app composes handlers and serves UI.

### Technical details
- N/A

## Step 3: Write PI-02 design doc + recipes

This step converts evidence into an intern-ready design/extraction guide, including: (1) current architecture map, (2) contract definitions, (3) refactor recommendations, and (4) phased implementation plan. It also fills a recipes doc with the most common “third-party wiring” snippets.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Produce a detailed, actionable guide and store it in the ticket.

**Inferred user intent:** Hand an intern (or any engineer) a single doc that enables reuse without back-and-forth.

**Commit (code):** N/A

### What I did
- Wrote the main guide:
  - `pinocchio/ttmp/2026/03/03/PI-02-.../design-doc/01-reusable-pinocchio-web-chat-analysis-extraction-guide.md`
- Wrote the copy/paste recipes:
  - `pinocchio/ttmp/2026/03/03/PI-02-.../reference/02-third-party-web-chat-reuse-copy-paste-recipes.md`

### Why
- This is the primary deliverable for PI-02.

### What worked
- The system has clear extension points on both backend (handler-first model) and frontend (registry-based SEM + renderers).

### What didn't work
- N/A

### What I learned
- The most valuable refactors are small “packaging helpers” (UI assets export, app-config helper, JS script loader helper) rather than major backend changes.

### What was tricky to build
- Ensuring the doc stayed evidence-backed while also being prescriptive about future package boundaries.

### What warrants a second pair of eyes
- The proposed UI extraction plan (workspace package layout, publishing strategy) should be reviewed by whoever owns frontend build/release conventions.

### What should be done in the future
- Run ticket bookkeeping and validation steps (`docmgr relate`, `docmgr doctor`) and upload the bundle to reMarkable.

### Code review instructions
- Review the design doc top-to-bottom; verify key claims against referenced files/lines.
- Skim recipes for correctness and naming consistency.

### Technical details
- N/A

## Step 4: Ticket bookkeeping (relations) + `docmgr doctor`

This step links the most important code files into the design/recipes docs’ frontmatter (`RelatedFiles`) and ensures the ticket passes `docmgr doctor` cleanly, including vocabulary hygiene. This is what makes the ticket usable as a long-term reference rather than a one-off markdown dump.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Keep docmgr bookkeeping consistent and validate the ticket before uploading.

**Inferred user intent:** A durable, searchable, “doctor-clean” ticket that can be shared with others.

**Commit (code):** N/A

### What I did
- Related key code files to the main design doc:
  - `docmgr doc relate --root pinocchio/ttmp --doc pinocchio/ttmp/.../design-doc/01-reusable-pinocchio-web-chat-analysis-extraction-guide.md --file-note ...`
- Related key code files to the recipes doc:
  - `docmgr doc relate --root pinocchio/ttmp --doc pinocchio/ttmp/.../reference/02-third-party-web-chat-reuse-copy-paste-recipes.md --file-note ...`
- Related top-level folders to the ticket index:
  - `docmgr doc relate --root pinocchio/ttmp --ticket PI-02-REUSABLE-PINOCCHIO-WEBCHAT --file-note ...`
- Ran doctor and fixed unknown topic vocabulary:
  - `docmgr doctor --root pinocchio/ttmp --ticket PI-02-REUSABLE-PINOCCHIO-WEBCHAT --stale-after 30`
  - `docmgr vocab add --root pinocchio/ttmp --category topics --slug frontend|pinocchio|react|refactor|thirdparty|webchat|websocket --description ...`
  - reran doctor until it passed

### Why
- `RelatedFiles` makes the long-form doc reviewable and keeps the architecture map anchored to code.
- `docmgr doctor` passing cleanly is the “quality gate” before sharing/uploading.

### What worked
- `docmgr doc relate` updated frontmatter relations as expected.
- Vocabulary warnings were resolved by adding missing topic slugs.

### What didn't work
- First `docmgr doctor` run produced unknown-topic warnings (expected due to new topics).

### What I learned
- This workspace shares a single vocabulary file (`temporal-relationships/ttmp/vocabulary.yaml` via repo `.ttmp.yaml`), so new topics must be registered there even for `--root pinocchio/ttmp`.

### What was tricky to build
- Keeping “ticket-local docs” (under `pinocchio/ttmp`) aligned with a “global vocabulary” stored elsewhere.

### What warrants a second pair of eyes
- N/A (mechanical bookkeeping)

### What should be done in the future
- N/A

### Code review instructions
- Open `pinocchio/ttmp/.../design-doc/01-reusable-pinocchio-web-chat-analysis-extraction-guide.md` frontmatter and confirm `RelatedFiles` lists the expected code anchors.
- Run `docmgr doctor --root pinocchio/ttmp --ticket PI-02-REUSABLE-PINOCCHIO-WEBCHAT --stale-after 30` to confirm it stays clean.

### Technical details
- Doctor was clean after adding topic vocab entries.

## Step 5: Bundle upload to reMarkable

This step turns the ticket docs into a single PDF bundle with a ToC and uploads it to a dated folder on the reMarkable cloud, so the report can be consumed away from a laptop.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Upload the finished ticket docs to reMarkable as a bundle.

**Inferred user intent:** Make the report easy to read/review on a reMarkable tablet.

**Commit (code):** N/A

### What I did
- Verified remarquee connectivity:
  - `remarquee status`
  - `remarquee cloud account --non-interactive`
- Ran a dry-run bundle upload:
  - `remarquee upload bundle --dry-run ... --remote-dir /ai/2026/03/03/PI-02-REUSABLE-PINOCCHIO-WEBCHAT --toc-depth 2`
- Ran the real bundle upload:
  - `remarquee upload bundle ... --remote-dir /ai/2026/03/03/PI-02-REUSABLE-PINOCCHIO-WEBCHAT --toc-depth 2`
- Verified the remote folder listing:
  - `remarquee cloud ls /ai/2026/03/03/PI-02-REUSABLE-PINOCCHIO-WEBCHAT --long --non-interactive`

### Why
- Bundle upload produces a single PDF deliverable with a table of contents.

### What worked
- Upload succeeded and the folder listing shows the bundle document.

### What didn't work
- N/A

### What I learned
- Dry-run is a fast way to confirm the bundle composition before producing/uploading the PDF.

### What was tricky to build
- N/A (standard workflow)

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- N/A

### Code review instructions
- Re-run the `remarquee cloud ls` command to confirm the bundle is present.

### Technical details
- Remote dir: `/ai/2026/03/03/PI-02-REUSABLE-PINOCCHIO-WEBCHAT`

## Quick Reference

<!-- Provide copy/paste-ready content, API contracts, or quick-look tables -->

## Usage Examples

<!-- Show how to use this reference in practice -->

## Related

<!-- Link to related documents or resources -->
