# Diary

## Goal

Investigate where reasoning-summary text loses its line breaks: backend emission, websocket transport, persistence, or frontend rendering.

## Step 1: Instrumentation plan

I started by mapping the likely seams where the newline shape could change: the reasoning feature in the web-chat backend, the websocket payloads received by the browser, and the frontend card/markdown render path. The goal is to capture the same summary text at each boundary and compare the raw string representation rather than trusting the rendered view.

### Prompt Context

**User prompt (verbatim):** "ok, do it. create tasks, and then go through them and analyze the logs, you can launch the server with timelines db and then lookat it. keep a detailed diary"

**Assistant interpretation:** The user wants an investigation workflow, including temporary task tracking, reproducible logging, a running server with SQLite persistence, and a chronological diary of findings.

**Inferred user intent:** Determine exactly where the reasoning-summary formatting changes, with evidence from logs and persisted state, and document the investigation thoroughly.

### What I did
- Created `pinocchio/ttmp/2026/04/22/01-thinking-summary-format-debug/tasks.md` with the investigation checklist.
- Created `pinocchio/ttmp/2026/04/22/01-thinking-summary-format-debug/diary.md` as the running investigation log.

### Why
- Temporary tasks keep the inspection focused and let me mark off each validation layer in order.
- A diary is useful here because the evidence will come from several sources and the sequence matters.

### What worked
- The repository already has a canonical `cmd/web-chat` server with a SQLite-backed persistence path, so I can exercise the real stack instead of fabricating a toy repro.

### What didn't work
- N/A yet; this is the planning step.

### What I learned
- The main app exposes a reasoning feature that already handles `thinking-summary` as a special terminal-ish event, which makes it a good seam for temporary debug logging.

### What was tricky to build
- The tricky part is not adding more logic, but choosing the right observation points so we can tell whether the string is changing before it reaches persistence or only when ReactMarkdown renders it.

### What warrants a second pair of eyes
- Any temporary logging should stay narrow and not disturb event ordering or the existing reasoning-summary semantics.

### What should be done in the future
- Add the actual instrumentation and then remove or gate it once the diagnosis is complete.

### Code review instructions
- Start with the reasoning feature and the websocket receive path.
- Confirm the captured text is identical at backend emission, websocket receive, persisted snapshot, and rendered markdown input.

### Technical details
- Target seams:
  - `cmd/web-chat/reasoning_chat_feature.go`
  - `cmd/web-chat/web/src/ws/wsManager.ts` or browser-side websocket inspection
  - `cmd/web-chat/web/src/webchat/cards.tsx`
  - SQLite snapshot / hydration data under the server's `--timeline-db` path

## Step 2: Investigated summary stitching and wrote the bug report

I launched the real `cmd/web-chat` server with a SQLite timeline database and watched one reasoning-heavy turn flow through the entire stack. That let me compare backend logs, websocket frames, and the persisted snapshot side by side. The key result was that the malformed boundary appears before markdown rendering: the browser receives already-glued reasoning text, and the persisted snapshot stores the same string.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** The user wants a detailed bug report that traces where reasoning-summary text is assembled and how it propagates through the app.

**Inferred user intent:** Identify the true stitching boundary, document the evidence, and leave behind a precise report that a developer can act on.

### What I did
- Ran `cmd/web-chat` on port `:8091` with `--timeline-db /tmp/pinocchio-thinking-debug.db --debug-api --log-level debug`.
- Used Playwright to send a prompt and capture websocket frames from the browser.
- Queried the session snapshot endpoint to inspect the stored `ChatMessage` entities.
- Wrote a detailed report to `pinocchio/ttmp/2026/04/22/01-thinking-summary-format-debug/reference/01-bug-report-reasoning-summary-stitching.md`.
- Confirmed the report captures the path from runtime event → `ChatReasoning*` event → SEM/timeline projection → websocket/UI → markdown rendering.

### Why
- The investigation needed a real end-to-end repro to separate upstream stitching problems from frontend rendering behavior.
- A narrative bug report is more useful than a terse note because this issue spans multiple layers and the handoff points matter.

### What worked
- Backend debug logs clearly showed reasoning events with newline-containing completions and summaries.
- Browser websocket frames showed the same malformed boundary already present in transit.
- The persisted session snapshot matched the same text, proving storage was not repairing or introducing the defect.
- The report now names the likely root cause as upstream summary assembly rather than ReactMarkdown or CSS.

