import { h, render } from 'https://esm.sh/preact@10.22.0';
import htm from 'https://esm.sh/htm@3.1.1';
import { Timeline } from './timeline/components.js';
import { useStore } from './store.js';

const html = htm.bind(h);

const state = {};

// Unified Zustand store is provided by ./store.js

function mount() {
  const container = document.getElementById('timeline');
  const rerender = () => {
    try { document.getElementById('status').textContent = useStore.getState().app.status; } catch(_){ }
    const s = useStore.getState();
    const entities = s.timeline.order.map((id)=> s.timeline.byId[id]).filter(Boolean);
    console.debug('render: timeline entities count', entities.length, 'orderLen', s.timeline.order.length, 'byIdKeys', Object.keys(s.timeline.byId).length);
    try {
      render(html`<${Timeline} entities=${entities} />`, container);
    } catch(err) {
      console.error('render error', err);
    }
    container.scrollTop = container.scrollHeight;
  };
  try {
    // Subscribe to full timeline object changes
    const unsub1 = useStore.subscribe((s)=> s.timeline, (_)=>{ console.debug('sub: timeline changed'); rerender(); });
    // Also subscribe to order length only (lightweight)
    const unsub2 = useStore.subscribe((s)=> s.timeline.order.length, (len)=>{ console.debug('sub: order length', len); rerender(); });
    // Keep a reference to unsubscribes if needed later
    state.unsubTimeline = ()=>{ try { unsub1(); unsub2(); } catch(_){} };
  } catch(_){ }
  rerender();
}

// No bridge needed; the store applies timeline events directly

  /* Connection and chat moved into store: useStore.getState().wsConnect, .startChat */

  window.addEventListener('DOMContentLoaded', ()=>{
    console.debug('DOMContentLoaded');
    try { window.__store = useStore; } catch(_){ }
    mount();
    // Keep status element in sync with store
    try {
      document.getElementById('status').textContent = useStore.getState().app.status;
      useStore.subscribe((s)=> s.app.status, (status)=>{
        console.debug('status changed', status);
        try { document.getElementById('status').textContent = status; } catch(_){}
      });
    } catch(_){ }
    // Ensure we have a conversation and connect immediately (store-driven)
    const st = useStore.getState();
    if (!st.app.convId) {
      const cid = `conv-${Date.now()}-${Math.random().toString(36).slice(2,8)}`;
      console.debug('init convId', cid);
      useStore.getState().setConvId(cid);
      useStore.getState().wsConnect(cid);
    } else {
      console.debug('reuse convId', st.app.convId);
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
      console.debug('send prompt', v);
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


