import { fetchChapterHTML, fetchPhase6State, runPhase6 } from "../api.js";
import { byId, renderChecks, setHTML, setJSON } from "../dom.js";

export async function initPhase6Page() {
  const chapter = byId("phase6-chapter");
  if (chapter) {
    try {
      setHTML(chapter, await fetchChapterHTML("phase-6-webchat-migration"));
    } catch (error) {
      chapter.textContent = error.message;
    }
  }

  document.querySelector('[data-action="phase6-run"]')?.addEventListener("click", async () => {
    await runScenario();
  });
  document.querySelector('[data-action="phase6-refresh"]')?.addEventListener("click", async () => {
    await refreshState();
  });

  await refreshState();
}

async function runScenario() {
  const traceOutput = byId("phase6-trace-output");
  const snapshotOutput = byId("phase6-snapshot-output");
  const checksOutput = byId("phase6-checks");
  const routeOutput = byId("phase6-route-output");
  try {
    const data = await runPhase6({
      action: "run",
      baseUrl: baseURL(),
      profile: profile(),
      prompt: prompt(),
      timeoutSeconds: 45,
    });
    setJSON(traceOutput, data.trace || data);
    setJSON(snapshotOutput, data.snapshot || {});
    setJSON(routeOutput, { routeStatuses: data.routeStatuses, availableProfiles: data.availableProfiles, sessionId: data.sessionId });
    renderChecks(checksOutput, data.checks || {});
  } catch (error) {
    setJSON(traceOutput, { error: error.message });
    setJSON(snapshotOutput, { error: error.message });
    setJSON(routeOutput, { error: error.message });
    renderChecks(checksOutput, {});
  }
}

async function refreshState() {
  const traceOutput = byId("phase6-trace-output");
  const snapshotOutput = byId("phase6-snapshot-output");
  const checksOutput = byId("phase6-checks");
  const routeOutput = byId("phase6-route-output");
  try {
    const data = await fetchPhase6State(baseURL(), profile(), prompt());
    setJSON(traceOutput, data.trace || { status: "idle" });
    setJSON(snapshotOutput, data.snapshot || { status: "idle" });
    setJSON(routeOutput, { routeStatuses: data.routeStatuses || {}, availableProfiles: data.availableProfiles || [], sessionId: data.sessionId || "" });
    renderChecks(checksOutput, data.checks || {});
  } catch (error) {
    setJSON(traceOutput, { error: error.message });
    setJSON(snapshotOutput, { error: error.message });
    setJSON(routeOutput, { error: error.message });
    renderChecks(checksOutput, {});
  }
}

function baseURL() {
  return byId("phase6-base-url")?.value || "http://127.0.0.1:18112";
}

function profile() {
  return byId("phase6-profile")?.value || "gpt-5-nano-low";
}

function prompt() {
  return byId("phase6-prompt")?.value || "In one short sentence, explain ordinals.";
}