### What didn't work
- The attempt to fetch the session snapshot with a simple `curl | python -c 'json.load(...)'` pipeline failed once because the Python process tried to read from the wrong stdin source; the `curl -i` check afterward confirmed the endpoint was returning JSON normally.

### What I learned
- The reasoning summary path in `cmd/web-chat` is mostly a pass-through. It preserves the provider text and republishes it; it does not stitch paragraph breaks back in later.
- The issue is most visible at the moment the model transitions from streamed reasoning into a public-facing summary/heading boundary.

### What was tricky to build
- The tricky part was separating “what the provider meant to say” from “what the app actually received.” The logs proved that by the time the summary reached the browser, the separator was already gone.

### What warrants a second pair of eyes
- The report’s root-cause section is an inference from the evidence we captured, not a direct trace into the model provider implementation.
- If we change the boundary normalization layer later, we should be sure not to accidentally alter legitimate markdown structure.

### What should be done in the future
- Add a focused regression test for a boundary like `category.Creating an analysis plan`.
- Decide whether to repair boundaries in the runtime summary assembly or in the app-owned feature layer before publishing public-facing thoughts.

### Code review instructions
- Start with the report at `pinocchio/ttmp/2026/04/22/01-thinking-summary-format-debug/reference/01-bug-report-reasoning-summary-stitching.md`.
- Cross-check the cited files and confirm the flow matches the logs.
- If implementing a fix, inspect `cmd/web-chat/reasoning_chat_feature.go` before touching the frontend.

### Technical details
- Investigation artifacts:
  - backend logs from the `pinocchio-webchat-debug` tmux session
  - Playwright console frames showing `ChatReasoningAppended` payloads
  - `GET /api/chat/sessions/6f50b1c5-984f-4218-a43e-a66c001f0cd5`
- Key observation: the browser saw the malformed boundary in the websocket payload, so the UI renderer was not the source of the run-on text.

## Step 3: Traced Geppetto provider flow and isolated the likely stitching asymmetry

I then followed the path upstream into Geppetto, using the actual local workspace source selected by `go.work` rather than only the module cache. That mattered because this repo is built against the local `./geppetto` checkout. The main result is that reasoning summaries are currently a special case in the OpenAI Responses engine: Geppetto accumulates summary deltas by simple string concatenation, while normal reasoning text deltas go through a markdown-boundary normalizer first.

### Prompt Context

**User prompt (verbatim):** "Yes, let's study geppetto and the flow from there to web-chat and if can find where the glueing is happening, and if it's only for openai-responses or also other api types"

**Assistant interpretation:** The user wants the upstream emission path identified precisely, including whether the issue is specific to one provider family or a shared abstraction.

**Inferred user intent:** Move from a generic “upstream of the frontend” diagnosis to an exact code-path explanation and a narrower provider-level root-cause hypothesis.

### What I did
- Verified the workspace uses local repos via `go.work`, including `./geppetto`, `./pinocchio`, and `./sessionstream`.
- Confirmed `cmd/web-chat` uses the new sessionstream/chatapp path, not the legacy `pkg/webchat/sem_translator.go` path.
- Inspected `geppetto/pkg/steps/ai/openai_responses/engine.go` for `reasoning-summary`, `EventThinkingPartial`, and summary buffering.
- Inspected `geppetto/pkg/steps/ai/openai/engine_openai.go` for the non-Responses OpenAI path.
- Inspected `geppetto/pkg/steps/ai/streamhelpers/reasoning_markdown.go` to compare normalization behavior.
- Searched all provider engines under `geppetto/pkg/steps/ai` to see which ones emit reasoning events at all.

### Why
- The previous investigation proved the malformed text existed before the frontend. The next useful question was whether Pinocchio receives already-broken text from Geppetto, or whether Geppetto itself is the place where summary chunks are joined too naively.

### What worked
- `go.work` clearly showed that the repo is using the local `./geppetto` checkout, so tracing that source gives the real live behavior.
- The provider search was decisive: only the OpenAI-family engines currently emit `thinking-started`, `thinking-ended`, `EventThinkingPartial`, or `reasoning-summary`.
- The asymmetry in Geppetto is concrete:
  - `openai_responses/engine.go` appends summary deltas via `summaryBuf.WriteString(...)` with no normalization.
  - the same file appends raw reasoning text via `streamhelpers.NormalizeReasoningDelta(...)` before publishing `EventThinkingPartial`.
  - `openai/engine_openai.go` also uses `NormalizeReasoningDelta(...)` for `DeltaReasoning`, but does not emit `reasoning-summary` at all.

