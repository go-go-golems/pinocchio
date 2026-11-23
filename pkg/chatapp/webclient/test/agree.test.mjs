// The client's copy of a session agrees with the server: every captured stream
// (made by capture_test.go against agentserver with the scripted engine),
// replayed into a Timeline, gives the server's final snapshot, and the
// conversation derived from it equals the one derived from the snapshot alone.
// Run by `go test ./pkg/chatapp/webclient`, which sets CHATAPP_CAPTURES.
import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { Timeline } from "../chatapp-client.js";

const dir = process.env.CHATAPP_CAPTURES;

// protojson omits empty values; the client may hold them explicitly.
function norm(v) {
  if (Array.isArray(v)) {
    const a = v.map(norm).filter((x) => x !== undefined);
    return a.length ? a : undefined;
  }
  if (v && typeof v === "object") {
    const o = {};
    for (const k of Object.keys(v).sort()) {
      const x = norm(v[k]);
      if (x !== undefined) o[k] = x;
    }
    return Object.keys(o).length ? o : undefined;
  }
  if (v === "" || v === false || v === 0 || v === "0" || v === null || v === undefined) return undefined;
  return v;
}

const serverEntities = (snap) =>
  Object.fromEntries((snap.entities ?? []).filter((e) => !e.tombstone).map((e) => [e.kind + "/" + e.id, { created: Number(e.createdOrdinal ?? 0), payload: norm(e.payload) }]));
const clientEntities = (tl) => Object.fromEntries(tl.entities().map((e) => [e.kind + "/" + e.id, { created: e.created, payload: norm(e.payload) }]));

function replay(frames) {
  const tl = new Timeline();
  for (const f of frames) {
    if (f.event === "snapshot") tl.reset(f.data);
    else if (f.event === "ui") tl.apply(f.data);
  }
  return tl;
}

const captures = dir ? fs.readdirSync(dir).filter((f) => f.endsWith(".json")).map((f) => JSON.parse(fs.readFileSync(path.join(dir, f), "utf8"))) : [];

test("captures are present", { skip: dir ? false : "CHATAPP_CAPTURES is not set (go test ./pkg/chatapp/webclient runs this)" }, () => {
  assert.ok(captures.length >= 4, `captures in ${dir}`);
});

for (const c of captures) {
  c.streams.forEach((frames, i) => {
    test(`${c.name}, stream ${i + 1}: the copy equals the server's snapshot`, () => {
      assert.equal(frames[0].event, "snapshot", "a stream starts with a snapshot");
      assert.deepEqual(clientEntities(replay(frames)), serverEntities(c.final));
    });
    test(`${c.name}, stream ${i + 1}: the conversation equals the one from the snapshot alone`, () => {
      const fromSnapshot = new Timeline();
      fromSnapshot.reset(c.final);
      assert.deepEqual(replay(frames).answers(), fromSnapshot.answers());
    });
  });
}

const byName = Object.fromEntries(captures.map((c) => [c.name, c]));
const turnsOf = (name) => {
  const tl = new Timeline();
  tl.reset(byName[name].final);
  return tl.answers();
};
const kinds = (a) => a.parts.map((p) => p.kind).join(",");

test("a server tool: thinking, the call, the answer; then a second answer", { skip: !byName["server-tool-two-answers"] }, () => {
  const turns = turnsOf("server-tool-two-answers");
  assert.equal(turns.length, 2);
  const [a, b] = turns.map((t) => t.answer);
  assert.equal(turns[0].question.text, "/think plan the call -- /tool noop {}");
  assert.equal(kinds(a), "thinking,tool,text");
  assert.equal(a.parts[0].text, "plan the call");
  assert.equal(a.parts[1].name, "noop");
  assert.equal(a.parts[1].where, "server");
  assert.equal(a.parts[1].result, "ok");
  assert.equal(a.status, "finished");
  assert.equal(a.model, "fake:scripted");
  assert.equal(a.providerCalls, 2);
  assert.ok(a.usage.input > 0 && a.startedAt > 0 && a.finishedAt >= a.startedAt);
  for (const p of a.parts) assert.ok(p.startedAt >= a.startedAt && p.finishedAt >= p.startedAt, `${p.kind} times`);
  assert.equal(turns[1].question.text, "thanks");
  assert.equal(b.text, "echo: thanks");
  assert.equal(b.providerCalls, 1);
});

test("a browser tool: one part for the call, with where it ran and its result", { skip: !byName["browser-tool-and-late-attach"] }, () => {
  const [t] = turnsOf("browser-tool-and-late-attach");
  assert.equal(kinds(t.answer), "thinking,tool,text");
  const tool = t.answer.parts[1];
  assert.equal(tool.where, "browser");
  assert.deepEqual(tool.input, { code: "6*7" });
  assert.deepEqual(tool.result, { value: 42 });
  assert.equal(tool.status, "success");
  assert.ok(tool.startedAt > 0 && tool.finishedAt >= tool.startedAt);
  assert.match(t.answer.text, /^eval returned: /);
});

test("stopped and failed answers say so", { skip: !byName.stopped || !byName["failed-quota"] }, () => {
  const [stopped] = turnsOf("stopped");
  assert.equal(stopped.answer.status, "stopped");
  assert.ok(stopped.answer.finishedAt >= stopped.answer.startedAt);
  const [failed] = turnsOf("failed-quota");
  assert.equal(failed.answer.status, "failed");
  assert.equal(failed.answer.code, "quota_exceeded");
  assert.ok(failed.answer.error);
});
