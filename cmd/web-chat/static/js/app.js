import { create } from 'https://cdn.jsdelivr.net/npm/zustand@5.0.8/+esm';
  const useStore = create((set, get)=>({
    convId: '',
    runId: '',
    ws: null,
    status: 'idle',
    timeline: [],
    setConv(convId){ set({ convId }); },
    setRun(runId){ set({ runId }); },
    addMsg(msg){ set({ timeline: [...get().timeline, msg] }); },
    upsertAssistant(messageId, updater){
      const tl = get().timeline.slice();
      const idx = tl.findIndex(m=>m.id===messageId);
      if (idx === -1) {
        tl.push(Object.assign({ id: messageId, role: 'assistant', text: '', final: false }, updater));
      } else {
        tl[idx] = Object.assign({}, tl[idx], updater);
      }
      set({ timeline: tl });
    },
    setStatus(status){ set({ status }); },
    setWS(ws){ set({ ws }); },
  }));

  function render(){
    const { timeline, status } = useStore.getState();
    console.log('render() called, timeline:', timeline);
    document.getElementById('status').textContent = status;
    const root = document.getElementById('timeline');
    root.innerHTML = '';
    for (const it of timeline) {
      const div = document.createElement('div');
      div.className = `msg ${it.role === 'user' ? 'user' : 'assistant'}`;
      div.textContent = it.text || '';
      root.appendChild(div);
    }
    root.scrollTop = root.scrollHeight;
  }
  useStore.subscribe(render);

  function handleEvent(e){
    console.log('handleEvent received:', e);
    const md = (e.meta || {});
    const type = e.type;
    const messageId = (md && md.message_id) || e.id || '';
    if (e.type === 'user') {
      console.log('Adding user message from WS:', e.text);
      useStore.getState().addMsg({ id: `user-${Date.now()}`, role: 'user', text: e.text || '', final: true });
      return;
    }
    if (!messageId) return;
    if (type === 'partial') {
      console.log('Updating assistant partial:', messageId, e.completion);
      useStore.getState().upsertAssistant(messageId, { text: e.completion || '' });
    } else if (type === 'final') {
      console.log('Finalizing assistant:', messageId, e.text);
      useStore.getState().upsertAssistant(messageId, { text: e.text || '', final: true });
    } else if (type === 'start') {
      console.log('Starting assistant:', messageId);
      useStore.getState().upsertAssistant(messageId, {});
    }
  }

  function connectConv(convId){
    console.log('connectConv called with:', convId);
    const { ws } = useStore.getState();
    if (ws) { try { ws.close(); } catch(_){} }
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const wsUrl = `${proto}://${location.host}/ws?conv_id=${encodeURIComponent(convId)}`;
    console.log('Connecting WebSocket to:', wsUrl);
    const n = new WebSocket(wsUrl);
    useStore.getState().setStatus('connecting ws...');
    n.onopen = ()=> {
      console.log('WebSocket connected');
      useStore.getState().setStatus('ws connected');
    };
    n.onclose = ()=> {
      console.log('WebSocket closed');
      useStore.getState().setStatus('ws closed');
    };
    n.onerror = (err)=> {
      console.log('WebSocket error:', err);
      useStore.getState().setStatus('ws error');
    };
    n.onmessage = (ev)=>{
      console.log('WebSocket message received:', ev.data);
      try { handleEvent(JSON.parse(ev.data)); } catch(e){ console.error('Failed to parse WS message:', e); }
    };
    useStore.getState().setWS(n);
  }

  async function startChat(prompt){
    console.log('startChat called with:', prompt);
    const { convId } = useStore.getState();
    const payload = convId ? { prompt, conv_id: convId } : { prompt };
    console.log('POST /chat payload:', payload);
    const res = await fetch('/chat', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
    const j = await res.json();
    console.log('POST /chat response:', j);
    useStore.getState().setRun(j.run_id || '');
    const newConv = j.conv_id || convId || '';
    if (newConv && newConv !== convId) {
      useStore.getState().setConv(newConv);
      connectConv(newConv);
    }
    // Don't add user message locally - let the server broadcast it via WS to avoid duplicates
    console.log('startChat completed, waiting for server to broadcast user message');
  }

  window.addEventListener('DOMContentLoaded', ()=>{
    // Ensure we have a conversation and connect immediately
    let { convId } = useStore.getState();
    if (!convId) {
      const cid = `conv-${Date.now()}-${Math.random().toString(36).slice(2,8)}`;
      useStore.getState().setConv(cid);
      connectConv(cid);
    } else {
      connectConv(convId);
    }
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