### What didn't work
- I did not find a provider-agnostic “summary stitcher” abstraction; the behavior is provider-specific today. That means there is no single shared hook already intended for summary normalization.

### What I learned
- In the current stack, the runtime flow is:
  - Geppetto provider engine emits `events.EventThinkingPartial` and `events.EventInfo("reasoning-summary", ...)`
  - `pinocchio/cmd/web-chat/reasoning_chat_feature.go` republishes those as `ChatReasoning*`
  - `cmd/web-chat/web/src/ws/wsManager.ts` copies `content` straight into the UI entity
- The likely stitching site for the malformed public summary is Geppetto’s OpenAI Responses engine, specifically the summary accumulator `summaryBuf`.
- This is not a generic web-chat/sessionstream bug.
- It is also not currently a Claude/Gemini issue, because those engines are not emitting reasoning-summary events in the first place.

### What was tricky to build
- The subtle part was distinguishing between “OpenAI Chat reasoning text” and “OpenAI Responses reasoning summary.” They are similar in the UI because both flow through `EventThinkingPartial`, but upstream they are assembled differently.

### What warrants a second pair of eyes
- Even though the likely fault line is Geppetto’s `summaryBuf.WriteString(...)`, the provider may already be sending deltas without separators. If so, Geppetto is still the right repair point, but the semantic mistake begins at the provider stream shape.
- Any normalization added for summaries should be narrower than `NormalizeReasoningDelta(...)` unless we intentionally want identical markdown heuristics for public summaries.

### What should be done in the future
- Add a regression test in Geppetto for `openai_responses` summary deltas that end/start at a heading boundary, e.g. sentence end followed by `**Heading**` or a capitalized heading-like line.
- Decide whether summaries should reuse `NormalizeReasoningDelta(...)` or get a summary-specific normalizer.
- Only after that, consider removing the temporary Pinocchio debug logging.

### Code review instructions
- Start in `geppetto/pkg/steps/ai/openai_responses/engine.go` around the `response.reasoning_summary_text.delta` handler and the final `reasoning-summary` info event.
- Compare that to `response.reasoning_text.delta` in the same file and to `pkg/steps/ai/openai/engine_openai.go`.
- Then follow the forwarded event in `pinocchio/cmd/web-chat/reasoning_chat_feature.go` to confirm Pinocchio is just republishing the already-assembled string.

### Technical details
- Key files:
  - `go.work`
  - `geppetto/pkg/steps/ai/openai_responses/engine.go`
  - `geppetto/pkg/steps/ai/openai/engine_openai.go`
  - `geppetto/pkg/steps/ai/streamhelpers/reasoning_markdown.go`
  - `pinocchio/cmd/web-chat/main.go`
  - `pinocchio/cmd/web-chat/reasoning_chat_feature.go`
- Critical code-path fact: `cmd/web-chat` is wired with `appserver.NewServer(..., appserver.WithChatFeatureSets(newAgentModeChatFeature(), newReasoningChatFeature()))`, so the live app path is the sessionstream/chatapp stack, not the legacy SEM translator.

## Step 4: Patched Geppetto summary normalization and validated it end to end

After identifying the asymmetry in Geppetto, I implemented the narrowest fix I could justify: normalize streamed public reasoning-summary deltas the same way we already normalize markdown-looking reasoning text, and add one extra rule for the observed sentence-boundary glue case. This keeps the repair upstream, where the public summary string is first assembled, instead of teaching Pinocchio or the frontend to guess at damaged boundaries later.

I also added a regression test at the provider-engine level and reran the relevant Pinocchio reasoning tests. That combination gives us confidence that the fix changes the upstream summary assembly without altering the web-chat event plumbing.

### Prompt Context

**User prompt (verbatim):** "yes"

**Assistant interpretation:** Implement the Geppetto-side fix and add tests.

**Inferred user intent:** Move from diagnosis to a minimally invasive upstream repair, with evidence that it works in both Geppetto and the consuming web-chat layer.

