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
