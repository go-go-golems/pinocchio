import { fetchChapterHTML, fetchPhase5State, resetPhase5, runPhase5 } from "../api.js";
import { byId, renderChecks, setHTML, setJSON } from "../dom.js";

const client = { socket: null, frames: [] };

export async function initPhase5Page() {
  const chapter = byId("phase5-chapter");
  if (chapter) {
    try {
      setHTML(chapter, await fetchChapterHTML("phase-5-persistence-and-restart"));
    } catch (error) {
      chapter.textContent = error.message;
    }
  }

  document.querySelector('[data-action="phase5-connect"]')?.addEventListener("click", connectClient);
  document.querySelector('[data-action="phase5-subscribe"]')?.addEventListener("click", subscribeClient);
  document.querySelector('[data-action="phase5-seed"]')?.addEventListener("click", async () => {
    await runAction({ action: "seed-session", mode: mode(), sessionId: sessionId(), text: textValue() });
  });
  document.querySelector('[data-action="phase5-restart"]')?.addEventListener("click", async () => {
    await runAction({ action: "restart-backend", mode: mode(), sessionId: sessionId(), text: textValue() });
  });
  document.querySelector('[data-action="phase5-reconnect"]')?.addEventListener("click", () => {
    disconnectClient();
    connectClient();
  });
  document.querySelector('[data-action="phase5-refresh"]')?.addEventListener("click", refreshState);
  document.querySelector('[data-action="phase5-reset"]')?.addEventListener("click", async () => {
    disconnectClient();
    await resetPhase5(mode());
    await refreshState();
  });

  await refreshState();
}

async function runAction(input) {
  const traceOutput = byId("phase5-trace-output");
  const stateOutput = byId("phase5-state-output");
  const checksOutput = byId("phase5-checks");
  try {
    const data = await runPhase5(input);
    setJSON(traceOutput, data.trace || data);
    setJSON(stateOutput, { preRestart: data.preRestart, postRestart: data.postRestart, connections: data.connections });
    renderChecks(checksOutput, data.checks);
  } catch (error) {
    setJSON(traceOutput, { error: error.message });
    setJSON(stateOutput, { error: error.message });
    renderChecks(checksOutput, {});
  }
}

async function refreshState() {
  const traceOutput = byId("phase5-trace-output");
  const stateOutput = byId("phase5-state-output");
  const checksOutput = byId("phase5-checks");
  try {
    const data = await fetchPhase5State(mode(), sessionId(), textValue());
    setJSON(traceOutput, data.trace || data);
    setJSON(stateOutput, { preRestart: data.preRestart, postRestart: data.postRestart, connections: data.connections });
    renderChecks(checksOutput, data.checks);
  } catch (error) {
    setJSON(traceOutput, { error: error.message });
    setJSON(stateOutput, { error: error.message });
    renderChecks(checksOutput, {});
  }
}

function connectClient() {
  disconnectClient();
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  client.frames = [];
  client.socket = new WebSocket(`${protocol}//${window.location.host}/api/phase5/ws`);
  client.socket.onopen = () => renderClient({ status: "open", frames: client.frames });
  client.socket.onmessage = (event) => {
    try {
      client.frames.push(JSON.parse(event.data));
    } catch {
      client.frames.push({ type: "raw", payload: event.data });
    }
    renderClient({ status: readyState(client.socket), frames: client.frames });
    void refreshState();
  };
  client.socket.onclose = () => renderClient({ status: "closed", frames: client.frames });
  client.socket.onerror = () => renderClient({ status: "error", frames: client.frames });
}

function subscribeClient() {
  if (!client.socket || client.socket.readyState !== WebSocket.OPEN) {
    renderClient({ error: "connect first", frames: client.frames });
    return;
  }
  client.socket.send(JSON.stringify({ type: "subscribe", sessionId: sessionId(), sinceOrdinal: "0" }));
}

function disconnectClient() {
  if (client.socket) {
    client.socket.close();
    client.socket = null;
  }
  renderClient({ status: "closed", frames: client.frames });
}

function renderClient(value) {
  setJSON(byId("phase5-client-output"), value);
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

function mode() {
  return byId("phase5-mode")?.value || "memory";
}

function sessionId() {
  return byId("phase5-session-id")?.value || "persist-demo";
}

function textValue() {
  return byId("phase5-text")?.value || "persist this record";
}