### What I did
- Added `NormalizeReasoningSummaryDelta(current, delta string)` to `geppetto/pkg/steps/ai/streamhelpers/reasoning_markdown.go`.
- Kept existing markdown-boundary behavior by reusing `NormalizeReasoningDelta(...)` first.
- Added a narrow sentence-boundary rule: if the current summary ends in sentence punctuation and the next chunk begins with an uppercase/digit continuation without leading whitespace, prepend a single space.
- Updated `geppetto/pkg/steps/ai/openai_responses/engine.go` so `response.reasoning_summary_text.delta` writes normalized text into:
  - `summaryBuf`
  - `currentReasoningSummary`
  - `EventThinkingPartial.Completion`
- Added unit tests in `geppetto/pkg/steps/ai/streamhelpers/reasoning_markdown_test.go`.
- Added an integration regression test in `geppetto/pkg/steps/ai/openai_responses/engine_test.go` that checks the final summary becomes `The provider groups work into a category. Creating an analysis plan` instead of a glued boundary.
- Ran:
  - `cd geppetto && gofmt -w pkg/steps/ai/streamhelpers/reasoning_markdown.go pkg/steps/ai/streamhelpers/reasoning_markdown_test.go pkg/steps/ai/openai_responses/engine.go pkg/steps/ai/openai_responses/engine_test.go && go test ./pkg/steps/ai/streamhelpers ./pkg/steps/ai/openai_responses -count=1`
  - `cd pinocchio && go test ./cmd/web-chat -run 'TestReasoningChatFeature' -count=1`

### Why
- The bug source sits upstream in Geppetto’s summary accumulator.
- Fixing it there ensures all downstream consumers receive the repaired text without introducing presentation-layer heuristics.
- A summary-specific normalizer is safer than changing Pinocchio or ReactMarkdown, because those layers no longer need to infer missing separators from already-flattened text.

### What worked
- The Geppetto targeted tests passed:
  - `ok   github.com/go-go-golems/geppetto/pkg/steps/ai/streamhelpers`
  - `ok   github.com/go-go-golems/geppetto/pkg/steps/ai/openai_responses`
- The Pinocchio reasoning feature test also passed:
  - `ok   github.com/go-go-golems/pinocchio/cmd/web-chat`
- The new regression test exercises the exact class of failure we observed: a sentence-ending chunk followed immediately by a capitalized continuation.

### What didn't work
- No implementation failure during this step.
- One design option I chose not to use was broad frontend-side repair; that would have hidden the provider-specific bug instead of fixing the source.

### What I learned
- The right repair point is the Geppetto `summaryBuf` accumulator, not `reasoning_chat_feature.go`.
- The existing `NormalizeReasoningDelta(...)` helper already encoded the right philosophy for markdown-like boundaries; the summary path simply had not been brought under similar protection.
- Only the OpenAI Responses engine currently needed this fix, because it is the only engine emitting public `reasoning-summary` events.

### What was tricky to build
- The main challenge was choosing a heuristic that was narrow enough not to mutate legitimate text. A broad “insert whitespace before capitals” rule would be too risky. I constrained it to sentence punctuation followed by an uppercase/digit continuation with no leading whitespace, and I preserved the existing markdown-block rule separately.

### What warrants a second pair of eyes
- The sentence-boundary heuristic is intentionally conservative, but it is still a heuristic. Reviewers should think about edge cases such as abbreviations, quoted headings, or provider chunks that intentionally start with punctuation.
- If we later see summary boundaries that want paragraph breaks rather than plain spaces, we may need a richer summary-specific normalization rule.

### What should be done in the future
- Re-run the live `cmd/web-chat` server against the same reasoning-heavy prompt and confirm the repaired boundary in:
  - backend logs
  - websocket frames
  - persisted `/api/chat/sessions/:id` snapshot
- Remove or narrow the temporary Pinocchio debug logging after live verification.

### Code review instructions
- Start with `geppetto/pkg/steps/ai/streamhelpers/reasoning_markdown.go` and review `NormalizeReasoningSummaryDelta(...)`.
- Then inspect `geppetto/pkg/steps/ai/openai_responses/engine.go` around `response.reasoning_summary_text.delta`.
- Validate with:
  - `cd geppetto && go test ./pkg/steps/ai/streamhelpers ./pkg/steps/ai/openai_responses -count=1`
  - `cd pinocchio && go test ./cmd/web-chat -run 'TestReasoningChatFeature' -count=1`

