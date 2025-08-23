### Design: Move web-chat to a semantic, Zustand-driven timeline and app state

This document analyzes the current timeline/WebSocket wiring in the web chat and proposes a redesign where all app behavior is driven by a single Zustand store with semantic actions. The goal is to eliminate ad-hoc module state, reduce DOM imperative updates, and route every WebSocket and user action through a clear, testable API.

---

### 1) Current wiring and behavior

- **Files involved**:
  - `pinocchio/cmd/web-chat/web/src/app.js`
  - `pinocchio/cmd/web-chat/web/src/timeline/store.js`

- **Current state holders**:
  - Module-level `state` in `app.js`:
    - `convId`, `runId`, `ws`, `status`, and `timelineStore` (custom class instance)
  - `useDevStore` (Zustand) only for devtools/inspection:
    - `convId`, `runId`, `status`, `wsConnected`, `timelineCounts` with trivial setters (`setStatus`, `setWsConnected`, etc.)
  - Renderer: `mount()` subscribes to `timelineStore` and imperatively calls `render()` + sets `#status` text content.

- **WebSocket flow** (in `app.js`):
  - `connectConv(convId)` opens WS and sets `onopen/onclose/onerror/onmessage`.
  - `onmessage` calls `handleEvent(JSON.parse(ev.data))`.
  - `handleEvent`:
    - If legacy `{ type: 'user' }`, it synthesizes two events directly into `timelineStore`: a `created` and a `completed` `llm_text` entity (non-streaming), then returns.
    - Else it expects TL-wrapped payload: `{ tl: true, event }` and calls `handleTimelineEvent(event)` → `timelineStore.applyEvent(event)`.

- **Timeline store** (in `timeline/store.js`):
  - A custom class holding `entities: Map`, `order: string[]`, and a simple pub/sub for UI notifications.
  - Methods: `applyEvent`, `onCreate`, `onUpdate`, `onComplete`, `onDelete`, plus selectors: `getEntity`, `getOrderedEntities`, `getEntitiesByKind`, `getStats`, `clear`.
  - Special rule: remove generic `tool_call_result` when a custom `<tool>_result` exists to avoid duplicate widgets.

- **Chat flow**:
  - `startChat(prompt)` POSTs `/chat`, records `runId` and maybe a new `convId` from response, then calls `connectConv()` for the conv, and waits for server-driven user echo via WS to avoid duplicates.

---

### 2) Problem statement

- App logic is split across module state (`state`), DOM side effects (setting `#status`), a custom `TimelineStore`, and a partial Zustand `useDevStore`.
- There are only trivial actions in Zustand (`setStatus`, etc.). Semantic actions are missing.
- Testing specific behaviors (e.g., tool streaming, WS reconnect, entity lifecycle) is harder because the behavior is not encapsulated in a single store boundary.
- Components cannot subscribe declaratively to domain state; they rely on `timelineStore.subscribe` and manual re-render triggers.

---

### 3) Goals for the redesign

- **Single source of truth**: One Zustand store managing app, websocket, chat, and timeline state.
- **Semantic actions**: First-class, domain-specific actions (not just setters) for both user intent and WebSocket-driven events.
- **Normalized timeline**: Keep the timeline as normalized data (by `id` + ordered list), with lifecycle semantics preserved.
- **Declarative UI**: Components derive state via selectors; no manual DOM updates.
- **Testability**: Actions are pure (or orchestrated) functions that can be unit-tested without the DOM.
- **Devtools**: Keep devtools integration; leverage action names for time-travel and debugging.

---

### 4) Proposed Zustand store structure (slices)

Create a single `useStore` with slices for app, websocket, chat, and timeline. Each slice exposes state and semantic actions.

