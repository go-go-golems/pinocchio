import { fetchChapterHTML, phase1ExportURL, resetLab, runPhase1 } from "../api.js";
import { byId, renderChecks, setHTML, setJSON } from "../dom.js";

export async function initPhase1Page() {
  const chapter = byId("phase1-chapter");
  if (chapter) {
    try {
      setHTML(chapter, await fetchChapterHTML("phase-1-command-to-projection"));
    } catch (error) {
      chapter.textContent = error.message;
    }
  }
  const sessionInput = byId("session-id");
  const promptInput = byId("prompt");
  const traceOutput = byId("trace-output");
  const sessionOutput = byId("session-output");
  const snapshotOutput = byId("snapshot-output");
  const checksOutput = byId("checks");

  document.querySelector('[data-action="run-phase1"]')?.addEventListener("click", async () => {
    try {
      const data = await runPhase1({
        sessionId: sessionInput?.value,
        prompt: promptInput?.value,
        commandName: "LabStart",
      });
      setJSON(traceOutput, data.trace || data);
      setJSON(sessionOutput, { session: data.session, uiEvents: data.uiEvents });
      setJSON(snapshotOutput, data.snapshot || data);
      renderChecks(checksOutput, data.checks);
    } catch (error) {
      setJSON(traceOutput, { error: error.message });
      setJSON(sessionOutput, { error: error.message });
      setJSON(snapshotOutput, { error: error.message });
      renderChecks(checksOutput, {});
    }
  });

  document.querySelector('[data-action="reset-lab"]')?.addEventListener("click", async () => {
    await resetLab();
    traceOutput.textContent = "Lab reset.";
    sessionOutput.textContent = "Lab reset.";
    snapshotOutput.textContent = "Lab reset.";
    checksOutput.innerHTML = "";
  });

  document.querySelector('[data-action="export-phase1-json"]')?.addEventListener("click", () => {
    openExport(sessionInput?.value, "json");
  });

  document.querySelector('[data-action="export-phase1-markdown"]')?.addEventListener("click", () => {
    openExport(sessionInput?.value, "markdown");
  });
}

function openExport(sessionId, format) {
  window.open(phase1ExportURL(sessionId, format), "_blank");
}
