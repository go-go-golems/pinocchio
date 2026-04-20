import { loadPagePartials, showPage } from "./dom.js";
import { initOverviewPage } from "./pages/overview.js";
import { initPhase1Page } from "./pages/phase1.js";
import { initPhase2Page } from "./pages/phase2.js";
import { initPhase3Page } from "./pages/phase3.js";
import { initPhase4Page } from "./pages/phase4.js";
import { initPhase5Page } from "./pages/phase5.js";

async function main() {
  await loadPagePartials();
  bindNavigation();
  await initOverviewPage();
  await initPhase1Page();
  await initPhase2Page();
  await initPhase3Page();
  await initPhase4Page();
  await initPhase5Page();
  showPage(initialPage());
}

function bindNavigation() {
  document.querySelectorAll("[data-page-target]").forEach((button) => {
    button.addEventListener("click", () => {
      const page = button.dataset.pageTarget;
      window.location.hash = page;
      showPage(page);
    });
  });
}

function initialPage() {
  const hash = window.location.hash.replace(/^#/, "");
  return hash || "overview";
}

main().catch((error) => {
  console.error("bootstrap systemlab UI:", error);
});