```javascript
// store/index.js
import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

export const useStore = create(devtools((set, get) => ({
  // App slice (ids, status, UI flags)
  app: {
    convId: '',
    runId: '',
    status: 'idle', // 'connecting', 'connected', 'error', etc.
  },
  setConvId: (convId) => set((s) => ({ app: { ...s.app, convId } }), false, 'app/setConvId'),
  setRunId: (runId) => set((s) => ({ app: { ...s.app, runId } }), false, 'app/setRunId'),
  setStatus: (status) => set((s) => ({ app: { ...s.app, status } }), false, 'app/setStatus'),

  // WebSocket slice
  ws: {
    connected: false,
    url: '',
    instance: null, // store the WebSocket instance
    reconnectAttempts: 0,
  },
  wsConnect: (convId) => {/* open WS, set handlers to dispatch wsOnOpen, wsOnMessage, etc. */},
  wsDisconnect: () => {/* close WS safely and update state */},
  wsOnOpen: () => set((s) => ({ app: { ...s.app, status: 'ws connected' }, ws: { ...s.ws, connected: true } }), false, 'ws/onOpen'),
  wsOnClose: () => set((s) => ({ app: { ...s.app, status: 'ws closed' }, ws: { ...s.ws, connected: false } }), false, 'ws/onClose'),
  wsOnError: (err) => set((s) => ({ app: { ...s.app, status: 'ws error' } }), false, 'ws/onError'),
  wsOnMessage: (payload) => get().handleIncoming(payload),

  // Chat slice (HTTP and user intents)
  startChat: async (prompt) => {/* POST /chat, update convId/runId, maybe wsConnect */},

  // Timeline slice
  timeline: {
    byId: {},
    order: [],
  },
  // semantic lifecycle actions
  tlCreated: ({ entityId, kind, renderer, props, startedAt }) => {/* create entity */},
  tlUpdated: ({ entityId, patch, version, updatedAt }) => {/* update entity + special rules */},
  tlCompleted: ({ entityId, result }) => {/* mark completed and merge result */},
  tlDeleted: ({ entityId }) => {/* remove entity from byId and order */},

  // Higher-level domain actions
  userSendPrompt: (text) => {/* optionally echo user message or rely on server */},
  toolCallStart: (id, input) => {/* create tool_call entity */},
  toolCallStreamUpdate: (id, delta) => {/* patch */},
  toolCallResult: (id, result) => {/* complete + dedupe generic result */},
  llmTextPartial: (id, delta) => {/* ensure entity exists, append text */},
  llmTextFinal: (id, text) => {/* complete with final text */},

  // Event ingestion
  handleIncoming: (msg) => {/* normalize and route to semantic actions */},
})), { name: 'web-chat' });
```

Notes:
- Keep function names action-oriented and domain-specific (e.g., `tlCompleted`, `toolCallResult`, `wsOnMessage`).
- The `handleIncoming` function encapsulates WS payload normalization, keeping components simple.

---

### 5) Timeline state: normalized model and rules

State shape:

```javascript
timeline: {
  byId: {
    [entityId]: {
      id,
      kind,
      renderer, // { kind: string, ... }
      props,    // renderer props
      version,
      startedAt,
      updatedAt,
      completedAt,
      completed,
      result,
    }
  },
  order: [entityId, ...]
}
```

Lifecycle actions semantics:
- **tlCreated**: insert if not exists; append `entityId` to `order`.
- **tlUpdated**: shallow-merge `patch` into `props`, bump version and `updatedAt`.
- **tlCompleted**: set `completed=true`, set `completedAt`, and if `result` is an object, merge into `props` so widgets get final display data.
- **tlDeleted**: remove from `byId` and `order`.

Special rule (keep from current implementation): when a generic `tool_call_result` entity is present and a corresponding custom `<tool>_result` entity exists, prune the generic one to avoid duplicate widgets.

---

### 6) WebSocket ingestion and normalization

The store ingests WS messages and maps them to semantic actions. We keep a generic lifecycle path and add higher-level mappings where possible.

