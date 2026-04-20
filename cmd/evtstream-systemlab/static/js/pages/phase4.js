import { fetchChapterHTML, fetchPhase4State, resetPhase4, runPhase4 } from "../api.js";
import { byId, renderChecks, setHTML, setJSON } from "../dom.js";

const client = { socket: null, frames: [] };

export async function initPhase4Page() {
  const chapter = byId("phase4-chapter");
  if (chapter) {
    try {
      setHTML(chapter, await fetchChapterHTML("phase-4-chat-example"));
    } catch (error) {
      chapter.textContent = error.message;
    }
  }

  document.querySelector('[data-action="phase4-connect"]')?.addEventListener("click", connectClient);
  document.querySelector('[data-action="phase4-subscribe"]')?.addEventListener("click", subscribeClient);
  document.querySelector('[data-action="phase4-send"]')?.addEventListener("click", async () => {
    await runAction({ action: "send", sessionId: sessionId(), prompt: prompt() });
  });
  document.querySelector('[data-action="phase4-stop"]')?.addEventListener("click", async () => {
    await runAction({ action: "stop", sessionId: sessionId(), prompt: prompt() });
  });
  document.querySelector('[data-action="phase4-refresh"]')?.addEventListener("click", refreshState);
  document.querySelector('[data-action="phase4-reset"]')?.addEventListener("click", async () => {
    disconnectClient();
    await resetPhase4();
    await refreshState();
  });

  await refreshState();
}

async function runAction(input) {
  const traceOutput = byId("phase4-trace-output");
  const stateOutput = byId("phase4-state-output");
  const checksOutput = byId("phase4-checks");
  try {
    const data = await runPhase4(input);
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
  const traceOutput = byId("phase4-trace-output");
  const stateOutput = byId("phase4-state-output");
  const checksOutput = byId("phase4-checks");
  try {
    const data = await fetchPhase4State(sessionId(), prompt());
    setJSON(traceOutput, data.trace || data);
    setJSON(stateOutput, { connections: data.connections, snapshot: data.snapshot });
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
  client.socket = new WebSocket(`${protocol}//${window.location.host}/api/phase4/ws`);
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
  setJSON(byId("phase4-client-output"), value);
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
  return byId("phase4-session-id")?.value || "chat-demo";
}

function prompt() {
  return byId("phase4-prompt")?.value || "Explain ordinals in plain language";
}
