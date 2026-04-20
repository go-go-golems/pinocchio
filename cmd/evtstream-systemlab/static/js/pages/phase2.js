import { fetchChapterHTML, phase2ExportURL, runPhase2 } from "../api.js";
import { byId, renderChecks, setHTML, setJSON } from "../dom.js";

export async function initPhase2Page() {
  const chapter = byId("phase2-chapter");
  if (chapter) {
    try {
      setHTML(chapter, await fetchChapterHTML("phase-2-ordering-and-ordinals"));
    } catch (error) {
      chapter.textContent = error.message;
    }
  }
  const sessionAInput = byId("phase2-session-a");
  const sessionBInput = byId("phase2-session-b");
  const burstCountInput = byId("phase2-burst-count");
  const streamModeInput = byId("phase2-stream-mode");
  const traceOutput = byId("phase2-trace-output");
  const messagesOutput = byId("phase2-messages-output");
  const ordinalsOutput = byId("phase2-ordinals-output");
  const snapshotsOutput = byId("phase2-snapshots-output");
  const checksOutput = byId("phase2-checks");

  bindAction('[data-action="phase2-publish-a"]', () => submitAction("publish-a"));
  bindAction('[data-action="phase2-publish-b"]', () => submitAction("publish-b"));
  bindAction('[data-action="phase2-burst-a"]', () => submitAction("burst-a"));
  bindAction('[data-action="phase2-restart-consumer"]', () => submitAction("restart-consumer"));
  bindAction('[data-action="phase2-reset"]', () => submitAction("reset-phase2"));
  bindAction('[data-action="phase2-export-json"]', () => window.open(phase2ExportURL("json"), "_blank"));
  bindAction('[data-action="phase2-export-markdown"]', () => window.open(phase2ExportURL("markdown"), "_blank"));

  async function submitAction(action) {
    try {
      const data = await runPhase2({
        action,
        sessionA: sessionAInput?.value,
        sessionB: sessionBInput?.value,
        burstCount: Number.parseInt(burstCountInput?.value || "4", 10),
        streamMode: streamModeInput?.value,
      });
      setJSON(traceOutput, data.trace || data);
      setJSON(messagesOutput, data.messageHistory || data);
      setJSON(ordinalsOutput, data.perSessionOrdinals || data);
      setJSON(snapshotsOutput, data.snapshots || data);
      renderChecks(checksOutput, data.checks);
    } catch (error) {
      const value = { error: error.message };
      setJSON(traceOutput, value);
      setJSON(messagesOutput, value);
      setJSON(ordinalsOutput, value);
      setJSON(snapshotsOutput, value);
      renderChecks(checksOutput, {});
    }
  }
}

function bindAction(selector, handler) {
  document.querySelector(selector)?.addEventListener("click", handler);
}