### Technical details
- Modified files:
  - `geppetto/pkg/steps/ai/streamhelpers/reasoning_markdown.go`
  - `geppetto/pkg/steps/ai/streamhelpers/reasoning_markdown_test.go`
  - `geppetto/pkg/steps/ai/openai_responses/engine.go`
  - `geppetto/pkg/steps/ai/openai_responses/engine_test.go`
- The fix normalizes the actual stored summary string before it becomes:
  - `EventThinkingPartial.Completion`
  - `metadata.Extra["reasoning_summary_text"]`
  - `EventInfo("reasoning-summary", {text: ...})`

## Step 5: Re-ran the live web-chat app and confirmed clean persisted summary text

After the Geppetto tests passed, I restarted the real `cmd/web-chat` server on `:8091`, sent a fresh reasoning-heavy prompt through the browser, and verified the resulting thinking summary in the live UI and session snapshot. This was the final confidence check that the upstream fix actually changes the user-visible behavior in the canonical app, not just unit tests.

The live result was good: the thinking summary rendered with clean section boundaries like `**Outlining evaluation framework**` and `**Comparing product strategies**`, and the persisted summary text contained no `.Capitalized` glued boundary matches. That means the repaired summary assembly survived the full path from Geppetto to Pinocchio to browser hydration/storage.

### Prompt Context

**User prompt (verbatim):** (same as Step 4)

**Assistant interpretation:** Run the live app and confirm the patch works outside of isolated tests.

**Inferred user intent:** Make sure the actual web-chat experience is fixed, not just the lower-level engine code.

### What I did
- Restarted the server with:
  - `cd pinocchio && go run ./cmd/web-chat web-chat --addr :8091 --timeline-db /tmp/pinocchio-thinking-debug.db --debug-api --log-level debug --profile-registries /home/manuel/.config/pinocchio/profiles.yaml`
- Opened `http://127.0.0.1:8091/` in Playwright.
- Submitted this prompt:
  - `Please reason carefully about how to compare three possible product strategies for a small AI startup. Before answering, think deeply and use multiple internal sections or phases of analysis. Then provide a short final answer with a recommendation.`
- Observed the live session at:
  - `http://127.0.0.1:8091/?sessionId=184bfd9c-b417-4bd8-9f33-62138f7ba31c`
- Queried the persisted snapshot:
  - `GET /api/chat/sessions/184bfd9c-b417-4bd8-9f33-62138f7ba31c`
- Extracted the stored `thinking` message and checked for old glue indicators like `.Creating` and generic `\.[A-Z]` matches.

### Why
- The patch point was upstream, but the original bug report was about a visible web-chat failure. A live rerun closes the loop and confirms the fix lands in the real product surface.

### What worked
- The server came up cleanly on `:8091`.
- The browser connected successfully (`ws: connected`) and streamed a thinking message.
- The live thinking summary contained explicit markdown sections such as:
  - `**Structuring AI strategies**`
  - `**Outlining evaluation framework**`
  - `**Comparing product strategies**`
  - `**Final recommendation for strategy**`
  - `**Providing a structured comparison**`
- The persisted snapshot check reported:
  - `HAS_CATEGORY_CREATING False`
  - `DOT_CAP_MATCHES []`
- The stored text showed clean spacing and paragraph boundaries throughout.

### What didn't work
- My first attempt to background the server with `nohup` did not leave a readable log file, so I reran the server in the foreground once to confirm startup and then relaunched it with a simpler `bash -lc '... nohup ... &'` wrapper.
- I also hit the same shell/pipeline footgun as earlier when trying to pipe `curl` JSON into an inline Python here-doc; switching to `json=$(curl ...)` and `python -c ... "$json"` avoided that problem.

### What I learned
- The Geppetto fix is sufficient for the canonical app path; no additional Pinocchio or frontend normalization was needed.
- The thinking summary now persists as clean markdown-ish text, so hydration and replay should also stay fixed.

### What was tricky to build
- The tricky part was that the live repro is nondeterministic: the provider chooses its own public summary phrasing. Instead of waiting for the exact historical `category.Creating` phrase, I validated the more general invariant that the stored summary no longer contains suspicious sentence-boundary glue patterns like `\.[A-Z]`.

### What warrants a second pair of eyes
- It would still be useful to collect a second live sample with the exact kind of boundary that originally failed, just to show the fix on a near-identical shape rather than on the broader no-glue invariant.
- The current debug logging remains broad and verbose; if it stays in tree any longer, it should probably be narrowed or removed.

