export async function fetchStatus() {
  const resp = await fetch("/api/status");
  return parseJSON(resp, "load status");
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

export function phase1ExportURL(sessionId, format) {
  const sid = encodeURIComponent(sessionId || "lab-session-1");
  return `/api/phase1/export?sessionId=${sid}&format=${encodeURIComponent(format)}`;
}

async function parseJSON(resp, action) {
  const data = await resp.json();
  if (!resp.ok) {
    const message = data?.error || `${action} failed with status ${resp.status}`;
    throw new Error(message);
  }
  return data;
}
