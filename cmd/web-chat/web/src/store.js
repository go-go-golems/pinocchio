import { create } from "https://esm.sh/zustand@4.5.2";
import { devtools } from "https://esm.sh/zustand@4.5.2/middleware";

export const useStore = create(devtools((set, get)=>({
  app: {
    convId: '',
    runId: '',
    status: 'idle',
  },
  setConvId: (v)=> set((s)=>({ app: { ...s.app, convId: v } }), false, 'app/setConvId'),
  setRunId: (v)=> set((s)=>({ app: { ...s.app, runId: v } }), false, 'app/setRunId'),
  setStatus: (v)=> set((s)=>({ app: { ...s.app, status: v } }), false, 'app/setStatus'),

  metrics: {
    timelineCounts: { total: 0, completed: 0, byKind: {} },
  },
  setTimelineCounts: (v)=> set((s)=>({ metrics: { ...s.metrics, timelineCounts: v } }), false, 'metrics/setTimelineCounts'),

  // A callback provided by the UI layer to dispatch timeline events
  handlers: {
    timelineEventHandler: null,
  },
  setTimelineEventHandler: (fn)=> set((s)=>({ handlers: { ...s.handlers, timelineEventHandler: fn } }), false, 'handlers/setTimelineEventHandler'),

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
      set((s)=>({ app: { ...s.app, status: 'connecting ws...' }, ws: { ...s.ws, url, instance: n } }), false, 'ws/connect');
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
    set((s)=>({ ws: { ...s.ws, instance: null, connected: false } }), false, 'ws/disconnect');
  },
  wsOnOpen: ()=>{
    set((s)=>({ app: { ...s.app, status: 'ws connected' }, ws: { ...s.ws, connected: true } }), false, 'ws/onOpen');
  },
  wsOnClose: ()=>{
    set((s)=>({ app: { ...s.app, status: 'ws closed' }, ws: { ...s.ws, connected: false } }), false, 'ws/onClose');
  },
  wsOnError: (_err)=>{
    set((s)=>({ app: { ...s.app, status: 'ws error' }, ws: { ...s.ws, connected: false } }), false, 'ws/onError');
  },
  wsOnMessage: (payload)=>{
    get().handleIncoming(payload);
  },

  // Normalize and dispatch incoming messages to semantic handlers
  handleIncoming: (payload)=>{
    const h = get().handlers.timelineEventHandler;
    if (!payload) return;
    // legacy non-timeline messages (user echo)
    if (payload && payload.type === 'user') {
      if (typeof h === 'function') {
        const id = `user-${Date.now()}`;
        try { h({ type: 'created', entityId: id, kind: 'llm_text', renderer: { kind: 'llm_text' }, props: { role: 'user', text: '', streaming: false }, startedAt: Date.now() }); } catch(_){}
        try { h({ type: 'completed', entityId: id, result: { text: payload.text || '' } }); } catch(_){ }
      }
      return;
    }
    // TL-wrapped messages: { tl: true, event: {...} }
    if (payload.tl && payload.event && typeof h === 'function') {
      try { h(payload.event); } catch(_){ }
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
    set((s)=>({ app: { ...s.app, runId: newRun, convId: newConv } }), false, 'chat/startChat:received');
    if (newConv && newConv !== convId) {
      get().wsConnect(newConv);
    }
  },
}), { name: 'web-chat' }));


