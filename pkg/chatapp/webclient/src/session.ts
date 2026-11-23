// Session is one chatapp session seen from a page: it keeps a Timeline current
// from the server's event stream (GET {base}/sessions/{id}/events), sends
// messages, stops runs, and runs the browser tools the page registered.
//
// One path for every change: a snapshot (first connect, or any reconnect)
// replaces the timeline, a UI event updates it, and listeners, waiting sends
// and browser tool calls all read the timeline after the change. There is no
// separate recovery logic.
import { type Answer, type Snapshot, TERMINAL, Timeline, type UIEvent } from "./timeline.ts";

export type Options = {
  /** fetch to use (a page's own transport); default globalThis.fetch. */
  fetch?: typeof fetch;
  /** Extra request headers. */
  headers?: Record<string, string>;
};
export type SessionInfo = {
  id: string;
  title?: string;
  labels?: Record<string, string>;
  created?: string;
  updated?: string;
  usage?: Record<string, number>;
  data?: unknown;
  [k: string]: unknown;
};
export type BrowserTool = {
  name: string;
  description?: string;
  inputSchema?: object;
  /** "human": a person approves each call (the page asks); the server waits longer. */
  mode?: "frontend" | "human";
  execute: (input: any, ctx: { signal: AbortSignal; callId: string }) => unknown;
};

