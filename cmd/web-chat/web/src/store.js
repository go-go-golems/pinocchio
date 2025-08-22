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
  tlCreated: ({ entityId, kind, renderer, props, startedAt })=> set((s)=>{
    console.debug('tlCreated', { entityId, kind, renderer, props, startedAt });
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
  }, false, { type: 'timeline/created', payload: ({ entityId, kind, hasProps: !!props }) }),
  tlUpdated: ({ entityId, patch, version, updatedAt })=> set((s)=>{
    console.debug('tlUpdated', { entityId, patch, version, updatedAt });
    const existing = s.timeline.byId[entityId];
    let nextById = s.timeline.byId;
    let nextOrder = s.timeline.order;
    let entity = existing;
    if (!existing) {
      // create placeholder, infer kind best-effort
      let inferredKind = 'llm_text';
      if (patch && (Object.prototype.hasOwnProperty.call(patch, 'exec') || Object.prototype.hasOwnProperty.call(patch, 'input'))) {
        inferredKind = 'tool_call';
      }
      entity = {
        id: entityId,
        kind: inferredKind,
        renderer: { kind: inferredKind },
        props: {},
        startedAt: Date.now(),
        completed: false,
        result: null,
        version: 0,
        updatedAt: null,
        completedAt: null,
      };
      nextById = { ...nextById, [entityId]: entity };
      nextOrder = [ ...nextOrder, entityId ];
    }
    const updated = {
      ...entity,
      props: { ...entity.props, ...(patch || {}) },
      version: version || (entity.version + 1),
      updatedAt: updatedAt || Date.now(),
    };
    // Special rule: prune generic tool_call_result if custom exists
    if (updated.kind === 'tool_call_result' && typeof updated.id === 'string') {
      const base = updated.id.replace(/:result$/, '');
      const customId = base + ':custom';
      if (nextById[customId]) {
        const newById = { ...nextById };
        delete newById[updated.id];
        const idx = nextOrder.indexOf(updated.id);
        const newOrder = idx >= 0 ? [ ...nextOrder.slice(0, idx), ...nextOrder.slice(idx+1) ] : nextOrder;
        return { timeline: { byId: newById, order: newOrder } };
      }
    }
    return { timeline: { byId: { ...nextById, [entityId]: updated }, order: nextOrder } };
  }, false, { type: 'timeline/updated', payload: ({ entityId, patchKeys: patch ? Object.keys(patch) : [] }) }),
  tlCompleted: ({ entityId, result })=> set((s)=>{
    console.debug('tlCompleted', { entityId, hasResult: result !== undefined && result !== null });
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
  }, false, { type: 'timeline/completed', payload: ({ entityId }) }),
  tlDeleted: ({ entityId })=> set((s)=>{
    console.debug('tlDeleted', { entityId });
    if (!s.timeline.byId[entityId]) return {};
    const nextById = { ...s.timeline.byId };
    delete nextById[entityId];
    const idx = s.timeline.order.indexOf(entityId);
    const nextOrder = idx >= 0 ? [ ...s.timeline.order.slice(0, idx), ...s.timeline.order.slice(idx+1) ] : s.timeline.order;
    return { timeline: { byId: nextById, order: nextOrder } };
  }, false, { type: 'timeline/deleted', payload: ({ entityId }) }),
  tlClear: ()=> set((s)=>({ timeline: { byId: {}, order: [] } }), false, { type: 'timeline/clear' }),

  // Higher-level semantic actions (optionally used by ingestion or UI orchestration)
  llmTextStart: (entityId, role = 'assistant', metadata = undefined)=>{
    console.debug('llmTextStart', { entityId, role });
    get().tlCreated({ entityId, kind: 'llm_text', renderer: { kind: 'llm_text' }, props: { role, text: '', streaming: true, metadata } });
  },
  llmTextAppend: (entityId, delta)=> set((s)=>{
    console.debug('llmTextAppend', { entityId, deltaLen: (delta || '').length });
    const e = s.timeline.byId[entityId];
    if (!e) return {};
    const cur = (e.props && e.props.text) || '';
    const patch = { text: `${cur}${delta || ''}`, streaming: true };
    return get().tlUpdated({ entityId, patch });
  }, false, { type: 'llm/textAppend', payload: ({ entityId, deltaLen: (typeof delta === 'string' ? delta.length : 0) }) }),
  llmTextFinal: (entityId, text, metadata = undefined)=>{
    console.debug('llmTextFinal', { entityId, textLen: (text || '').length });
    get().tlUpdated({ entityId, patch: { text: text || '', streaming: false, metadata } });
    get().tlCompleted({ entityId, result: { text: text || '' } });
  },

  toolCallStart: (entityId, name, input)=>{
    console.debug('toolCallStart', { entityId, name });
    get().tlCreated({ entityId, kind: 'tool_call', renderer: { kind: 'tool_call' }, props: { name, input, exec: true } });
  },
  toolCallDelta: (entityId, patch)=>{
    console.debug('toolCallDelta', { entityId, keys: patch ? Object.keys(patch) : [] });
    get().tlUpdated({ entityId, patch });
  },
  toolCallDone: (entityId)=>{
    console.debug('toolCallDone', { entityId });
    get().tlUpdated({ entityId, patch: { exec: false } });
    get().tlCompleted({ entityId });
  },
  toolCallResult: (resultEntityId, result)=>{
    console.debug('toolCallResult', { resultEntityId });
    // Generic tool_call_result, will be deduped if a custom result exists
    const exists = get().timeline.byId[resultEntityId];
    if (!exists) get().tlCreated({ entityId: resultEntityId, kind: 'tool_call_result', renderer: { kind: 'tool_call_result' }, props: { result } });
    get().tlUpdated({ entityId: resultEntityId, patch: { result } });
    get().tlCompleted({ entityId: resultEntityId, result });
  },

  ws: {
    connected: false,
    url: '',
    instance: null,
    reconnectAttempts: 0,
  },
  wsConnect: (convId)=>{
    console.debug('wsConnect', { convId });
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
        console.debug('ws.onmessage', ev.data);
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
    console.debug('wsDisconnect');
    const inst = get().ws.instance;
    if (inst) { try { inst.close(); } catch(_){} }
    set((s)=>({ ws: { ...s.ws, instance: null, connected: false } }), false, { type: 'ws/disconnect' });
  },
  wsOnOpen: ()=>{
    console.debug('wsOnOpen');
    set((s)=>({ app: { ...s.app, status: 'ws connected' }, ws: { ...s.ws, connected: true } }), false, { type: 'ws/onOpen' });
  },
  wsOnClose: ()=>{
    console.debug('wsOnClose');
    set((s)=>({ app: { ...s.app, status: 'ws closed' }, ws: { ...s.ws, connected: false } }), false, { type: 'ws/onClose' });
  },
  wsOnError: (err)=>{
    console.debug('wsOnError', err);
    set((s)=>({ app: { ...s.app, status: 'ws error' }, ws: { ...s.ws, connected: false } }), false, { type: 'ws/onError', payload: { message: String(err && err.message || err) } });
  },
  wsOnMessage: (payload)=>{
    console.debug('wsOnMessage', payload);
    get().handleIncoming(payload);
  },

  // Normalize and dispatch incoming messages to semantic handlers
  handleIncoming: (payload)=>{
    console.debug('handleIncoming', payload);
    if (!payload) return;
    // TL-wrapped messages: { tl: true, event: {...} }
    if (payload.tl && payload.event) {
      const ev = payload.event;
      switch (ev.type) {
        case 'created':
          get().tlCreated(ev);
          break;
        case 'updated':
          get().tlUpdated(ev);
          break;
        case 'completed':
          get().tlCompleted(ev);
          break;
        case 'deleted':
          get().tlDeleted(ev);
          break;
        default:
          console.debug('handleIncoming: unknown event type', ev.type);
          break;
      }
      return;
    }
    console.debug('handleIncoming: unrecognized payload shape');
  },

  // HTTP chat action
  startChat: async (prompt)=>{
    console.debug('startChat', { prompt });
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


