// src/timeline.ts
var TERMINAL = /* @__PURE__ */ new Set(["finished", "failed", "stopped"]);
var num = (v) => typeof v === "number" ? v : typeof v === "string" && v !== "" ? Number(v) : 0;
var str = (v) => typeof v === "string" ? v : "";
var first = (...vs) => {
  for (const v of vs) if (typeof v === "string" && v !== "") return v;
  return "";
};
var TEXT_SEP = ":text:";
var parentOf = (segmentId) => {
  const i = segmentId.lastIndexOf(TEXT_SEP);
  return i > 0 ? segmentId.slice(0, i) : "";
};
var APPEND = /* @__PURE__ */ new Set(["CHAT_STREAM_PATCH_MODE_APPEND", "CHAT_STREAM_PATCH_MODE_UNSPECIFIED", "", void 0]);
var applyPatch = (current, patch, mode) => APPEND.has(mode) ? current + patch : patch;
var mergeCorrelation = (a, b) => {
  if (!b) return a;
  const out = { ...a || {} };
  for (const [k, v] of Object.entries(b)) if (v !== "" && v !== void 0 && v !== null) out[k] = v;
  return out;
};
var addUsage = (a, b) => {
  const out = {};
  for (const k of ["inputTokens", "outputTokens", "cachedTokens", "cacheCreationInputTokens", "cacheReadInputTokens"]) {
    const v = num(a?.[k]) + num(b?.[k]);
    if (v) out[k] = v;
  }
  return out;
};
var Timeline = class {
  #entities = /* @__PURE__ */ new Map();
  #ordinal = 0;
  // the ordinal the copy is current at
  #fallback = 0;
  // creation order for events without an ordinal
  /** Increases with every change. */
  version = 0;
  /** The ordinal of the last snapshot or event applied. */
  get ordinal() {
    return this.#ordinal;
  }
  /** A snapshot replaces the copy. */
  reset(snap) {
    this.#entities.clear();
    for (const e of snap.entities ?? []) {
      if (!e.tombstone) this.#entities.set(e.kind + "/" + e.id, { kind: e.kind, id: e.id, created: num(e.createdOrdinal), payload: e.payload ?? {} });
    }
    this.#ordinal = num(snap.snapshotOrdinal);
    this.version++;
  }
  /** Applies one UI event; true when the copy changed. An event the copy already reflects is ignored. */
  apply(ev) {
    const ord = num(ev.ordinal);
    if (ord && ord <= this.#ordinal) return false;
    const changed = this.#reduce(ev.name, ev.payload ?? {}, ord);
    if (ord) this.#ordinal = ord;
    if (changed) this.version++;
    return changed;
  }
  entity(kind, id) {
    return this.#entities.get(kind + "/" + id);
  }
  /** Entities (of one kind), in creation order. */
  entities(kind) {
    const out = [...this.#entities.values()].filter((e) => !kind || e.kind === kind);
    return out.sort((a, b) => a.created - b.created);
  }
  /** Whether a run of the session is running. */
  get running() {
    return this.entities("ChatRun").some((e) => e.payload.status === "running");
  }
  #get(kind, id) {
    return this.#entities.get(kind + "/" + id)?.payload;
  }
  #put(kind, id, payload, ord) {
    const key = kind + "/" + id;
    const cur = this.#entities.get(key);
    this.#entities.set(key, { kind, id, created: cur ? cur.created : ord || ++this.#fallback, payload });
    return true;
  }
  #run(id, ord, fn) {
    if (!id) return false;
    const r = { ...this.#get("ChatRun", id) ?? {}, messageId: id };
    fn(r);
    return this.#put("ChatRun", id, r, ord);
  }
  #reduce(name, p, ord) {
    const id = str(p.messageId).trim();
    switch (name) {
      case "ChatUserMessageAccepted": {
        if (!id) return false;
        const content = first(p.content, p.text);
        return this.#put("ChatMessage", id, {
          messageId: id,
          role: first(p.role, "user"),
          prompt: p.prompt,
          content,
          text: content,
          status: first(p.status, "accepted"),
          attachments: p.attachments,
          startedAtMs: p.atMs,
          finishedAtMs: p.atMs
        }, ord);
      }
      case "ChatRunStarted":
        return this.#run(id, ord, (r) => Object.assign(r, { status: "running", prompt: p.prompt, model: p.model, startedAtMs: p.startedAtMs }));
      case "ChatRunFinished":
        return this.#run(id, ord, (r) => Object.assign(r, { status: "finished", truncated: p.truncated, reason: p.reason, finishedAtMs: p.finishedAtMs }));
      case "ChatRunStopped":
        return this.#run(id, ord, (r) => Object.assign(r, { status: "stopped", finishedAtMs: p.finishedAtMs }));
      case "ChatRunFailed": {
        if (!id) return false;
        this.#run(id, ord, (r) => Object.assign(r, { status: "failed", code: p.code, error: p.error, finishedAtMs: p.finishedAtMs }));
        const content = first(p.error, "Chat run failed");
        return this.#put("ChatMessage", id, {
          messageId: id,
          role: "error",
          content,
          text: content,
          status: first(p.status, "failed"),
          final: true,
          correlation: p.correlation,
          startedAtMs: p.finishedAtMs,
          finishedAtMs: p.finishedAtMs
        }, ord);
      }
      case "ChatProviderCallFinished":
        return this.#run(id, ord, (r) => {
          r.usage = addUsage(r.usage, p.usage);
          r.providerCalls = num(r.providerCalls) + 1;
        });
      case "ChatTextSegmentStarted":
      case "ChatTextPatch":
      case "ChatTextSegmentFinished":
        return this.#text(name, id, p, ord);
      case "ChatReasoningSegmentStarted":
      case "ChatReasoningPatch":
      case "ChatReasoningSegmentFinished":
        return this.#thinking(name, id, p, ord);
      case "ChatToolCallStarted":
      case "ChatToolArgumentsPatch":
      case "ChatToolCallRequested":
      case "ChatToolExecutionStarted":
      case "ChatToolCallFinished":
        return this.#toolCall(name, p, ord);
      case "ChatToolResultReady": {
        const call = str(p.toolCallId);
        if (!call) return false;
        return this.#put("ChatToolResult", call + ":result", {
          messageId: p.messageId,
          toolCallId: call,
          toolName: p.toolName,
          result: p.result,
          status: p.status,
          correlation: p.correlation
        }, ord);
      }
      case "ChatToolCallProgress": {
        const call = str(p.toolCallId);
        if (!call) return false;
        const cur = { ...this.#get("ChatToolCall", call) ?? {}, toolCallId: call };
        if (!cur.messageId) cur.messageId = p.messageId;
        cur.state = p.state;
        return this.#put("ChatToolCall", call, cur, ord);
      }
      case "ChatFrontendToolCallRequested": {
        const call = str(p.toolCallId);
        if (!call) return false;
        return this.#put("ChatFrontendToolCall", call, {
          toolCallId: call,
          toolName: p.toolName,
          parentMessageId: p.messageId,
          mode: p.mode,
          status: first(p.status, "requested"),
          input: p.input,
          executor: p.executor,
          requestedAtMs: p.atMs
        }, ord);
      }
      case "ChatFrontendToolResultReceived": {
        const call = str(p.toolCallId);
        if (!call) return false;
        const cur = { ...this.#get("ChatFrontendToolCall", call) ?? {}, toolCallId: call };
        if (p.toolName) cur.toolName = p.toolName;
        if (p.messageId) cur.parentMessageId = p.messageId;
        Object.assign(cur, { result: p.result, status: first(p.status, "success"), error: p.error, executor: p.executor, finishedAtMs: p.atMs });
        return this.#put("ChatFrontendToolCall", call, cur, ord);
      }
      default:
        return false;
    }
  }
  // projections.go: ChatTextSegmentStarted / ChatTextPatch / ChatTextSegmentFinished
  #text(name, id, p, ord) {
    if (!id) return false;
    const had = this.#get("ChatMessage", id);
    const e = { ...had ?? {} };
    const current = first(e.content, e.text);
    let content;
    if (name === "ChatTextPatch") {
      content = applyPatch(current, str(p.text), p.mode);
      if (content === "") return false;
    } else if (name === "ChatTextSegmentFinished") {
      content = first(p.content, p.text, current);
      if (content === "" && !had) return false;
    } else {
      content = current;
      if (content === "" && !had) return false;
    }
    e.messageId = id;
    e.role = first(p.role, "assistant");
    e.prompt = first(p.prompt, e.prompt);
    e.content = content;
    e.text = content;
    if (name === "ChatTextPatch") {
      e.status = first(p.status, "streaming");
      e.streaming = !p.final;
      e.final = p.final;
    } else if (name === "ChatTextSegmentFinished") {
      e.status = first(p.status, "finished");
      e.streaming = p.streaming;
      e.final = p.final;
    } else {
      e.status = first(p.status, "streaming");
      e.streaming = p.streaming;
    }
    if (!num(e.startedAtMs) && num(p.atMs)) e.startedAtMs = p.atMs;
    if (name === "ChatTextSegmentFinished") e.finishedAtMs = p.atMs;
    e.parentMessageId = parentOf(id);
    e.correlation = mergeCorrelation(e.correlation, p.correlation);
    return this.#put("ChatMessage", id, e, ord);
  }
  // plugins/reasoning.go: reasoningEntityFromFields
  #thinking(name, id, p, ord) {
    if (!id) return false;
    const had = this.#get("ChatMessage", id);
    const e = { ...had ?? {} };
    let content;
    if (name === "ChatReasoningPatch") content = applyPatch(first(e.content, e.text), str(p.text), p.mode);
    else if (name === "ChatReasoningSegmentFinished") content = first(p.content, p.text);
    else content = "";
    if (content === "") content = first(e.content, e.text);
    if (content === "" && !had) return false;
    e.messageId = id;
    e.role = first(p.role, "thinking");
    e.content = content;
    e.text = content;
    e.parentMessageId = first(p.parentMessageId, e.parentMessageId);
    e.correlation = mergeCorrelation(e.correlation, p.correlation);
    e.segmentType = "reasoning";
    if (!num(e.startedAtMs) && num(p.atMs)) e.startedAtMs = p.atMs;
    if (name === "ChatReasoningSegmentFinished") {
      e.status = first(p.status, "finished");
      e.streaming = false;
      e.finishedAtMs = p.atMs;
    } else {
      e.status = first(p.status, "streaming");
      e.streaming = name === "ChatReasoningPatch" ? !p.final : p.streaming;
    }
    return this.#put("ChatMessage", id, e, ord);
  }
  // plugins/toolcall.go: toolCallFieldsFromCanonical + mergeToolCallFields
  #toolCall(name, p, ord) {
    const call = str(p.toolCallId);
    const status = first(
      p.status,
      { ChatToolCallStarted: "pending", ChatToolArgumentsPatch: "streaming_args", ChatToolCallRequested: "pending", ChatToolExecutionStarted: "executing", ChatToolCallFinished: "completed" }[name]
    );
    const input = name === "ChatToolArgumentsPatch" ? str(p.arguments) : name === "ChatToolCallFinished" ? "" : str(p.input);
    const e = { ...this.#get("ChatToolCall", call) ?? {} };
    if (p.messageId) e.messageId = p.messageId;
    if (call) e.toolCallId = call;
    if (p.toolName) e.toolName = p.toolName;
    if (input) e.input = status === "streaming_args" ? str(e.input) + input : input;
    e.executing = name === "ChatToolCallFinished" || name === "ChatToolArgumentsPatch" ? false : !!p.executing;
    if (status) e.status = status;
    e.correlation = mergeCorrelation(e.correlation, p.correlation);
    if (name === "ChatToolExecutionStarted") {
      if (!num(e.startedAtMs)) e.startedAtMs = p.atMs;
      e.state = "running";
    } else if (name === "ChatToolCallFinished") {
      e.status = "completed";
      e.executing = false;
      e.finishedAtMs = p.atMs;
      e.state = "";
    }
    return this.#put("ChatToolCall", call, e, ord);
  }
  /** The conversation: each question with its answer, in order. */
  answers() {
    const turns = /* @__PURE__ */ new Map();
    const turnOf = (run, order) => {
      let t = turns.get(run);
      if (!t) turns.set(run, t = { turn: { question: null, answer: null }, order });
      t.order = Math.min(t.order, order);
      return t.turn;
    };
    const answers = /* @__PURE__ */ new Map();
    for (const e of this.entities("ChatRun")) {
      const a = toAnswer(e.payload, e.id);
      answers.set(e.id, a);
      turnOf(e.id, e.created).answer = a;
    }
    const tools = /* @__PURE__ */ new Map();
    const parts = [];
    for (const e of this.entities()) {
      const p = e.payload;
      if (e.kind === "ChatMessage") {
        if (p.role === "user") {
          const run = e.id.endsWith("-user") ? e.id.slice(0, -5) : e.id;
          turnOf(run, e.created).question = { id: e.id, text: first(p.text, p.content), at: num(p.startedAtMs) };
        } else if (p.role === "assistant" || p.role === "thinking") {
          parts.push({
            run: str(p.parentMessageId),
            created: e.created,
            part: { kind: p.role === "thinking" ? "thinking" : "text", id: e.id, text: first(p.text, p.content), streaming: !!p.streaming, startedAt: num(p.startedAtMs), finishedAt: num(p.finishedAtMs) }
          });
        }
      } else if (e.kind === "ChatToolCall" || e.kind === "ChatToolResult" || e.kind === "ChatFrontendToolCall") {
        const call = str(p.toolCallId) || e.id;
        let t = tools.get(call);
        if (!t) {
          t = { part: { kind: "tool", id: call, name: "", input: void 0, result: void 0, error: "", status: "", state: "", where: "server", mode: "", executor: null, startedAt: 0, finishedAt: 0 }, created: e.created };
          tools.set(call, t);
          parts.push({ run: str(e.kind === "ChatFrontendToolCall" ? p.parentMessageId : p.messageId), created: e.created, part: t.part });
        }
        mergeTool(t.part, e.kind, p);
        const run = str(e.kind === "ChatFrontendToolCall" ? p.parentMessageId : p.messageId);
        const holder = parts.find((x) => x.part === t.part);
        if (holder && !holder.run && run) holder.run = run;
      }
    }
    parts.sort((a, b) => a.created - b.created);
    for (const { run, part } of parts) {
      const a = answers.get(run);
      if (a) a.parts.push(part);
    }
    for (const a of answers.values()) a.text = a.parts.filter((x) => x.kind === "text").map((x) => x.text).join("\n\n");
    return [...turns.values()].sort((a, b) => a.order - b.order).map((t) => t.turn);
  }
  /** One answer by its run's message id. */
  answer(id) {
    for (const t of this.answers()) if (t.answer?.id === id) return t.answer;
    return null;
  }
};
function toAnswer(p, id) {
  const u = p.usage ?? {};
  return {
    id,
    status: first(p.status, "running"),
    code: str(p.code),
    error: str(p.error),
    truncated: !!p.truncated,
    reason: str(p.reason),
    model: str(p.model),
    startedAt: num(p.startedAtMs),
    finishedAt: num(p.finishedAtMs),
    usage: { input: num(u.inputTokens), output: num(u.outputTokens), cachedInput: num(u.cachedTokens) + num(u.cacheReadInputTokens) },
    providerCalls: num(p.providerCalls),
    parts: [],
    text: ""
  };
}
var parseMaybe = (v) => {
  if (typeof v !== "string" || v === "") return v === "" ? void 0 : v;
  try {
    return JSON.parse(v);
  } catch {
    return v;
  }
};
function mergeTool(t, kind, p) {
  t.name = first(p.toolName, t.name);
  if (kind === "ChatFrontendToolCall") {
    t.where = "browser";
    if (p.input !== void 0) t.input = p.input;
    if (p.result !== void 0) t.result = p.result;
    t.error = first(p.error, t.error);
    t.status = first(p.status, t.status);
    t.mode = first(p.mode, t.mode);
    t.executor = p.executor ?? t.executor;
    t.startedAt = num(p.requestedAtMs) || t.startedAt;
    t.finishedAt = num(p.finishedAtMs) || t.finishedAt;
  } else if (kind === "ChatToolCall") {
    if (t.input === void 0 && p.input !== void 0) t.input = parseMaybe(p.input);
    if (t.where === "server") t.status = first(p.status, t.status);
    t.state = str(p.state);
    t.startedAt = t.startedAt || num(p.startedAtMs);
    t.finishedAt = t.finishedAt || num(p.finishedAtMs);
  } else if (kind === "ChatToolResult") {
    if (t.result === void 0) t.result = parseMaybe(p.result);
    if (p.status === "error" && !t.error) t.error = typeof t.result === "string" ? t.result : "error";
  }
}

