// Timeline is a client's copy of one session's timeline entities: a snapshot
// replaces it, each UI event changes it the way chatapp's projections change
// the server's entities (pkg/chatapp/projections.go, plugins/reasoning.go,
// plugins/toolcall.go, frontendtools/plugin.go), and answers() derives the
// conversation from it. It does no I/O, so it is tested by replaying captured
// streams against the server's own snapshot (test/agree.test.mjs).
//
// Payloads keep the server's JSON shapes (protojson: camelCase, int64 as
// strings); answers() turns them into plain numbers and strings.

export type Payload = Record<string, any>;
export type Entity = { kind: string; id: string; created: number; payload: Payload };
export type UIEvent = { sessionId?: string; ordinal?: string | number; name: string; payload?: Payload };
export type SnapshotEntity = { kind: string; id: string; createdOrdinal?: string | number; tombstone?: boolean; payload?: Payload };
export type Snapshot = { sessionId?: string; snapshotOrdinal?: string | number; status?: string; entities?: SnapshotEntity[] };

export type Usage = { input: number; output: number; cachedInput: number };
export type Question = { id: string; text: string; at: number };
export type TextPart = {
  kind: "thinking" | "text";
  id: string;
  text: string;
  streaming: boolean;
  startedAt: number;
  finishedAt: number;
};
export type ToolPart = {
  kind: "tool";
  id: string; // the tool call id
  name: string;
  input: unknown;
  result: unknown; // the JSON value when the result parses as JSON, else the text
  error: string;
  status: string;
  state: string; // "running", "awaiting-person" or "" (server tools)
  where: "server" | "browser";
  mode: string; // browser tools: TOOL_EXECUTION_MODE_FRONTEND_AUTO | _HUMAN
  executor: Payload | null; // browser tools: the page that runs the call
  startedAt: number;
  finishedAt: number;
};
export type Part = TextPart | ToolPart;
export type Answer = {
  id: string; // the run's message id
  status: "running" | "finished" | "failed" | "stopped";
  code: string;
  error: string;
  truncated: boolean;
  reason: string;
  model: string;
  startedAt: number;
  finishedAt: number; // 0 while running
  usage: Usage;
  providerCalls: number;
  parts: Part[]; // in the order they were created
  text: string; // the text parts, joined
};
export type Turn = { question: Question | null; answer: Answer | null };

export const TERMINAL = new Set(["finished", "failed", "stopped"]);

const num = (v: unknown): number => (typeof v === "number" ? v : typeof v === "string" && v !== "" ? Number(v) : 0);
const str = (v: unknown): string => (typeof v === "string" ? v : "");
const first = (...vs: unknown[]): string => {
  for (const v of vs) if (typeof v === "string" && v !== "") return v;
  return "";
};
const TEXT_SEP = ":text:";
// A text segment's id is "<run>:text:<segment>" (chatapp).
export const parentOf = (segmentId: string): string => {
  const i = segmentId.lastIndexOf(TEXT_SEP);
  return i > 0 ? segmentId.slice(0, i) : "";
};
const APPEND = new Set(["CHAT_STREAM_PATCH_MODE_APPEND", "CHAT_STREAM_PATCH_MODE_UNSPECIFIED", "", undefined]);
const applyPatch = (current: string, patch: string, mode: unknown) => (APPEND.has(mode as any) ? current + patch : patch);
const mergeCorrelation = (a: Payload | undefined, b: Payload | undefined): Payload | undefined => {
  if (!b) return a;
  const out: Payload = { ...(a || {}) };
  for (const [k, v] of Object.entries(b)) if (v !== "" && v !== undefined && v !== null) out[k] = v;
  return out;
};
const addUsage = (a: Payload | undefined, b: Payload | undefined): Payload => {
  const out: Payload = {};
  for (const k of ["inputTokens", "outputTokens", "cachedTokens", "cacheCreationInputTokens", "cacheReadInputTokens"]) {
    const v = num(a?.[k]) + num(b?.[k]);
    if (v) out[k] = v;
  }
  return out;
};

export class Timeline {
  #entities = new Map<string, Entity>();
  #ordinal = 0; // the ordinal the copy is current at
  #fallback = 0; // creation order for events without an ordinal
  /** Increases with every change. */
  version = 0;

  /** The ordinal of the last snapshot or event applied. */
  get ordinal(): number {
    return this.#ordinal;
  }

  /** A snapshot replaces the copy. */
  reset(snap: Snapshot): void {
    this.#entities.clear();
    for (const e of snap.entities ?? []) {
      if (!e.tombstone) this.#entities.set(e.kind + "/" + e.id, { kind: e.kind, id: e.id, created: num(e.createdOrdinal), payload: e.payload ?? {} });
    }
    this.#ordinal = num(snap.snapshotOrdinal);
    this.version++;
  }

