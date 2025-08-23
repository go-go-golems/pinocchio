import { create } from "https://esm.sh/zustand@4.5.2";
import { devtools } from "https://esm.sh/zustand@4.5.2/middleware";
import { subscribeWithSelector } from "https://esm.sh/zustand@4.5.2/middleware";

export const useStore = create(devtools(subscribeWithSelector((set, get)=>({
  app: {
    convId: '',
    runId: '',
    status: 'idle',
  },
  setConvId: (v)=> set((s)=>({ app: { ...s.app, convId: v } }), false, { type: 'app/setConvId', payload: { convId: v } }),
  setRunId: (v)=> set((s)=>({ app: { ...s.app, runId: v } }), false, { type: 'app/setRunId', payload: { runId: v } }),
  setStatus: (v)=> set((s)=>({ app: { ...s.app, status: v } }), false, { type: 'app/setStatus', payload: { status: v } }),

  

  // Normalized timeline slice
  timeline: {
    byId: {},
    order: [],
  },
  // Internal helpers (no devtools action names) — use sparingly
  tlCreated: ({ entityId, kind, renderer, props, startedAt })=> set((s)=>{
    if (s.timeline.byId[entityId]) return {};
    const entity = {
      id: entityId,
      kind,
      renderer: renderer || { kind },
      props: { ...(props || {}) },
      startedAt: startedAt || Date.now(),
      completed: false,
      result: null,
      version: 0,
      updatedAt: null,
      completedAt: null,
    };
    return {
      timeline: {
        byId: { ...s.timeline.byId, [entityId]: entity },
        order: [ ...s.timeline.order, entityId ],
      }
    };
  }),
  tlUpdated: ({ entityId, patch, version, updatedAt })=> set((s)=>{
    const existing = s.timeline.byId[entityId];
    if (!existing) return {};
    const updated = {
      ...existing,
      props: { ...existing.props, ...(patch || {}) },
      version: version || (existing.version + 1),
      updatedAt: updatedAt || Date.now(),
    };
    return { timeline: { byId: { ...s.timeline.byId, [entityId]: updated }, order: s.timeline.order } };
  }),
  tlCompleted: ({ entityId, result })=> set((s)=>{
    const entity = s.timeline.byId[entityId];
    if (!entity) return {};
    const completed = {
      ...entity,
      completed: true,
      completedAt: Date.now(),
      result: result !== undefined ? result : entity.result,
      props: (result && typeof result === 'object') ? { ...entity.props, ...result } : entity.props,
    };
    return { timeline: { byId: { ...s.timeline.byId, [entityId]: completed }, order: s.timeline.order } };
  }),
  tlDeleted: ({ entityId })=> set((s)=>{
    if (!s.timeline.byId[entityId]) return {};
    const nextById = { ...s.timeline.byId };
    delete nextById[entityId];
    const idx = s.timeline.order.indexOf(entityId);
    const nextOrder = idx >= 0 ? [ ...s.timeline.order.slice(0, idx), ...s.timeline.order.slice(idx+1) ] : s.timeline.order;
    return { timeline: { byId: nextById, order: nextOrder } };
  }),
  tlClear: ()=> set((s)=>({ timeline: { byId: {}, order: [] } })),

  // Debug slice (for devtools visibility, minimal state impact)
  debug: {
    recvCount: 0,
    lastWsPayload: null,
    lastSemEvent: null,
    lastSemType: '',
  },

  // Simple dedupe buffer for user messages we just sent
  recentUserMsgs: [],
  _pushRecentUserText: (text)=>{
    const now = Date.now();
    const arr = get().recentUserMsgs || [];
    const next = [ ...arr.filter((x)=> now - x.ts < 5000), { text: String(text || ''), ts: now } ];
    set({ recentUserMsgs: next }, false, { type: 'debug/recentUserMsgs:add', payload: { text: String(text || '') } });
  },
  _seenRecentUserText: (text)=>{
    const now = Date.now();
    const arr = get().recentUserMsgs || [];
    const t = String(text || '');
    return arr.some((x)=> x.text === t && now - x.ts < 5000);
  },

  // Higher-level semantic actions — these alone should appear in devtools
  llmTextStart: (entityId, role = 'assistant', metadata = undefined)=> set((s)=>{
    const exists = s.timeline.byId[entityId];
    if (exists) return {};
    const entity = {
      id: entityId,
      kind: 'llm_text',
      renderer: { kind: 'llm_text' },
      props: { role, text: '', streaming: true, metadata },
      startedAt: Date.now(),
      completed: false,
      result: null,
      version: 0,
      updatedAt: null,
      completedAt: null,
    };
    return { timeline: { byId: { ...s.timeline.byId, [entityId]: entity }, order: [ ...s.timeline.order, entityId ] } };
  }, false, { type: 'sem/llm.start', payload: ({ entityId, role, hasMeta: !!metadata }) }),

  llmTextAppend: (entityId, delta)=> set((s)=>{
    const e = s.timeline.byId[entityId];
    if (!e) return {};
    const cur = (e.props && e.props.text) || '';
    const updated = {
      ...e,
      props: { ...e.props, text: `${cur}${delta || ''}`, streaming: true },
      version: e.version + 1,
      updatedAt: Date.now(),
    };
    return { timeline: { byId: { ...s.timeline.byId, [entityId]: updated }, order: s.timeline.order } };
  }, false, { type: 'sem/llm.delta', payload: ({ entityId, deltaLen: (typeof delta === 'string' ? delta.length : 0) }) }),

  llmTextFinal: (entityId, text, metadata = undefined)=> set((s)=>{
    const e = s.timeline.byId[entityId];
    if (!e) return {};
    const updated = {
      ...e,
      props: { ...e.props, text: text || '', streaming: false, metadata },
      version: e.version + 1,
      updatedAt: Date.now(),
      completed: true,
      completedAt: Date.now(),
      result: { text: text || '' },
    };
    return { timeline: { byId: { ...s.timeline.byId, [entityId]: updated }, order: s.timeline.order } };
  }, false, { type: 'sem/llm.final', payload: ({ entityId, textLen: (typeof text === 'string' ? text.length : 0), hasMeta: !!metadata }) }),

  toolCallStart: (entityId, name, input)=> set((s)=>{
    if (s.timeline.byId[entityId]) return {};
    const entity = {
      id: entityId,
      kind: 'tool_call',
      renderer: { kind: 'tool_call' },
      props: { name, input, exec: true },
      startedAt: Date.now(),
      completed: false,
      result: null,
      version: 0,
      updatedAt: null,
      completedAt: null,
    };
    return { timeline: { byId: { ...s.timeline.byId, [entityId]: entity }, order: [ ...s.timeline.order, entityId ] } };
  }, false, { type: 'sem/tool.start', payload: ({ entityId, name }) }),

  toolCallDelta: (entityId, patch)=> set((s)=>{
    const e = s.timeline.byId[entityId];
    if (!e) return {};
    const updated = {
      ...e,
      props: { ...e.props, ...(patch || {}) },
      version: e.version + 1,
      updatedAt: Date.now(),
    };
    return { timeline: { byId: { ...s.timeline.byId, [entityId]: updated }, order: s.timeline.order } };
  }, false, { type: 'sem/tool.delta', payload: ({ entityId, keys: patch ? Object.keys(patch) : [] }) }),

  toolCallDone: (entityId)=> set((s)=>{
    const e = s.timeline.byId[entityId];
    if (!e) return {};
    const updated = {
      ...e,
      props: { ...e.props, exec: false },
      completed: true,
      completedAt: Date.now(),
      version: e.version + 1,
      updatedAt: Date.now(),
    };
    return { timeline: { byId: { ...s.timeline.byId, [entityId]: updated }, order: s.timeline.order } };
  }, false, { type: 'sem/tool.done', payload: ({ entityId }) }),

  toolCallResult: (baseId, result)=> set((s)=>{
    const entityId = (typeof baseId === 'string' && (baseId.endsWith(':custom') || baseId.endsWith(':result'))) ? baseId : `${baseId}:result`;
    const exists = s.timeline.byId[entityId];
    if (!exists) {
      const entity = {
        id: entityId,
        kind: 'tool_call_result',
        renderer: { kind: 'tool_call_result' },
        props: { result },
        startedAt: Date.now(),
        completed: false,
        result: null,
        version: 0,
        updatedAt: null,
        completedAt: null,
      };
      const byId = { ...s.timeline.byId, [entityId]: entity };
      const order = [ ...s.timeline.order, entityId ];
      return { timeline: { byId, order } };
    }
    const updated = {
      ...exists,
      props: { ...exists.props, result },
      completed: true,
      completedAt: Date.now(),
      version: exists.version + 1,
      updatedAt: Date.now(),
      result,
    };
    return { timeline: { byId: { ...s.timeline.byId, [entityId]: updated }, order: s.timeline.order } };
  }, false, { type: 'sem/tool.result', payload: ({ baseId, hasCustomSuffix: typeof baseId === 'string' && (baseId.endsWith(':custom') || baseId.endsWith(':result')) }) }),

  // User prompt orchestration: create local user message + POST /chat
  sendPrompt: async (text)=>{
    const t = String(text || '').trim();
    if (!t) return;
    const id = `user-${Date.now()}-${Math.random().toString(36).slice(2,6)}`;
    set((s)=>{
      const entity = {
        id,
        kind: 'llm_text',
        renderer: { kind: 'llm_text' },
        props: { role: 'user', text: t, streaming: false },
        startedAt: Date.now(),
        completed: true,
        result: { text: t },
        version: 0,
        updatedAt: Date.now(),
        completedAt: Date.now(),
      };
      return { timeline: { byId: { ...s.timeline.byId, [id]: entity }, order: [ ...s.timeline.order, id ] } };
    }, false, { type: 'sem/user.prompt', payload: ({ id, len: t.length }) });
    try { get()._pushRecentUserText(t); } catch(_){ }
    await get().startChat(t);
  },

  ws: {
    connected: false,
    url: '',
    instance: null,
    reconnectAttempts: 0,
  },
  wsConnect: (convId)=>{
    try {
      const state = get();
      if (state.ws && state.ws.instance) {
        try { state.ws.instance.close(); } catch(_){}
      }
      const proto = location.protocol === 'https:' ? 'wss' : 'ws';
      const url = `${proto}://${location.host}/ws?conv_id=${encodeURIComponent(convId)}`;
      const n = new WebSocket(url);
      set((s)=>({ app: { ...s.app, status: 'connecting ws...' }, ws: { ...s.ws, url, instance: n } }), false, { type: 'ws/connect', payload: { url } });
      n.onopen = ()=> { get().wsOnOpen(); };
      n.onclose = ()=> { get().wsOnClose(); };
      n.onerror = (err)=> { get().wsOnError(err); };
      n.onmessage = (ev)=>{
        try {
          const payload = JSON.parse(ev.data);
          get().wsOnMessage(payload);
        } catch(e) {
          get().wsOnError(e);
        }
      };
    } catch(err) {
      get().wsOnError(err);
    }
  },
  wsDisconnect: ()=>{
    const inst = get().ws.instance;
    if (inst) { try { inst.close(); } catch(_){} }
    set((s)=>({ ws: { ...s.ws, instance: null, connected: false } }), false, { type: 'ws/disconnect' });
  },
  wsOnOpen: ()=>{
    set((s)=>({ app: { ...s.app, status: 'ws connected' }, ws: { ...s.ws, connected: true } }), false, { type: 'ws/onOpen' });
  },
  wsOnClose: ()=>{
    set((s)=>({ app: { ...s.app, status: 'ws closed' }, ws: { ...s.ws, connected: false } }), false, { type: 'ws/onClose' });
  },
  wsOnError: (err)=>{
    set((s)=>({ app: { ...s.app, status: 'ws error' }, ws: { ...s.ws, connected: false } }), false, { type: 'ws/onError', payload: { message: String(err && err.message || err) } });
  },
  wsOnMessage: (payload)=>{
    // Log raw WS payload to devtools without mutating UI state
    set((s)=>({ debug: { ...s.debug, lastWsPayload: payload } }), false, { type: 'ws/message', payload });
    get().handleIncoming(payload);
  },

  // SEM-only ingestion and routing
  handleIncoming: (payload)=>{
    if (!payload || !payload.sem || !payload.event) return;
    const ev = payload.event;
    // Log semantic event to devtools and update debug counters
    set((s)=>({ debug: { ...s.debug, lastSemEvent: ev, lastSemType: ev.type || '', recvCount: (s.debug?.recvCount || 0) + 1 } }), false, { type: 'sem/recv', payload: ev });
    switch (ev.type) {
      case 'llm.start':
        get().llmTextStart(ev.id, ev.role || 'assistant', ev.metadata);
        return;
      case 'llm.delta':
        get().llmTextAppend(ev.id, ev.delta || '');
        return;
      case 'llm.final':
        get().llmTextFinal(ev.id, ev.text || '', ev.metadata);
        return;
      case 'tool.start':
        get().toolCallStart(ev.id, ev.name, ev.input);
        return;
      case 'tool.delta':
        get().toolCallDelta(ev.id, ev.patch || {});
        return;
      case 'tool.done':
        get().toolCallDone(ev.id);
        return;
      case 'tool.result': {
        const resultId = ev.customKind ? `${ev.id}:custom` : `${ev.id}:result`;
        set((s)=>{
          const exists = s.timeline.byId[resultId];
          if (!exists) {
            const entity = { id: resultId, kind: ev.customKind || 'tool_call_result', renderer: { kind: ev.customKind || 'tool_call_result' }, props: { result: ev.result }, startedAt: Date.now(), completed: false, result: null, version: 0, updatedAt: null, completedAt: null };
            return { timeline: { byId: { ...s.timeline.byId, [resultId]: entity }, order: [ ...s.timeline.order, resultId ] } };
          }
          const updated = { ...exists, props: { ...exists.props, result: ev.result }, completed: true, completedAt: Date.now(), version: exists.version + 1, updatedAt: Date.now(), result: ev.result };
          return { timeline: { byId: { ...s.timeline.byId, [resultId]: updated }, order: s.timeline.order } };
        }, false, { type: 'sem/tool.result', payload: ({ id: resultId }) });
        return;
      }
      case 'agent.mode': {
        const id = ev.id || `agentmode-${Date.now()}`;
        set((s)=>{
          const entity = { id, kind: 'agent_mode', renderer: { kind: 'agent_mode' }, props: { title: ev.title, from: ev.from, to: ev.to, analysis: ev.analysis }, startedAt: Date.now(), completed: true, result: null, version: 0, updatedAt: Date.now(), completedAt: Date.now() };
          return { timeline: { byId: { ...s.timeline.byId, [id]: entity }, order: [ ...s.timeline.order, id ] } };
        }, false, { type: 'sem/agent.mode', payload: ({ id }) });
        return;
      }
      case 'log': {
        const id = ev.id || `log-${Date.now()}`;
        set((s)=>{
          const entity = { id, kind: 'log_event', renderer: { kind: 'log_event' }, props: { level: ev.level || 'info', message: ev.message, fields: ev.fields }, startedAt: Date.now(), completed: true, result: { message: ev.message }, version: 0, updatedAt: Date.now(), completedAt: Date.now() };
          return { timeline: { byId: { ...s.timeline.byId, [id]: entity }, order: [ ...s.timeline.order, id ] } };
        }, false, { type: 'sem/log', payload: ({ id }) });
        return;
      }
      default:
        return;
    }
  },

  // HTTP chat action
  startChat: async (prompt)=>{
    const convId = get().app.convId;
    const body = convId ? { prompt, conv_id: convId } : { prompt };
    const res = await fetch('/chat', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    const j = await res.json();
    const newRun = j.run_id || '';
    const newConv = j.conv_id || convId || '';
    set((s)=>({ app: { ...s.app, runId: newRun, convId: newConv } }), false, { type: 'chat/startChat:received', payload: { runId: newRun, convId: newConv } });
    if (newConv && newConv !== convId) {
      get().wsConnect(newConv);
    }
  },
})), { name: 'web-chat' }));