- **Generic TL routing**: `{ tl: true, event }` → `tlCreated|tlUpdated|tlCompleted|tlDeleted`
- **High-level mappings** (when fields are present):
  - LLM streaming: `created(kind=llm_text, props.streaming=true, role=assistant)` → `llmTextStart`
  - LLM delta: `updated(kind=llm_text, patch.delta)` → `llmTextAppend`
  - LLM text growth: `updated(kind=llm_text, patch.text)` grows previous text → compute `delta` and call `llmTextAppend`
  - LLM final: `completed(kind=llm_text, result.text)` → `llmTextFinal`
  - Tool start: `created(kind=tool_call, props: { name, input })` → `toolCallStart`
  - Tool delta: `updated(kind=tool_call, patch)` → `toolCallDelta`
  - Tool done: `completed(kind=tool_call)` → `toolCallDone`
  - Tool result: `created|completed(kind=tool_call_result or *_result, result|props.result)` → `toolCallResult` (with dedupe for generic vs custom)
- **User echo dedupe**: Locally-created user messages set a short-lived mark; server echoes with the same text are skipped.

Minimal example (concept):

```javascript
handleIncoming = (payload) => {
  if (!payload || !payload.tl || !payload.event) return;
  const ev = payload.event;
  const ent = get().timeline.byId[ev.entityId];
  switch (ev.type) {
    case 'created':
      if (ev.kind === 'llm_text' && ev.props?.streaming && ev.props?.role==='assistant') return get().llmTextStart(ev.entityId, 'assistant', ev.props.metadata);
      if (ev.kind === 'tool_call') return get().toolCallStart(ev.entityId, ev.props?.name, ev.props?.input);
      if (/(_result|tool_call_result)$/.test(ev.kind)) return get().toolCallResult(ev.entityId, ev.props?.result ?? ev.result);
      return get().tlCreated(ev);
    case 'updated':
      if ((ent?.kind ?? ev.kind) === 'llm_text' && ev.patch) {
        const delta = ev.patch.delta || diffText(ent?.props?.text, ev.patch.text);
        if (delta) return get().llmTextAppend(ev.entityId, delta);
      }
      if ((ent?.kind ?? ev.kind) === 'tool_call') return get().toolCallDelta(ev.entityId, ev.patch);
      return get().tlUpdated(ev);
    case 'completed':
      if ((ent?.kind ?? ev.kind) === 'llm_text') return get().llmTextFinal(ev.entityId, ev.result?.text ?? ent?.props?.text, ev.props?.metadata);
      if ((ent?.kind ?? ev.kind) === 'tool_call') return get().toolCallDone(ev.entityId);
      if (/(_result|tool_call_result)$/.test(ent?.kind ?? ev.kind)) return get().toolCallResult(ev.entityId, ev.props?.result ?? ev.result);
      return get().tlCompleted(ev);
  }
};
```

---

### 7) Chat actions and side effects

Encapsulate the HTTP and WS orchestration inside the store and provide `sendPrompt(text)` to create a local user message immediately and POST `/chat`.

```javascript
async function sendPrompt(text) {
  const id = `user-${Date.now()}-${Math.random().toString(36).slice(2,6)}`;
  get().tlCreated({ entityId: id, kind: 'llm_text', renderer: { kind: 'llm_text' }, props: { role: 'user', text, streaming: false } });
  get().tlCompleted({ entityId: id, result: { text } });
  await get().startChat(text);
}
```

---

### 8) UI integration and rendering

- Replace `timelineStore.subscribe` + manual `render()` with component-level selectors:
  - A thin wrapper hook: `const entities = useStore((s) => selectOrderedEntities(s.timeline));`
  - `Timeline` reads `entities` from props or calls the hook directly.
  - `status` is rendered from `useStore((s) => s.app.status)`; no direct DOM manipulation.
- Keep auto-scroll behavior in a small effect after render.

Selector helpers:

```javascript
const selectOrderedEntities = (timeline) => timeline.order
  .map((id) => timeline.byId[id])
  .filter(Boolean);
```

---

### 9) Mapping: WS and user events → semantic actions

