# Bug Report: reasoning-summary boundaries lose paragraph/section separation

## Summary

Reasoning-summary text is being delivered to the UI with a missing separator at some chunk boundaries, which makes section transitions appear glued together, e.g. `category.Creating an analysis plan` instead of `category. Creating an analysis plan` or a proper paragraph break.

The failure is not in markdown rendering. The content arrives at the browser already flattened at the boundary, and the frontend simply renders what it receives.

## Impact

- Public-facing reasoning summaries look malformed and harder to read.
- Headings and paragraph boundaries become ambiguous.
- The issue is visible in the “thinking”/reasoning summary path, which is user-facing and highly sensitive to readability.

## Reproduction

1. Start `cmd/web-chat` with a persistent timeline store.
2. Submit a prompt that produces a long reasoning summary with multiple conceptual sections.
3. Observe the browser websocket stream and the persisted timeline snapshot.
4. Look for a boundary where one chunk ends with a word like `category` and the next chunk begins with a capitalized heading-like phrase such as `Creating an analysis plan`.
5. The combined text renders as `category.Creating...` instead of preserving a separator.

## Environment used for investigation

- Server: `go run ./cmd/web-chat web-chat --addr :8091 --timeline-db /tmp/pinocchio-thinking-debug.db --debug-api --log-level debug --profile-registries /home/manuel/.config/pinocchio/profiles.yaml`
- Browser: local Chrome via Playwright against `http://127.0.0.1:8091/`
- Conversation/session observed: `6f50b1c5-984f-4218-a43e-a66c001f0cd5`

## Investigation notes

I traced the reasoning-summary path through four layers:

1. runtime event handling in `cmd/web-chat/reasoning_chat_feature.go`
2. semantic event translation in `pkg/webchat/sem_translator.go`
3. websocket / UI mutation handling in `cmd/web-chat/web/src/ws/wsManager.ts`
4. rendering in `cmd/web-chat/web/src/webchat/cards.tsx` and `Markdown.tsx`

The key finding is that the boundary loss is already present before rendering:

- backend logs show `runtime thinking partial` and `runtime reasoning summary` with `text_has_newline=true`
- websocket payloads show the combined reasoning content with the same missing separator at the boundary
- persisted snapshots store the same text
- the DOM renders the same text, which means the renderer is not the root cause

## Detailed flow

### 1) Runtime event → app-owned reasoning feature

File: `cmd/web-chat/reasoning_chat_feature.go`

Relevant behavior:

- `EventThinkingPartial` is published as `ChatReasoningAppended`
- `EventInfo("thinking-started")` is published as `ChatReasoningStarted`
- `EventInfo("thinking-ended")` is published as `ChatReasoningFinished`
- `EventInfo("reasoning-summary")` is published as `ChatReasoningFinished` with `source=summary`

The feature passes through the incoming text directly:

- `content: ev.Completion` for streaming deltas
- `content: infoText(ev.Data)` for the summary event

There is no normalization layer here that could explain the missing separator.

### 2) SEM translation / timeline projection

File: `pkg/webchat/sem_translator.go`

Relevant behavior:

- `reasoning-summary` becomes `llm.thinking.summary`
- the summary text is encoded into `sempb.LlmFinal{Text: text}`
- no string joining or whitespace repair happens here

File: `pkg/webchat/timeline_projector.go`

Relevant behavior:

- `llm.thinking.summary` is written into the timeline entity `content`
- if the summary has text, it is stored as-is
- the projector keeps the last observed content when the summary text is empty

So the timeline projection preserves the incoming string; it does not stitch paragraphs back together.

### 3) Browser websocket handling

File: `cmd/web-chat/web/src/ws/wsManager.ts`

Relevant behavior:

- `ChatReasoningStarted`, `ChatReasoningAppended`, and `ChatReasoningFinished` are mapped into message entities
- the `content` field is taken directly from `payload.content || payload.text || payload.chunk`
- there is no markdown normalization or whitespace repair

This means whatever boundary exists in the websocket payload is what the UI receives.

### 4) Rendering

Files:

- `cmd/web-chat/web/src/webchat/cards.tsx`
- `cmd/web-chat/web/src/webchat/Markdown.tsx`
- `cmd/web-chat/web/src/webchat/styles/webchat.css`

Relevant behavior:

- `MessageCard` passes `content` directly into `Markdown`
- `ReactMarkdown` renders the text
- CSS adds spacing for paragraphs and headings, but cannot restore a separator that is missing from the source text

Conclusion: rendering is not the root cause.

## Evidence captured

### Backend logs

The reasoning feature logs showed:

- `runtime thinking partial` with `completion_has_newline=true`
- `runtime reasoning summary` with `text_has_newline=true`
- `projecting reasoning timeline entity` with `content_has_newline=true`

This confirms the text contains newline characters in aggregate, but not necessarily the separator that was lost at the boundary of the problematic chunk.

### Websocket payloads

The browser console captured `ChatReasoningAppended` frames whose payload `content` contained the already-glued text. The frame was not corrected in transit.

### Persisted snapshot

`GET /api/chat/sessions/:sessionId` showed the final thinking entity with the same flattened text, including the boundary loss.

## Likely root cause

The actual stitching is happening upstream of this repository’s UI layers: the model/provider or summary synthesizer is emitting a chunk boundary without a preserved separator. The app code then faithfully passes that string through the following pipeline:

`runtime event` → `ChatReasoning*` event → `SEM` event → `timeline snapshot / websocket` → `UI message entity` → `Markdown`

If the upstream source drops a space/newline between a completed sentence and a following heading, the app currently has no repair step.

## Why the bug is visible specifically at summary boundaries

The bug appears most often where the provider transitions from free-form reasoning text into a public-facing summary section. That is the point where the text changes shape:

- a sentence ends
- a new heading begins
- the boundary may need a blank line or at least one space

When the boundary is missing, the output looks like a run-on sentence or a malformed markdown paragraph.

## Recommended next steps

1. Confirm which upstream component emits the final `reasoning-summary` text.
2. Add a narrow normalization step at the boundary that produces public-facing thinking summaries, if the upstream source cannot guarantee paragraph separation.
3. Add a regression test that feeds a summary with a boundary like `category` + `Creating an analysis plan` and verifies the rendered content preserves separation.
4. Keep the fix close to the event assembly layer, not the markdown renderer.

## Files reviewed

- `/home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio/cmd/web-chat/reasoning_chat_feature.go`
- `/home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio/pkg/webchat/sem_translator.go`
- `/home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio/pkg/webchat/timeline_projector.go`
- `/home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
- `/home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`
- `/home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio/cmd/web-chat/web/src/webchat/Markdown.tsx`
- `/home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio/cmd/web-chat/web/src/webchat/styles/webchat.css`

## Open question

Is the upstream reasoning-summary generator allowed to emit incomplete punctuation/whitespace at chunk boundaries, or should the app repair that before publishing the public-facing summary?
