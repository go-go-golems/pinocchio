# Diary

## Goal

This diary records the creation of a Pinocchio-focused code review ticket and intern-facing cleanup guide for the sessionstream, chatapp, and web-chat integration surface.

## Step 1: Create the ticket and map the codebase

I created a new ticket under `pinocchio/ttmp` and started with inventory rather than edits. The goal was to understand the code as an intern would encounter it: package layout, runtime flow, recent diaries, large files, and the seams where Sessionstream, Chatapp, web-chat backend, and the React client meet.

### Prompt Context

**User prompt (verbatim):**

```text
Now do a similar code review but for pinocchio/ and in particular around the session stream and chatapp and web-chat integration.
Create a new ticket in pinocchio/ttmp  to do a code review of the whole package, and especially the lately add code / improvements (see diaries too).

Now do a similar code review but for pinocchio/ and in particular around the session stream and chatapp and web-chat integration.

We are looking for not just issues, but also deprecated code, unclear code, too long files, code that could be organized better, too large packages, overengineered functionality, etc...

Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.

Keep a diary as you work.
──────────────────────────────────────────────────────────
```

**Assistant interpretation:** Create a new Pinocchio ticket, inspect the repository with emphasis on sessionstream/chatapp/web-chat and recent debug/protobuf/observability work, write a detailed onboarding/code-review guide with cleanup recommendations, upload it to reMarkable, and maintain a diary.

**Inferred user intent:** Produce a durable intern-readable architecture and cleanup artifact comparable to the Geppetto code review guide, but focused on Pinocchio's web-chat runtime integration and maintainability risks.

### What I did
- Checked `pinocchio` worktree status.
- Counted repository surface area and largest Go/TypeScript files.
- Listed Go packages with `go list ./...`.
- Searched recent ticket diaries for `sessionstream`, `web-chat`, `chatapp`, hydration, debug, and observer references.
- Read key files in `pkg/chatapp`, `pkg/chatapp/plugins`, `cmd/web-chat/app`, `cmd/web-chat`, and `cmd/web-chat/web/src/ws`.
- Created the ticket skeleton under `ttmp/2026/05/07/PINO-CODE-REVIEW--code-review-and-cleanup-guide-for-sessionstream-chatapp-webchat-integration`.

### Why
- The integration spans multiple packages and a sibling module (`sessionstream`), so a useful review must explain runtime ownership before listing problems.
- Recent work added typed protobuf payloads, debug recorder/export paths, Geppetto observability records, and provider IDs on reasoning payloads; these are valuable but increase coupling and validation complexity.

### What worked
- `pinocchio` was clean before the review.
- Inventory quickly identified major complexity hot spots:
  - `pkg/chatapp/chat.go` at about 802 lines.
  - `cmd/web-chat/app/debug_reconcile_db.go` at about 778 lines.
  - `cmd/web-chat/app/debug_recorder.go` at about 473 lines.
  - `cmd/web-chat/main.go` at about 451 lines.
  - `cmd/web-chat/web/src/ws/wsManager.ts` at about 432 lines.
  - `pkg/chatapp/plugins/reasoning.go` at about 483 lines.

### What didn't work
- No implementation blocker yet.
- The broader workspace still has unrelated dirty Sessionstream files, so this ticket should avoid modifying Sessionstream until that worktree is intentionally handled.

### What I learned
- The core ownership split is good: Sessionstream owns event/session substrate; Chatapp owns chat domain translation; `cmd/web-chat/app` owns HTTP/debug/export integration; the React app owns rendering and frontend debug capture.
- The code mostly uses typed protobuf payloads now, but the frontend still hand-maps JSON payloads into generic timeline props.
- Debug/reconcile functionality is powerful but concentrated in large files that mix schema, inserts, views, parsing, and provider-specific correlation logic.

### What was tricky to build
- The history has multiple overlapping migrations: legacy `/chat` routes removed, protobuf schema enforcement, stream debug APIs, Geppetto observability, and provider IDs on `ReasoningUpdate`. The guide needs to distinguish settled architecture from recent scaffolding and from obsolete paths.