| Source event | Payload shape (today) | Action(s) |
| --- | --- | --- |
| WS open | n/a | `wsOnOpen()` |
| WS close | n/a | `wsOnClose()` |
| WS error | Error | `wsOnError(err)` |
| WS message (timeline created) | `{ tl: true, event: { type: 'created', kind, props } }` | `llmTextStart` (assistant+streaming), `toolCallStart`, `toolCallResult`, else `tlCreated` |
| WS message (timeline updated) | `{ tl: true, event: { type: 'updated', patch } }` | `llmTextAppend` (delta or text growth), `toolCallDelta`, else `tlUpdated` |
| WS message (timeline completed) | `{ tl: true, event: { type: 'completed', result } }` | `llmTextFinal`, `toolCallDone`, `toolCallResult`, else `tlCompleted` |
| WS message (timeline deleted) | `{ tl: true, event: { type: 'deleted' } }` | `tlDeleted` |
| User presses Send | `prompt` string | `sendPrompt(prompt)` → `startChat(prompt)` |

Dedupe rules: Local user message echoes from the server are skipped for a short window.

---

### 10) Migration plan

- [x] Introduce `store` with slices and semantic actions.
- [x] Refactor WS handlers into store (`wsConnect`, `wsOnOpen`, `wsOnMessage`, ...).
- [x] Move `startChat` logic into store (`startChat`).
- [x] Replace custom `TimelineStore` with Zustand timeline slice.
- [x] Update `Timeline` to read from store selectors; remove manual `render()` subscription to custom store.
- [x] Remove direct DOM `#status` updates in favor of store-driven state (now mirrored for simplicity).
- [x] Remove legacy timeline modules (`timeline/store.js`, `controller.js`, `registry.js`, `types.js`, `renderers/*`).
- [ ] Add unit tests for semantic actions and ingestion mapping.
- [ ] Optional: add reconnect/backoff and heartbeat.

---

### 11) Risks and considerations

- Ensure WS lifecycle is robust (reconnects, stale instance close) within the store; avoid leaking event handlers.
- Keep actions small and predictable; isolate side effects (network/WS) to a few orchestrator actions.
- Make sure the renderer auto-scroll logic does not fight with user scroll.
- Back-compat: legacy `{ type: 'user' }` path is removed in favor of store-driven `sendPrompt()`; ensure backend WS echoes are TL-wrapped.

---

### 12) Minimal examples (pseudocode)

```javascript
// Wiring WS open
wsConnect = (convId) => {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const url = `${proto}://${location.host}/ws?conv_id=${encodeURIComponent(convId)}`;
  const n = new WebSocket(url);
  set((s) => ({ app: { ...s.app, status: 'connecting ws...' }, ws: { ...s.ws, url, instance: n } }), false, 'ws/connect');
  n.onopen = () => get().wsOnOpen();
  n.onclose = () => get().wsOnClose();
  n.onerror = (err) => get().wsOnError(err);
  n.onmessage = (ev) => {
    try { get().wsOnMessage(JSON.parse(ev.data)); } catch (e) { get().wsOnError(e); }
  };
};

// Timeline creation
tlCreated = ({ entityId, kind, renderer, props, startedAt }) => set((s) => {
  if (s.timeline.byId[entityId]) return {};
  const entity = { id: entityId, kind, renderer: renderer || { kind }, props: { ...(props || {}) }, startedAt: startedAt || Date.now(), completed: false, result: null, version: 0, updatedAt: null, completedAt: null };
  return {
    timeline: {
      byId: { ...s.timeline.byId, [entityId]: entity },
      order: [...s.timeline.order, entityId],
    }
  };
}, false, 'timeline/created');
```

---

### 13) Expected benefits

- Clear, semantic API for all state-changing behavior.
- Easier to test, debug, and evolve (e.g., add new widget types, streaming behaviors).
- UI becomes declarative and predictable, reducing manual DOM touch points.
- Devtools provides a comprehensible action history.

---

### 14) Next steps

- [ ] Confirm final action mappings with backend event shapes.
- [ ] Add tests for mapping logic and dedupe paths.
- [ ] Consider reconnect/backoff and heartbeat.