/** An error answer from the server: {error, code} with the HTTP status. */
export class ChatError extends Error {
  code: string;
  status: number;
  constructor(code: string, message: string, status = 0) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

const clean = (o: Record<string, unknown>) => Object.fromEntries(Object.entries(o).filter(([, v]) => v !== undefined && v !== null && v !== ""));
const rid = (p: string) => p + Math.random().toString(36).slice(2, 10);
const sleep = (ms: number, signal?: AbortSignal) =>
  new Promise<void>((resolve) => {
    const t = setTimeout(resolve, ms);
    signal?.addEventListener("abort", () => (clearTimeout(t), resolve()), { once: true });
  });

async function request(base: string, method: string, path: string, body: unknown, o: Options): Promise<any> {
  const f = o.fetch ?? globalThis.fetch;
  const r = await f(base + path, {
    method,
    headers: { ...(body === undefined ? {} : { "Content-Type": "application/json" }), ...(o.headers ?? {}) },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const txt = await r.text();
  let data: any = null;
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

/** Reads server-sent events from a response body: calls fn(event, data) per frame. */
export async function readSSE(body: ReadableStream<Uint8Array>, fn: (event: string, data: string) => Promise<void> | void): Promise<void> {
  const reader = body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  for (;;) {
    const x = await reader.read();
    if (x.done) return;
    buf += dec.decode(x.value, { stream: true }).replace(/\r\n/g, "\n");
    let i: number;
    while ((i = buf.indexOf("\n\n")) >= 0) {
      const block = buf.slice(0, i);
      buf = buf.slice(i + 2);
      let event = "message";
      const data: string[] = [];
      for (const line of block.split("\n")) {
        if (line.startsWith("event:")) event = line.slice(6).trim();
        else if (line.startsWith("data:")) data.push(line.slice(5).replace(/^ /, ""));
      }
      if (data.length) await fn(event, data.join("\n"));
    }
  }
}

type Outbox = { toolName: string; status: string; result: unknown; error: string; sending: boolean; tries: number; next: number };

export class Session {
  readonly id: string;
  readonly base: string;
  readonly timeline = new Timeline();
  info: SessionInfo;
  readonly #o: Options;
  readonly #listeners = new Set<(ev: UIEvent | null) => void>();
  readonly #waits = new Set<() => void>();
  readonly #stream = new AbortController();
  #closed = false;
  #ready!: Promise<void>;
  #error: ChatError | null = null;
  // browser tools
  #tools: BrowserTool[] = [];
  readonly #client = rid("page-");
  readonly #connection = rid("c-");
  #revision = 0;
  #executor: any = null;
  readonly #ran = new Set<string>();
  readonly #outbox = new Map<string, Outbox>();
  readonly #calls = new Map<string, AbortController>();

  private constructor(base: string, id: string, info: SessionInfo, o: Options) {
    this.base = base.replace(/\/$/, "");
    this.id = id;
    this.info = info;
    this.#o = o;
  }

  /** Creates a session ({labels, data, id?}) and opens it. */
  static async create(base: string, req: { labels?: Record<string, string>; data?: unknown; id?: string } = {}, o: Options = {}): Promise<Session> {
    const out = await request(base.replace(/\/$/, ""), "POST", "/sessions", clean(req as Record<string, unknown>), o);
    return Session.#start(base, out.sessionId, out.session ?? { id: out.sessionId }, o);
  }

  /** Opens an existing session: resolves after its first snapshot. */
  static async open(base: string, id: string, o: Options = {}): Promise<Session> {
    const out = await request(base.replace(/\/$/, ""), "GET", "/sessions/" + encodeURIComponent(id), undefined, o);
    return Session.#start(base, id, out.session ?? { id }, o);
  }

  /** Connects to a session the caller already has (its info from create or list): resolves after the first snapshot. */
  static async attach(base: string, info: SessionInfo, o: Options = {}): Promise<Session> {
    if (!info?.id) throw new ChatError("invalid_request", "attach needs the session's info with its id");
    return Session.#start(base, info.id, info, o);
  }

  /** Lists sessions that have all these labels. */
  static async list(base: string, labels: Record<string, string> = {}, o: Options = {}): Promise<SessionInfo[]> {
    const q = Object.entries(labels).map(([k, v]) => "label=" + encodeURIComponent(k + ":" + v)).join("&");
    const out = await request(base.replace(/\/$/, ""), "GET", "/sessions" + (q ? "?" + q : ""), undefined, o);
    return out?.sessions ?? [];
  }

  static async #start(base: string, id: string, info: SessionInfo, o: Options): Promise<Session> {
    const s = new Session(base, id, info, o);
    let ready!: () => void;
    s.#ready = new Promise<void>((r) => (ready = r));
    void s.#connect(ready);
    await s.#ready;
    if (s.#error) throw s.#error;
    return s;
  }

  /** Calls fn after every change (ev: the UI event; null: a snapshot). Returns an unsubscribe function. */
  on(fn: (ev: UIEvent | null) => void): () => void {
    this.#listeners.add(fn);
    return () => this.#listeners.delete(fn);
  }

  /** Sends a message; resolves with the run's message id (the answer's id). */
  async send(prompt: string, extra: Record<string, unknown> = {}): Promise<string> {
    const out = await this.#api("POST", "/messages", { ...extra, prompt });
    const id = out?.messageId;
    if (!id) throw new ChatError("invalid_response", "the server accepted the message without naming its run");
    return id;
  }

  /** Resolves with the answer once it is finished, failed or stopped. */
  wait(id: string, signal?: AbortSignal): Promise<Answer> {
    return new Promise<Answer>((resolve, reject) => {
      const check = () => {
        if (signal?.aborted) return done(() => reject(signal.reason ?? new ChatError("aborted", "aborted")));
        if (this.#closed) return done(() => reject(this.#error ?? new ChatError("closed", "the session was closed")));
        const a = this.timeline.answer(id);
        if (a && TERMINAL.has(a.status)) done(() => resolve(a));
      };
      const done = (fn: () => void) => {
        this.#waits.delete(check);
        fn();
      };
      this.#waits.add(check);
      signal?.addEventListener("abort", check, { once: true });
      check();
    });
  }

  async stop(): Promise<void> {
    await this.#api("POST", "/stop");
  }

  async delete(): Promise<void> {
    await this.#api("DELETE", "");
    this.close();
  }

  /** Stops the stream and the page's running tool calls. */
  close(): void {
    if (this.#closed) return;
    this.#closed = true;
    this.#stream.abort();
    for (const c of this.#calls.values()) c.abort();
    this.#notify(null);
  }

  /** Registers the page's browser tools (replacing earlier ones) and runs the calls the server assigns to this page. */
  async tools(list: BrowserTool[]): Promise<void> {
    this.#tools = list;
    await this.#register();
    this.#derive();
  }

  async #api(method: string, path: string, body?: unknown): Promise<any> {
    return request(this.base, method, "/sessions/" + encodeURIComponent(this.id) + path, body, this.#o);
  }

  async #register(): Promise<void> {
    const out = await this.#api("POST", "/tools/manifest", {
      clientInstanceId: this.#client,
      connectionId: this.#connection,
      revision: ++this.#revision,
      tools: this.#tools.map((t) => clean({ name: t.name, description: t.description, inputSchema: t.inputSchema ?? { type: "object" }, mode: t.mode ?? "frontend", available: true })),
    });
    this.#executor = out?.executor ?? null;
  }

  async #connect(ready: () => void): Promise<void> {
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
            this.timeline.reset(JSON.parse(data) as Snapshot);
            if (this.#tools.length) await this.#register().catch(() => {});
            for (const o of this.#outbox.values()) o.next = 0; // connected again: deliver now
            ready();
            this.#notify(null);
          } else if (event === "ui") {
            const ev = JSON.parse(data) as UIEvent;
            if (this.timeline.apply(ev)) this.#notify(ev);
          }
          // "error" (stream_overflow, server_closed): the stream ends; reconnect below
        });
      } catch {
        if (this.#closed) return;
      }
      // A new stream starts with a new snapshot: nothing is lost in between.
      await sleep(Math.min(5000, 250 * 2 ** attempt), this.#stream.signal);
    }
  }

  #notify(ev: UIEvent | null): void {
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
  #derive(): void {
    if (!this.#tools.length) return;
    for (const e of this.timeline.entities("ChatFrontendToolCall")) {
      const p = e.payload;
      if (p.status === "requested") {
        if (this.#outbox.has(e.id)) this.#deliver(e.id); // still waited for: deliver again
        else void this.#call(e.id, p);
      } else {
        this.#outbox.delete(e.id); // the server has it, or gave up on the call
        this.#calls.get(e.id)?.abort();
      }
    }
  }

  async #call(id: string, p: any): Promise<void> {
    if (this.#ran.has(id) || !this.#executor || p.executor?.assignmentId !== this.#executor.assignmentId) return;
    this.#ran.add(id);
    const tool = this.#tools.find((t) => t.name === p.toolName);
    const ctrl = new AbortController();
    this.#calls.set(id, ctrl);
    let status = "success";
    let result: unknown = null;
    let error = "";
    try {
      if (!tool) throw new Error("this page has no tool " + p.toolName);
      result = await tool.execute(p.input ?? {}, { signal: ctrl.signal, callId: id });
      if (result === undefined) result = null;
      if (JSON.stringify(result) === undefined) result = String(result);
    } catch (e: any) {
      status = "failed";
      error = String(e?.message ?? e);
    } finally {
      this.#calls.delete(id);
    }
    if (ctrl.signal.aborted || this.#closed) return; // the run was stopped: the server no longer waits
    this.#outbox.set(id, { toolName: p.toolName, status, result, error, sending: false, tries: 0, next: 0 });
    this.#deliver(id);
  }

  // One delivery attempt; a network or server failure keeps the result for a later attempt.
  #deliver(id: string): void {
    const r = this.#outbox.get(id);
    if (!r || r.sending || Date.now() < r.next) return;
    r.sending = true;
    this.#api("POST", "/tools/results", clean({ toolCallId: id, toolName: r.toolName, status: r.status, result: r.result, error: r.error, executor: this.#executor }))
      .then(() => this.#outbox.delete(id))
      .catch((e: any) => {
        if (e instanceof ChatError && e.status >= 400 && e.status < 500) this.#outbox.delete(id); // late, unknown or invalid: final
        else {
          r.tries++;
          const wait = Math.min(5000, 200 * 2 ** r.tries);
          r.next = Date.now() + wait;
          setTimeout(() => !this.#closed && this.#derive(), wait);
        }
      })
      .finally(() => {
        r.sending = false;
      });
  }
}
