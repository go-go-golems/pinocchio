import { fetchChapterHTML } from "../api.js";
import { byId, setHTML } from "../dom.js";

export async function initPhase5Page() {
  const chapter = byId("phase5-chapter");
  if (!chapter) {
    return;
  }
  try {
    setHTML(chapter, await fetchChapterHTML("phase-5-persistence-and-restart"));
  } catch (error) {
    chapter.textContent = error.message;
  }
}