  /** Applies one UI event; true when the copy changed. An event the copy already reflects is ignored. */
  apply(ev: UIEvent): boolean {
    const ord = num(ev.ordinal);
    if (ord && ord <= this.#ordinal) return false;
    const changed = this.#reduce(ev.name, ev.payload ?? {}, ord);
    if (ord) this.#ordinal = ord;
    if (changed) this.version++;
    return changed;
  }

  entity(kind: string, id: string): Entity | undefined {
    return this.#entities.get(kind + "/" + id);
  }

  /** Entities (of one kind), in creation order. */
  entities(kind?: string): Entity[] {
    const out = [...this.#entities.values()].filter((e) => !kind || e.kind === kind);
    return out.sort((a, b) => a.created - b.created);
  }

  /** Whether a run of the session is running. */
  get running(): boolean {
    return this.entities("ChatRun").some((e) => e.payload.status === "running");
  }

  #get(kind: string, id: string): Payload | undefined {
    return this.#entities.get(kind + "/" + id)?.payload;
  }

  #put(kind: string, id: string, payload: Payload, ord: number): true {
    const key = kind + "/" + id;
    const cur = this.#entities.get(key);
    this.#entities.set(key, { kind, id, created: cur ? cur.created : ord || ++this.#fallback, payload });
    return true;
  }

