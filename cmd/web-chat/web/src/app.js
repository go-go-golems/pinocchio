import { h, render } from 'https://esm.sh/preact@10.22.0';
import htm from 'https://esm.sh/htm@3.1.1';
import { TimelineStore } from './timeline/store.js';
import { Timeline } from './timeline/components.js';
import { useStore } from './store.js';

const html = htm.bind(h);

const state = {
  convId: '',
  runId: '',
  ws: null,
  status: 'idle',
  timelineStore: new TimelineStore(),
};

// Unified Zustand store is provided by ./store.js

function mount() {
  const container = document.getElementById('timeline');
  const rerender = () => {
    try { document.getElementById('status').textContent = useStore.getState().app.status; } catch(_){ }
    const entities = state.timelineStore.getOrderedEntities();
    render(html`<${Timeline} entities=${entities} />`, container);
    container.scrollTop = container.scrollHeight;
    try { useStore.getState().setTimelineCounts(state.timelineStore.getStats()); } catch(_){ }
  };
  state.timelineStore.subscribe(rerender);
  rerender();
}

// Bridge: let the store deliver timeline events to our TimelineStore instance
try {
  useStore.getState().setTimelineEventHandler((ev)=>{
    try { state.timelineStore.applyEvent(ev); } catch(err) { console.error('timeline handler error', err); }
  });
} catch(_){ }

  /* Connection and chat moved into store: useStore.getState().wsConnect, .startChat */

  window.addEventListener('DOMContentLoaded', ()=>{
    mount();
    // Keep status element in sync with store
    try {
      document.getElementById('status').textContent = useStore.getState().app.status;
      useStore.subscribe((s)=> s.app.status, (status)=>{
        try { document.getElementById('status').textContent = status; } catch(_){}
      });
    } catch(_){ }
    // Ensure we have a conversation and connect immediately (store-driven)
    const st = useStore.getState();
    if (!st.app.convId) {
      const cid = `conv-${Date.now()}-${Math.random().toString(36).slice(2,8)}`;
      useStore.getState().setConvId(cid);
      useStore.getState().wsConnect(cid);
    } else {
      useStore.getState().wsConnect(st.app.convId);
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
      useStore.getState().startChat(v);
    };
    document.getElementById('send-btn').addEventListener('click', send);
    input.addEventListener('keydown', (e)=>{
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        send();
      }
    });
  });


