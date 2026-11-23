// Timeline without captures: the rules a page relies on.
import { test } from "node:test";
import assert from "node:assert/strict";
import { Timeline } from "../chatapp-client.js";

test("an event the snapshot already reflects is ignored", () => {
  const tl = new Timeline();
  tl.reset({ snapshotOrdinal: "5", entities: [{ kind: "ChatRun", id: "r", createdOrdinal: "3", payload: { messageId: "r", status: "running" } }] });
  assert.equal(tl.apply({ ordinal: "5", name: "ChatRunFinished", payload: { messageId: "r", status: "finished" } }), false);
  assert.equal(tl.running, true);
  assert.equal(tl.apply({ ordinal: "6", name: "ChatRunFinished", payload: { messageId: "r", status: "finished" } }), true);
  assert.equal(tl.running, false);
});

test("text patches append; an empty segment start makes no entity", () => {
  const tl = new Timeline();
  tl.reset({ snapshotOrdinal: "1", entities: [] });
  const seg = "r:text:s1";
  assert.equal(tl.apply({ ordinal: "2", name: "ChatTextSegmentStarted", payload: { messageId: seg, role: "assistant", streaming: true } }), false);
  tl.apply({ ordinal: "3", name: "ChatTextPatch", payload: { messageId: seg, text: "Hello ", mode: "CHAT_STREAM_PATCH_MODE_APPEND", atMs: "100" } });
  tl.apply({ ordinal: "4", name: "ChatTextPatch", payload: { messageId: seg, text: "world", mode: "CHAT_STREAM_PATCH_MODE_APPEND", atMs: "120" } });
  const e = tl.entity("ChatMessage", seg);
  assert.equal(e.payload.text, "Hello world");
  assert.equal(e.payload.parentMessageId, "r");
  assert.equal(e.payload.startedAtMs, "100", "the first patch's time is the start");
  assert.equal(e.created, 3);
});

test("usage is summed per run", () => {
  const tl = new Timeline();
  tl.reset({ snapshotOrdinal: "1" });
  tl.apply({ ordinal: "2", name: "ChatRunStarted", payload: { messageId: "r", model: "m", startedAtMs: "10" } });
  tl.apply({ ordinal: "3", name: "ChatProviderCallFinished", payload: { messageId: "r", usage: { inputTokens: 10, outputTokens: 2 } } });
  tl.apply({ ordinal: "4", name: "ChatProviderCallFinished", payload: { messageId: "r", usage: { inputTokens: 15, outputTokens: 3 } } });
  tl.apply({ ordinal: "5", name: "ChatProviderCallFinished", payload: { usage: { inputTokens: 99 } } }); // no run: ignored
  const a = tl.answer("r");
  assert.deepEqual(a.usage, { input: 25, output: 5, cachedInput: 0 });
  assert.equal(a.providerCalls, 2);
  assert.equal(a.model, "m");
});
