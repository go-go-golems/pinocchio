import { fetchChapterHTML, fetchPhase3State, resetPhase3, runPhase3 } from "../api.js";
import { byId, renderChecks, setHTML, setJSON } from "../dom.js";

const clients = {
  a: makeClientState(),
  b: makeClientState(),
};

export async function initPhase3Page() {
  const chapter = byId("phase3-chapter");
  if (chapter) {
    try {
      setHTML(chapter, await fetchChapterHTML("phase-3-hydration-and-reconnect"));
    } catch (error) {
      chapter.textContent = error.message;
    }
  }

  byId("phase3-client-a-output").textContent = "Client A idle.";
  byId("phase3-client-b-output").textContent = "Client B idle.";

  document.querySelector('[data-action="phase3-connect-a"]')?.addEventListener("click", () => connectClient("a"));
  document.querySelector('[data-action="phase3-subscribe-a"]')?.addEventListener("click", () => subscribeClient("a"));
  document.querySelector('[data-action="phase3-disconnect-a"]')?.addEventListener("click", () => disconnectClient("a"));
  document.querySelector('[data-action="phase3-connect-b"]')?.addEventListener("click", () => connectClient("b"));
  document.querySelector('[data-action="phase3-subscribe-b"]')?.addEventListener("click", () => subscribeClient("b"));
  document.querySelector('[data-action="phase3-disconnect-b"]')?.addEventListener("click", () => disconnectClient("b"));
  document.querySelector('[data-action="phase3-seed"]')?.addEventListener("click", async () => {
    await runAction({ action: "seed-session", sessionId: sessionId(), prompt: prompt() });
  });
  document.querySelector('[data-action="phase3-refresh"]')?.addEventListener("click", async () => {
    await refreshState();
  });
  document.querySelector('[data-action="phase3-reset"]')?.addEventListener("click", async () => {
    disconnectClient("a");
    disconnectClient("b");
    await resetPhase3();
    await refreshState();
  });

  await refreshState();
}

async function runAction(input) {
  const traceOutput = byId("phase3-trace-output");
  const stateOutput = byId("phase3-state-output");
  const checksOutput = byId("phase3-checks");
  try {
    const data = await runPhase3(input);
    setJSON(traceOutput, data.trace || data);
    setJSON(stateOutput, { connections: data.connections, snapshot: data.snapshot });
    renderChecks(checksOutput, data.checks);
  } catch (error) {
    setJSON(traceOutput, { error: error.message });
    setJSON(stateOutput, { error: error.message });
    renderChecks(checksOutput, {});
  }
}

async function refreshState() {
  const traceOutput = byId("phase3-trace-output");
  const stateOutput = byId("phase3-state-output");
  const checksOutput = byId("phase3-checks");
  try {
    const data = await fetchPhase3State(sessionId(), prompt());
    setJSON(traceOutput, data.trace || data);
    setJSON(stateOutput, { connections: data.connections, snapshot: data.snapshot });
    renderChecks(checksOutput, data.checks);
  } catch (error) {
    setJSON(traceOutput, { error: error.message });
    setJSON(stateOutput, { error: error.message });
    renderChecks(checksOutput, {});
  }
}

function connectClient(name) {
  const client = clients[name];
  disconnectClient(name);
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  client.frames = [];
  client.socket = new WebSocket(`${protocol}//${window.location.host}/api/phase3/ws`);
  client.socket.onopen = () => renderClient(name, { status: "open", frames: client.frames });
  client.socket.onmessage = (event) => {
    try {
      client.frames.push(JSON.parse(event.data));
    } catch {
      client.frames.push({ type: "raw", payload: event.data });
    }
    renderClient(name, { status: readyState(client.socket), frames: client.frames });
    void refreshState();
  };
  client.socket.onclose = () => renderClient(name, { status: "closed", frames: client.frames });
  client.socket.onerror = () => renderClient(name, { status: "error", frames: client.frames });
}

function subscribeClient(name) {
  const client = clients[name];
  if (!client.socket || client.socket.readyState !== WebSocket.OPEN) {
    renderClient(name, { error: "connect first", frames: client.frames });
    return;
  }
  const payload = {
    type: "subscribe",
    sessionId: sessionId(),
    sinceOrdinal: byId(`phase3-client-${name}-since`)?.value || "0",
  };
  client.socket.send(JSON.stringify(payload));
}

function disconnectClient(name) {
  const client = clients[name];
  if (client.socket) {
    client.socket.close();
    client.socket = null;
  }
  renderClient(name, { status: "closed", frames: client.frames });
}

function renderClient(name, value) {
  setJSON(byId(`phase3-client-${name}-output`), value);
}

function makeClientState() {
  return { socket: null, frames: [] };
}

function readyState(socket) {
  switch (socket?.readyState) {
    case WebSocket.CONNECTING:
      return "connecting";
    case WebSocket.OPEN:
      return "open";
    case WebSocket.CLOSING:
      return "closing";
    case WebSocket.CLOSED:
      return "closed";
    default:
      return "idle";
  }
}

function sessionId() {
  return byId("phase3-session-id")?.value || "reconnect-demo";
}

function prompt() {
  return byId("phase3-prompt")?.value || "watch reconnect preserve a coherent snapshot";
}
