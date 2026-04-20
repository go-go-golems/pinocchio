import { loadPagePartials, showPage } from "./dom.js";
import { initOverviewPage } from "./pages/overview.js";
import { initPhase1Page } from "./pages/phase1.js";
import { initPhase2Page } from "./pages/phase2.js";

async function main() {
  await loadPagePartials();
  bindNavigation();
  await initOverviewPage();
  await initPhase1Page();
  initPhase2Page();
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