### What warrants a second pair of eyes
- Whether the recommended package splits preserve enough locality for a small team.
- Whether debug SQLite exports should remain in `cmd/web-chat/app` or move to a small internal package once the schema stabilizes.
- Whether frontend event mapping should be schema/generated now or after one more round of payload stabilization.

### What should be done in the future
- Upload the final guide bundle to reMarkable.
- Commit the ticket docs after upload succeeds.

### Code review instructions
- Start with the design guide's runtime flow diagram.
- Then review issues in priority order: release alignment, `chat.go` split, debug SQLite split, frontend websocket split, reasoning segment state, and typed frontend payload mapping.

### Technical details
- Inventory commands included:
  - `git status --short`
  - `rg --files | wc -l`
  - `find . -type f -name '*.go' -print | xargs wc -l | sort -nr | head -40`
  - `go list ./...`
  - `rg -n "sessionstream|web-chat|chatapp|ReasoningUpdate|debug_recorder|reconcile|hydration|observer" ...`

## Step 2: Upload the guide bundle to reMarkable

After drafting the guide, I bundled the ticket's index, design guide, diary, task list, and changelog into a single PDF and uploaded it to the reMarkable ticket folder. The first upload attempt found a real documentation-formatting problem: the verbatim prompt in the diary used literal `\n` sequences inside a quoted Markdown paragraph, which caused Pandoc/LaTeX to treat `\nCreate` as an undefined TeX control sequence.

I fixed the diary prompt formatting by moving the prompt into a fenced `text` block. The dry-run then succeeded, the upload completed, and the remote listing confirmed the document is present.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Publish the intern-facing review bundle to reMarkable after writing it.

**Inferred user intent:** Make the analysis easy to read/review on the reMarkable, not only inside the repository.

### What I did
- Ran `remarquee status`.
- Ran a dry-run bundle upload for:
  - `index.md`
  - `design-doc/01-pinocchio-sessionstream-chatapp-webchat-code-review-and-intern-guide.md`
  - `reference/01-diary.md`
  - `tasks.md`
  - `changelog.md`
- Fixed the diary prompt formatting after Pandoc failed.
- Re-ran the dry-run successfully.
- Uploaded the bundle to:
  - `/ai/2026/05/07/PINO-CODE-REVIEW/PINO-CODE-REVIEW Sessionstream Chatapp Webchat Code Review.pdf`
- Verified the remote listing with `remarquee cloud ls /ai/2026/05/07/PINO-CODE-REVIEW --long --non-interactive`.

### Why
- The user explicitly requested upload to reMarkable.
- Bundling the ticket docs gives one navigable PDF with the guide and working diary.

### What worked
- `remarquee status` returned `remarquee: ok`.
- Final upload succeeded:
  - `OK: uploaded PINO-CODE-REVIEW Sessionstream Chatapp Webchat Code Review.pdf -> /ai/2026/05/07/PINO-CODE-REVIEW`
- Remote listing showed:
  - `[f] PINO-CODE-REVIEW Sessionstream Chatapp Webchat Code Review`

### What didn't work
- First upload failed with:
  - `pandoc failed: Error producing PDF.`
  - `! Undefined control sequence.`
  - `l.1554 web-chat integration.\nCreate`
- Root cause: the diary stored the long prompt in a quoted inline paragraph with literal backslash-n sequences. Pandoc/LaTeX interpreted the backslash command.

### What I learned
- Long verbatim prompts should be stored as fenced `text` blocks in diary docs, especially when they contain backslashes or copied terminal separators.

### What was tricky to build
- The upload problem was not in the design guide; it was in the diary metadata. The fix preserved the prompt content while making it safe for Markdown-to-PDF conversion.

### What warrants a second pair of eyes
- The guide is intentionally broad. A reviewer should check whether any of the cleanup recommendations should be split into separate tickets before implementation.

### What should be done in the future
- Commit the ticket docs.
- If this ticket is later managed by docmgr, reconcile the manual `pinocchio/ttmp` ticket path with the workspace's current docmgr root configuration.

### Code review instructions
- Review the uploaded PDF for readability and the repository Markdown for precise file references.
- If modifying the diary prompt, preserve fenced formatting so future uploads continue to render.

### Technical details
- Upload command used `remarquee upload bundle ... --remote-dir /ai/2026/05/07/PINO-CODE-REVIEW --toc-depth 2`.
