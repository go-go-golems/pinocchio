export function byId(id) {
  return document.getElementById(id);
}

export function queryAll(selector) {
  return Array.from(document.querySelectorAll(selector));
}

export function setJSON(element, value) {
  element.textContent = JSON.stringify(value, null, 2);
}

export function setHTML(element, value) {
  element.innerHTML = value;
}

export function renderChecks(container, checks) {
  container.innerHTML = "";
  const entries = Object.entries(checks || {});
  if (!entries.length) {
    container.textContent = "No checks.";
    return;
  }

  for (const [name, ok] of entries) {
    const badge = document.createElement("span");
    badge.className = "badge";
    badge.textContent = `${ok ? "✓" : "✗"} ${name}`;
    container.appendChild(badge);
  }
}

export function showPage(name) {
  queryAll(".page").forEach((page) => {
    page.classList.toggle("is-hidden", page.id !== `page-${name}`);
  });
}

export async function loadPagePartials() {
  const pages = queryAll(".page[data-partial]");
  await Promise.all(
    pages.map(async (page) => {
      const partialURL = page.dataset.partial;
      const resp = await fetch(partialURL);
      if (!resp.ok) {
        throw new Error(`load partial ${partialURL}: ${resp.status}`);
      }
      page.innerHTML = await resp.text();
    }),
  );
}
