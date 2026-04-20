import { fetchStatus } from "../api.js";
import { byId, setJSON } from "../dom.js";

export async function initOverviewPage() {
  const output = byId("status-output");
  if (!output) {
    return;
  }
  const data = await fetchStatus();
  setJSON(output, data);
}