### What should be done in the future
- Remove or gate the temporary Pinocchio debug logging now that the diagnosis and live validation are complete.
- Optionally add another Geppetto regression test that targets a markdown heading boundary and a plain sentence-boundary case in the same stream.

### Code review instructions
- Reproduce by restarting `cmd/web-chat` on `:8091`, opening the browser, and sending a reasoning-heavy prompt.
- Confirm the persisted snapshot for the session contains clean section boundaries and no `\.[A-Z]` glue matches in the `thinking` message.
- Start with:
  - `geppetto/pkg/steps/ai/openai_responses/engine.go`
  - `geppetto/pkg/steps/ai/streamhelpers/reasoning_markdown.go`
  - `pinocchio/cmd/web-chat/reasoning_chat_feature.go`

### Technical details
- Live session ID:
  - `184bfd9c-b417-4bd8-9f33-62138f7ba31c`
- Evidence files / endpoints:
  - `/tmp/pinocchio-webchat-debug.log`
  - `http://127.0.0.1:8091/api/chat/sessions/184bfd9c-b417-4bd8-9f33-62138f7ba31c`
- Snapshot extraction result for stored thinking text:
  - `LEN 3560`
  - `HAS_CATEGORY_CREATING False`
  - `DOT_CAP_MATCHES []`

## Step 6: Removed the temporary Pinocchio reasoning debug logging

With the Geppetto fix validated in tests and in the live web-chat flow, the temporary backend logging in `cmd/web-chat/reasoning_chat_feature.go` was no longer needed. I removed the extra debug statements and the helper they depended on so the reasoning feature goes back to being a small pass-through layer instead of a heavily instrumented investigation seam.

This cleanup matters because the debug logs were intentionally verbose: they emitted previews, lengths, and newline flags for every reasoning delta and timeline projection. That was useful during diagnosis, but it would be noisy to keep around after the root cause moved upstream into Geppetto.

### Prompt Context

**User prompt (verbatim):** "remove"

**Assistant interpretation:** Remove the temporary diagnostic logging from the Pinocchio reasoning feature now that the fix has been validated.

**Inferred user intent:** Finish the investigation cleanly and leave the codebase without leftover debugging noise.

### What I did
- Removed the temporary `log.Debug()` calls from:
  - `HandleRuntimeEvent(...)`
  - `ProjectTimeline(...)`
- Removed the `reasoningPreview(...)` helper.
- Removed now-unused imports from `pinocchio/cmd/web-chat/reasoning_chat_feature.go`.
- Ran:
  - `cd pinocchio && gofmt -w cmd/web-chat/reasoning_chat_feature.go && go test ./cmd/web-chat -run 'TestReasoningChatFeature' -count=1`

### Why
- The extra logging had served its purpose during the investigation.
- Keeping it would add noisy debug output to a hot streaming path without ongoing value.

### What worked
- The cleanup was straightforward because the logging had been isolated to one file.
- The focused reasoning feature test still passed:
  - `ok   github.com/go-go-golems/pinocchio/cmd/web-chat`

### What didn't work
- N/A for this step.

### What I learned
- The reasoning feature remains a thin bridge from Geppetto events into sessionstream UI events; it did not need any permanent logic changes for this bug.

### What was tricky to build
- The only subtlety was making sure I removed both the runtime-event logs and the timeline-projection logs together, plus the now-unused helper/imports, so the file returned to a clean baseline rather than a half-instrumented state.

### What warrants a second pair of eyes
- Reviewers may want to confirm there are no remaining temporary logging fragments elsewhere in the web-chat path, especially if additional ad hoc debugging happened outside this file during the broader investigation.

### What should be done in the future
- If the team wants permanent observability for reasoning flows, add it intentionally with narrower, structured counters or trace hooks rather than keeping ad hoc text previews in the hot path.

### Code review instructions
- Inspect `pinocchio/cmd/web-chat/reasoning_chat_feature.go` and confirm only the temporary logging was removed.
- Validate with:
  - `cd pinocchio && go test ./cmd/web-chat -run 'TestReasoningChatFeature' -count=1`

### Technical details
- Cleaned file:
  - `pinocchio/cmd/web-chat/reasoning_chat_feature.go`
- Removed temporary symbols/usages:
  - `reasoningPreview(...)`
  - investigation-only `log.Debug()` calls
