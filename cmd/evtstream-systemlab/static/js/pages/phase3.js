import { fetchChapterHTML } from "../api.js";
import { byId, setHTML } from "../dom.js";

export async function initPhase3Page() {
  const chapter = byId("phase3-chapter");
  if (!chapter) {
    return;
  }
  try {
    setHTML(chapter, await fetchChapterHTML("phase-3-hydration-and-reconnect"));
  } catch (error) {
    chapter.textContent = error.message;
  }
}
