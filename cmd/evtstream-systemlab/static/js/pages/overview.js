import { fetchChapterHTML, fetchStatus } from "../api.js";
import { byId, setHTML, setJSON } from "../dom.js";

export async function initOverviewPage() {
  const chapter = byId("phase0-chapter");
  if (chapter) {
    try {
      setHTML(chapter, await fetchChapterHTML("phase-0-foundations"));
    } catch (error) {
      chapter.textContent = error.message;
    }
  }

  const output = byId("status-output");
  if (!output) {
    return;
  }
  const data = await fetchStatus();
  setJSON(output, data);
}
