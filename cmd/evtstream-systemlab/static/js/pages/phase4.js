import { fetchChapterHTML } from "../api.js";
import { byId, setHTML } from "../dom.js";

export async function initPhase4Page() {
  const chapter = byId("phase4-chapter");
  if (!chapter) {
    return;
  }
  try {
    setHTML(chapter, await fetchChapterHTML("phase-4-chat-example"));
  } catch (error) {
    chapter.textContent = error.message;
  }
}