// src/session.ts
var ChatError = class extends Error {
  code;
  status;
  constructor(code, message, status = 0) {
    super(message);
    this.code = code;
    this.status = status;
  }
};
var clean = (o) => Object.fromEntries(Object.entries(o).filter(([, v]) => v !== void 0 && v !== null && v !== ""));
var rid = (p) => p + Math.random().toString(36).slice(2, 10);
var sleep = (ms, signal) => new Promise((resolve) => {
  const t = setTimeout(resolve, ms);
  signal?.addEventListener("abort", () => (clearTimeout(t), resolve()), { once: true });
});
async function request(base, method, path, body, o) {
  const f = o.fetch ?? globalThis.fetch;
  const r = await f(base + path, {
    method,
    headers: { ...body === void 0 ? {} : { "Content-Type": "application/json" }, ...o.headers ?? {} },
    body: body === void 0 ? void 0 : JSON.stringify(body)
  });
  const txt = await r.text();
  let data = null;
  try {
    data = txt ? JSON.parse(txt) : null;
  } catch {
    data = null;
  }
  if (!r.ok) {
    const e = data?.error;
    const code = (typeof e === "object" && e ? e.code : data?.code) || "http_" + r.status;
    throw new ChatError(code, (typeof e === "object" && e ? e.message : e) || "HTTP " + r.status, r.status);
  }
  return data;
}
async function readSSE(body, fn) {
  const reader = body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  for (; ; ) {
    const x = await reader.read();
    if (x.done) return;
    buf += dec.decode(x.value, { stream: true }).replace(/\r\n/g, "\n");
    let i;
    while ((i = buf.indexOf("\n\n")) >= 0) {
      const block = buf.slice(0, i);
      buf = buf.slice(i + 2);
      let event = "message";
      const data = [];
      for (const line of block.split("\n")) {
        if (line.startsWith("event:")) event = line.slice(6).trim();
        else if (line.startsWith("data:")) data.push(line.slice(5).replace(/^ /, ""));
      }
      if (data.length) await fn(event, data.join("\n"));
    }
  }
}
var Session = class _Session {
  id;
  base;
  timeline = new Timeline();
  info;
  #o;
  #listeners = /* @__PURE__ */ new Set();
  #waits = /* @__PURE__ */ new Set();
  #stream = new AbortController();
  #closed = false;
  #ready;
  #error = null;
  // browser tools
  #tools = [];
  #client = rid("page-");
  #connection = rid("c-");
  #revision = 0;
  #executor = null;
  #ran = /* @__PURE__ */ new Set();
  #outbox = /* @__PURE__ */ new Map();
  #calls = /* @__PURE__ */ new Map();
  constructor(base, id, info, o) {
    this.base = base.replace(/\/$/, "");
    this.id = id;
    this.info = info;
    this.#o = o;
  }
  /** Creates a session ({labels, data, id?}) and opens it. */
  static async create(base, req = {}, o = {}) {
    const out = await request(base.replace(/\/$/, ""), "POST", "/sessions", clean(req), o);
    return _Session.#start(base, out.sessionId, out.session ?? { id: out.sessionId }, o);
  }
  /** Opens an existing session: resolves after its first snapshot. */
  static async open(base, id, o = {}) {
    const out = await request(base.replace(/\/$/, ""), "GET", "/sessions/" + encodeURIComponent(id), void 0, o);
    return _Session.#start(base, id, out.session ?? { id }, o);
  }
  /** Connects to a session the caller already has (its info from create or list): resolves after the first snapshot. */
  static async attach(base, info, o = {}) {
    if (!info?.id) throw new ChatError("invalid_request", "attach needs the session's info with its id");
    return _Session.#start(base, info.id, info, o);
  }
  /** Lists sessions that have all these labels. */
  static async list(base, labels = {}, o = {}) {
    const q = Object.entries(labels).map(([k, v]) => "label=" + encodeURIComponent(k + ":" + v)).join("&");
    const out = await request(base.replace(/\/$/, ""), "GET", "/sessions" + (q ? "?" + q : ""), void 0, o);
    return out?.sessions ?? [];
  }
  static async #start(base, id, info, o) {
    const s = new _Session(base, id, info, o);
    let ready;
    s.#ready = new Promise((r) => ready = r);
    void s.#connect(ready);
    await s.#ready;
    if (s.#error) throw s.#error;
    return s;
  }
  /** Calls fn after every change (ev: the UI event; null: a snapshot). Returns an unsubscribe function. */
  on(fn) {
    this.#listeners.add(fn);
    return () => this.#listeners.delete(fn);
  }
  /** Sends a message; resolves with the run's message id (the answer's id). */
  async send(prompt, extra = {}) {
    const out = await this.#api("POST", "/messages", { ...extra, prompt });
    const id = out?.messageId;
    if (!id) throw new ChatError("invalid_response", "the server accepted the message without naming its run");
    return id;
  }
  /** Resolves with the answer once it is finished, failed or stopped. */
  wait(id, signal) {
    return new Promise((resolve, reject) => {
      const check = () => {
        if (signal?.aborted) return done(() => reject(signal.reason ?? new ChatError("aborted", "aborted")));
        if (this.#closed) return done(() => reject(this.#error ?? new ChatError("closed", "the session was closed")));
        const a = this.timeline.answer(id);
        if (a && TERMINAL.has(a.status)) done(() => resolve(a));
      };
      const done = (fn) => {
        this.#waits.delete(check);
        fn();
      };
      this.#waits.add(check);
      signal?.addEventListener("abort", check, { once: true });
      check();
    });
  }
  async stop() {
    await this.#api("POST", "/stop");
  }
  async delete() {
    await this.#api("DELETE", "");
    this.close();
  }
  /** Stops the stream and the page's running tool calls. */
  close() {
    if (this.#closed) return;
    this.#closed = true;
    this.#stream.abort();
    for (const c of this.#calls.values()) c.abort();
    this.#notify(null);
  }
  /** Registers the page's browser tools (replacing earlier ones) and runs the calls the server assigns to this page. */
  async tools(list) {
    this.#tools = list;
    await this.#register();
    this.#derive();
  }
  async #api(method, path, body) {
    return request(this.base, method, "/sessions/" + encodeURIComponent(this.id) + path, body, this.#o);
  }
  async #register() {
    const out = await this.#api("POST", "/tools/manifest", {
      clientInstanceId: this.#client,
      connectionId: this.#connection,
      revision: ++this.#revision,
      tools: this.#tools.map((t) => clean({ name: t.name, description: t.description, inputSchema: t.inputSchema ?? { type: "object" }, mode: t.mode ?? "frontend", available: true }))
    });
    this.#executor = out?.executor ?? null;
  }
  async #connect(ready) {
    const f = this.#o.fetch ?? globalThis.fetch;
    for (let attempt = 0; !this.#closed; attempt++) {
      try {
        const r = await f(this.base + "/sessions/" + encodeURIComponent(this.id) + "/events", { headers: this.#o.headers ?? {}, signal: this.#stream.signal });
        if (!r.ok || !r.body) {
          if (r.status >= 400 && r.status < 500) {
            this.#error = new ChatError(r.status === 404 ? "session_not_found" : "http_" + r.status, "the session's events are not available", r.status);
            ready();
            this.close();
            return;
          }
          throw new Error("HTTP " + r.status);
        }
        await readSSE(r.body, async (event, data) => {
          if (event === "snapshot") {
            attempt = 0;
            this.timeline.reset(JSON.parse(data));
            if (this.#tools.length) await this.#register().catch(() => {
            });
            for (const o of this.#outbox.values()) o.next = 0;
            ready();
            this.#notify(null);
          } else if (event === "ui") {
            const ev = JSON.parse(data);
            if (this.timeline.apply(ev)) this.#notify(ev);
          }
        });
      } catch {
        if (this.#closed) return;
      }
      await sleep(Math.min(5e3, 250 * 2 ** attempt), this.#stream.signal);
    }
  }
  #notify(ev) {
    this.#derive();
    for (const fn of [...this.#listeners]) {
      try {
        fn(ev);
      } catch (e) {
        console.error(e);
      }
    }
    for (const check of [...this.#waits]) check();
  }
  // The browser tool calls this page must run or deliver, from the timeline.
  #derive() {
    if (!this.#tools.length) return;
    for (const e of this.timeline.entities("ChatFrontendToolCall")) {
      const p = e.payload;
      if (p.status === "requested") {
        if (this.#outbox.has(e.id)) this.#deliver(e.id);
        else void this.#call(e.id, p);
      } else {
        this.#outbox.delete(e.id);
        this.#calls.get(e.id)?.abort();
      }
    }
  }
  async #call(id, p) {
    if (this.#ran.has(id) || !this.#executor || p.executor?.assignmentId !== this.#executor.assignmentId) return;
    this.#ran.add(id);
    const tool = this.#tools.find((t) => t.name === p.toolName);
    const ctrl = new AbortController();
    this.#calls.set(id, ctrl);
    let status = "success";
    let result = null;
    let error = "";
    try {
      if (!tool) throw new Error("this page has no tool " + p.toolName);
      result = await tool.execute(p.input ?? {}, { signal: ctrl.signal, callId: id });
      if (result === void 0) result = null;
      if (JSON.stringify(result) === void 0) result = String(result);
    } catch (e) {
      status = "failed";
      error = String(e?.message ?? e);
    } finally {
      this.#calls.delete(id);
    }
    if (ctrl.signal.aborted || this.#closed) return;
    this.#outbox.set(id, { toolName: p.toolName, status, result, error, sending: false, tries: 0, next: 0 });
    this.#deliver(id);
  }
  // One delivery attempt; a network or server failure keeps the result for a later attempt.
  #deliver(id) {
    const r = this.#outbox.get(id);
    if (!r || r.sending || Date.now() < r.next) return;
    r.sending = true;
    this.#api("POST", "/tools/results", clean({ toolCallId: id, toolName: r.toolName, status: r.status, result: r.result, error: r.error, executor: this.#executor })).then(() => this.#outbox.delete(id)).catch((e) => {
      if (e instanceof ChatError && e.status >= 400 && e.status < 500) this.#outbox.delete(id);
      else {
        r.tries++;
        const wait = Math.min(5e3, 200 * 2 ** r.tries);
        r.next = Date.now() + wait;
        setTimeout(() => !this.#closed && this.#derive(), wait);
      }
    }).finally(() => {
      r.sending = false;
    });
  }
};
export {
  ChatError,
  Session,
  TERMINAL,
  Timeline,
  parentOf,
  readSSE
};
