import { h, render } from 'https://esm.sh/preact@10.22.0';
import htm from 'https://esm.sh/htm@3.1.1';
import { TimelineStore } from './timeline/store.js';
import { Timeline } from './timeline/components.js';
import { create } from "https://esm.sh/zustand@4.5.2";
import { devtools } from "https://esm.sh/zustand@4.5.2/middleware";

const html = htm.bind(h);

const state = {
  convId: '',
  runId: '',
  ws: null,
  status: 'idle',
  timelineStore: new TimelineStore(),
};

// Expose app state to Redux DevTools via Zustand
const useDevStore = create(devtools((set)=>({
  convId: '',
  runId: '',
  status: 'idle',
  wsConnected: false,
  timelineCounts: { total: 0, completed: 0, byKind: {} },
  setConvId: (v)=> set({ convId: v }, false, 'setConvId'),
  setRunId: (v)=> set({ runId: v }, false, 'setRunId'),
  setStatus: (v)=> set({ status: v }, false, 'setStatus'),
  setWsConnected: (v)=> set({ wsConnected: v }, false, 'setWsConnected'),
  setTimelineCounts: (v)=> set({ timelineCounts: v }, false, 'setTimelineCounts'),
}), { name: 'web-chat' }));

function mount() {
  const container = document.getElementById('timeline');
  const rerender = () => {
    document.getElementById('status').textContent = state.status;
    const entities = state.timelineStore.getOrderedEntities();
    render(html`<${Timeline} entities=${entities} />`, container);
    container.scrollTop = container.scrollHeight;
    try { useDevStore.getState().setTimelineCounts(state.timelineStore.getStats()); } catch(_){}
  };
  state.timelineStore.subscribe(rerender);
  rerender();
}

function handleTimelineEvent(payload) {
  // TL-wrapped messages: { tl: true, event: {...} }
  if (!payload || !payload.tl || !payload.event) return;
  const ev = payload.event;
  console.debug('Timeline event:', ev);
  state.timelineStore.applyEvent(ev);
}

function handleEvent(e){
  // legacy non-timeline messages (user echo)
  if (e && e.type === 'user') {
    const id = `user-${Date.now()}`;
    state.timelineStore.applyEvent({ type: 'created', entityId: id, kind: 'llm_text', renderer: { kind: 'llm_text' }, props: { role: 'user', text: e.text || '', streaming: false }, startedAt: Date.now() });
    state.timelineStore.applyEvent({ type: 'completed', entityId: id, result: { text: e.text || '' } });
    return;
  }
  // new TL messages
  handleTimelineEvent(e);
}

  function connectConv(convId){
    console.log('connectConv called with:', convId);
    if (state.ws) { try { state.ws.close(); } catch(_){} }
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const wsUrl = `${proto}://${location.host}/ws?conv_id=${encodeURIComponent(convId)}`;
    console.log('Connecting WebSocket to:', wsUrl);
    const n = new WebSocket(wsUrl);
    state.status = 'connecting ws...';
    try { useDevStore.getState().setStatus(state.status); } catch(_){}
    n.onopen = ()=> {
      console.log('WebSocket connected');
      state.status = 'ws connected';
      document.getElementById('status').textContent = state.status;
      try { useDevStore.getState().setStatus(state.status); useDevStore.getState().setWsConnected(true); } catch(_){}
    };
    n.onclose = ()=> {
      console.log('WebSocket closed');
      state.status = 'ws closed';
      document.getElementById('status').textContent = state.status;
      try { useDevStore.getState().setStatus(state.status); useDevStore.getState().setWsConnected(false); } catch(_){}
    };
    n.onerror = (err)=> {
      console.log('WebSocket error:', err);
      state.status = 'ws error';
      document.getElementById('status').textContent = state.status;
      try { useDevStore.getState().setStatus(state.status); useDevStore.getState().setWsConnected(false); } catch(_){}
    };
    n.onmessage = (ev)=>{
      console.log('WebSocket message received:', ev.data);
      try { handleEvent(JSON.parse(ev.data)); } catch(e){ console.error('Failed to parse WS message:', e); }
    };
    state.ws = n;
  }

  async function startChat(prompt){
    console.log('startChat called with:', prompt);
    const payload = state.convId ? { prompt, conv_id: state.convId } : { prompt };
    console.log('POST /chat payload:', payload);
    const res = await fetch('/chat', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
    const j = await res.json();
    console.log('POST /chat response:', j);
    state.runId = j.run_id || '';
    try { useDevStore.getState().setRunId(state.runId); } catch(_){}
    const newConv = j.conv_id || state.convId || '';
    if (newConv && newConv !== state.convId) {
      state.convId = newConv;
      try { useDevStore.getState().setConvId(state.convId); } catch(_){}
      connectConv(newConv);
    }
    // Don't add user message locally - let the server broadcast it via WS to avoid duplicates
    console.log('startChat completed, waiting for server to broadcast user message');
  }

  window.addEventListener('DOMContentLoaded', ()=>{
    mount();
    // Ensure we have a conversation and connect immediately
    if (!state.convId) {
      const cid = `conv-${Date.now()}-${Math.random().toString(36).slice(2,8)}`;
      state.convId = cid;
      try { useDevStore.getState().setConvId(state.convId); } catch(_){}
      connectConv(cid);
    } else {
      connectConv(state.convId);
    }
    // Listen for append-to-prompt events from tool result widgets
    document.addEventListener('append-to-prompt', (e)=>{
      try {
        const text = (e && e.detail && e.detail.text) ? String(e.detail.text) : '';
        const input = document.getElementById('prompt');
        if (input) {
          const sep = input.value && text ? ' ' : '';
          input.value = `${input.value}${sep}${text}`;
          input.focus();
        }
      } catch(err) {
        console.error('append-to-prompt handler error', err);
      }
    });
    const input = document.getElementById('prompt');
    const send = ()=>{
      const v = (input.value || '').trim();
      if (!v) return;
      input.value = '';
      startChat(v);
    };
    document.getElementById('send-btn').addEventListener('click', send);
    input.addEventListener('keydown', (e)=>{
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        send();
      }
    });
  });