  #run(id: string, ord: number, fn: (r: Payload) => void): boolean {
    if (!id) return false;
    const r: Payload = { ...(this.#get("ChatRun", id) ?? {}), messageId: id };
    fn(r);
    return this.#put("ChatRun", id, r, ord);
  }

  #reduce(name: string, p: Payload, ord: number): boolean {
    const id = str(p.messageId).trim();
    switch (name) {
      case "ChatUserMessageAccepted": {
        if (!id) return false;
        const content = first(p.content, p.text);
        return this.#put("ChatMessage", id, {
          messageId: id, role: first(p.role, "user"), prompt: p.prompt, content, text: content,
          status: first(p.status, "accepted"), attachments: p.attachments, startedAtMs: p.atMs, finishedAtMs: p.atMs,
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
          messageId: id, role: "error", content, text: content, status: first(p.status, "failed"), final: true,
          correlation: p.correlation, startedAtMs: p.finishedAtMs, finishedAtMs: p.finishedAtMs,
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
          messageId: p.messageId, toolCallId: call, toolName: p.toolName, result: p.result, status: p.status, correlation: p.correlation,
        }, ord);
      }
      case "ChatToolCallProgress": {
        const call = str(p.toolCallId);
        if (!call) return false;
        const cur: Payload = { ...(this.#get("ChatToolCall", call) ?? {}), toolCallId: call };
        if (!cur.messageId) cur.messageId = p.messageId;
        cur.state = p.state;
        return this.#put("ChatToolCall", call, cur, ord);
      }
      case "ChatFrontendToolCallRequested": {
        const call = str(p.toolCallId);
        if (!call) return false;
        return this.#put("ChatFrontendToolCall", call, {
          toolCallId: call, toolName: p.toolName, parentMessageId: p.messageId, mode: p.mode, status: first(p.status, "requested"),
          input: p.input, executor: p.executor, requestedAtMs: p.atMs,
        }, ord);
      }
      case "ChatFrontendToolResultReceived": {
        const call = str(p.toolCallId);
        if (!call) return false;
        const cur: Payload = { ...(this.#get("ChatFrontendToolCall", call) ?? {}), toolCallId: call };
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
  #text(name: string, id: string, p: Payload, ord: number): boolean {
    if (!id) return false;
    const had = this.#get("ChatMessage", id);
    const e: Payload = { ...(had ?? {}) };
    const current = first(e.content, e.text);
    let content: string;
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
  #thinking(name: string, id: string, p: Payload, ord: number): boolean {
    if (!id) return false;
    const had = this.#get("ChatMessage", id);
    const e: Payload = { ...(had ?? {}) };
    let content: string;
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
  #toolCall(name: string, p: Payload, ord: number): boolean {
    const call = str(p.toolCallId);
    const status = first(
      p.status,
      { ChatToolCallStarted: "pending", ChatToolArgumentsPatch: "streaming_args", ChatToolCallRequested: "pending", ChatToolExecutionStarted: "executing", ChatToolCallFinished: "completed" }[name],
    );
    const input = name === "ChatToolArgumentsPatch" ? str(p.arguments) : name === "ChatToolCallFinished" ? "" : str(p.input);
    const e: Payload = { ...(this.#get("ChatToolCall", call) ?? {}) };
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
  answers(): Turn[] {
    const turns = new Map<string, { turn: Turn; order: number }>();
    const turnOf = (run: string, order: number) => {
      let t = turns.get(run);
      if (!t) turns.set(run, (t = { turn: { question: null, answer: null }, order }));
      t.order = Math.min(t.order, order);
      return t.turn;
    };
    const answers = new Map<string, Answer>();
    for (const e of this.entities("ChatRun")) {
      const a = toAnswer(e.payload, e.id);
      answers.set(e.id, a);
      turnOf(e.id, e.created).answer = a;
    }
    const tools = new Map<string, { part: ToolPart; created: number }>();
    const parts: { run: string; created: number; part: Part }[] = [];
    for (const e of this.entities()) {
      const p = e.payload;
      if (e.kind === "ChatMessage") {
        if (p.role === "user") {
          const run = e.id.endsWith("-user") ? e.id.slice(0, -5) : e.id;
          turnOf(run, e.created).question = { id: e.id, text: first(p.text, p.content), at: num(p.startedAtMs) };
        } else if (p.role === "assistant" || p.role === "thinking") {
          parts.push({
            run: str(p.parentMessageId), created: e.created,
            part: { kind: p.role === "thinking" ? "thinking" : "text", id: e.id, text: first(p.text, p.content), streaming: !!p.streaming, startedAt: num(p.startedAtMs), finishedAt: num(p.finishedAtMs) },
          });
        }
      } else if (e.kind === "ChatToolCall" || e.kind === "ChatToolResult" || e.kind === "ChatFrontendToolCall") {
        const call = str(p.toolCallId) || e.id;
        let t = tools.get(call);
        if (!t) {
          t = { part: { kind: "tool", id: call, name: "", input: undefined, result: undefined, error: "", status: "", state: "", where: "server", mode: "", executor: null, startedAt: 0, finishedAt: 0 }, created: e.created };
          tools.set(call, t);
          parts.push({ run: str(e.kind === "ChatFrontendToolCall" ? p.parentMessageId : p.messageId), created: e.created, part: t.part });
        }
        mergeTool(t.part, e.kind, p);
        const run = str(e.kind === "ChatFrontendToolCall" ? p.parentMessageId : p.messageId);
        const holder = parts.find((x) => x.part === t!.part);
        if (holder && !holder.run && run) holder.run = run;
      }
    }
    parts.sort((a, b) => a.created - b.created);
    for (const { run, part } of parts) {
      const a = answers.get(run);
      if (a) a.parts.push(part);
    }
    for (const a of answers.values()) a.text = a.parts.filter((x) => x.kind === "text").map((x) => (x as TextPart).text).join("\n\n");
    return [...turns.values()].sort((a, b) => a.order - b.order).map((t) => t.turn);
  }

  /** One answer by its run's message id. */
  answer(id: string): Answer | null {
    for (const t of this.answers()) if (t.answer?.id === id) return t.answer;
    return null;
  }
}

function toAnswer(p: Payload, id: string): Answer {
  const u = p.usage ?? {};
  return {
    id, status: (first(p.status, "running") as Answer["status"]), code: str(p.code), error: str(p.error),
    truncated: !!p.truncated, reason: str(p.reason), model: str(p.model),
    startedAt: num(p.startedAtMs), finishedAt: num(p.finishedAtMs),
    usage: { input: num(u.inputTokens), output: num(u.outputTokens), cachedInput: num(u.cachedTokens) + num(u.cacheReadInputTokens) },
    providerCalls: num(p.providerCalls), parts: [], text: "",
  };
}

const parseMaybe = (v: unknown): unknown => {
  if (typeof v !== "string" || v === "") return v === "" ? undefined : v;
  try {
    return JSON.parse(v);
  } catch {
    return v;
  }
};

function mergeTool(t: ToolPart, kind: string, p: Payload): void {
  t.name = first(p.toolName, t.name);
  if (kind === "ChatFrontendToolCall") {
    t.where = "browser";
    if (p.input !== undefined) t.input = p.input;
    if (p.result !== undefined) t.result = p.result;
    t.error = first(p.error, t.error);
    t.status = first(p.status, t.status);
    t.mode = first(p.mode, t.mode);
    t.executor = p.executor ?? t.executor;
    t.startedAt = num(p.requestedAtMs) || t.startedAt;
    t.finishedAt = num(p.finishedAtMs) || t.finishedAt;
  } else if (kind === "ChatToolCall") {
    if (t.input === undefined && p.input !== undefined) t.input = parseMaybe(p.input);
    if (t.where === "server") t.status = first(p.status, t.status);
    t.state = str(p.state);
    t.startedAt = t.startedAt || num(p.startedAtMs);
    t.finishedAt = t.finishedAt || num(p.finishedAtMs);
  } else if (kind === "ChatToolResult") {
    if (t.result === undefined) t.result = parseMaybe(p.result);
    if (p.status === "error" && !t.error) t.error = typeof t.result === "string" ? t.result : "error";
  }
}
