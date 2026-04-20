export async function fetchStatus() {
  const resp = await fetch("/api/status");
  return parseJSON(resp, "load status");
}

export async function fetchChapterHTML(name) {
  const resp = await fetch(`/api/chapters/${encodeURIComponent(name)}`);
  return parseText(resp, `load chapter ${name}`);
}

export async function runPhase1(input) {
  const resp = await fetch("/api/phase1/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return parseJSON(resp, "run phase 1 lab");
}

export async function resetLab() {
  const resp = await fetch("/api/reset", { method: "POST" });
  return parseJSON(resp, "reset lab");
}

export async function runPhase2(input) {
  const resp = await fetch("/api/phase2/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return parseJSON(resp, "run phase 2 lab");
}

export function phase1ExportURL(sessionId, format) {
  const sid = encodeURIComponent(sessionId || "lab-session-1");
  return `/api/phase1/export?sessionId=${sid}&format=${encodeURIComponent(format)}`;
}

export function phase2ExportURL(format) {
  return `/api/phase2/export?format=${encodeURIComponent(format)}`;
}

export async function runPhase3(input) {
  const resp = await fetch("/api/phase3/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return parseJSON(resp, "run phase 3 lab");
}

export async function fetchPhase3State(sessionId, prompt) {
  const params = new URLSearchParams();
  if (sessionId) params.set("sessionId", sessionId);
  if (prompt) params.set("prompt", prompt);
  const resp = await fetch(`/api/phase3/state?${params.toString()}`);
  return parseJSON(resp, "load phase 3 state");
}

export async function resetPhase3() {
  const resp = await fetch("/api/phase3/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: "reset-phase3" }),
  });
  return parseJSON(resp, "reset phase 3 lab");
}

export async function runPhase4(input) {
  const resp = await fetch("/api/phase4/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return parseJSON(resp, "run phase 4 lab");
}

export async function fetchPhase4State(sessionId, prompt) {
  const params = new URLSearchParams();
  if (sessionId) params.set("sessionId", sessionId);
  if (prompt) params.set("prompt", prompt);
  const resp = await fetch(`/api/phase4/state?${params.toString()}`);
  return parseJSON(resp, "load phase 4 state");
}

export async function resetPhase4() {
  const resp = await fetch("/api/phase4/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: "reset-phase4" }),
  });
  return parseJSON(resp, "reset phase 4 lab");
}

export async function runPhase5(input) {
  const resp = await fetch("/api/phase5/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return parseJSON(resp, "run phase 5 lab");
}

export async function fetchPhase5State(mode, sessionId, text) {
  const params = new URLSearchParams();
  if (mode) params.set("mode", mode);
  if (sessionId) params.set("sessionId", sessionId);
  if (text) params.set("text", text);
  const resp = await fetch(`/api/phase5/state?${params.toString()}`);
  return parseJSON(resp, "load phase 5 state");
}

export async function resetPhase5(mode) {
  const resp = await fetch("/api/phase5/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: "reset-phase5", mode }),
  });
  return parseJSON(resp, "reset phase 5 lab");
}

async function parseJSON(resp, action) {
  const data = await resp.json();
  if (!resp.ok) {
    const message = data?.error || `${action} failed with status ${resp.status}`;
    throw new Error(message);
  }
  return data;
}

async function parseText(resp, action) {
  const data = await resp.text();
  if (!resp.ok) {
    throw new Error(`${action} failed with status ${resp.status}`);
  }
  return data;
}
