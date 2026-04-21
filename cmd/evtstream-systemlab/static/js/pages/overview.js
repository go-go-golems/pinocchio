import { fetchChapterHTML, fetchStatus } from "../api.js";
import { byId, setHTML } from "../dom.js";

export async function initOverviewPage() {
  const chapter = byId("phase0-chapter");
  if (chapter) {
    try {
      setHTML(chapter, await fetchChapterHTML("phase-0-foundations"));
    } catch (error) {
      chapter.textContent = error.message;
    }
  }

  const statusContainer = byId("phase0-status");
  const output = byId("status-output");
  if (!output) {
    return;
  }
  
  const data = await fetchStatus();
  
  // Build StatusIndicator widget
  if (statusContainer) {
    const phases = data.phases || [];
    const labs = data.labs || [];
    
    let statusHTML = '<div class="phase-progress">';
    
    phases.forEach((phaseId, index) => {
      const lab = labs.find(l => l.id === phaseId);
      const isImplemented = lab && lab.implemented;
      const hasChapter = lab && lab.chapter;
      
      // Icon: ● for implemented, ○ for not
      const statusIcon = isImplemented ? '●' : '○';
      
      // Class: active for current phase (phase0), complete for implemented chapters, pending for not
      const isCurrentPhase = phaseId === 'phase0';
      const statusClass = isCurrentPhase ? 'status-active' : (isImplemented ? 'status-complete' : 'status-pending');
      
      // Phase number: index (0-based) matches phase number
      const phaseNum = index;
      const phaseName = getPhaseName(phaseId, index);
      
      statusHTML += `
        <div class="phase-item ${statusClass}">
          <span class="phase-icon">${statusIcon}</span>
          <span class="phase-name">Phase ${phaseNum}: ${phaseName}</span>
        </div>
      `;
    });
    
    statusHTML += '</div>';
    statusContainer.innerHTML = statusHTML;
  }
  
  // Also show raw JSON for debugging
  const outputData = {
    app: data.app,
    boundary: data.boundary,
    phases: data.phases,
    labs: data.labs
  };
  output.textContent = JSON.stringify(outputData, null, 2);
}

function getPhaseName(phaseId, index) {
  const names = [
    "Foundations",
    "Command → Event → Projection",
    "Ordering and Ordinals",
    "Hydration and Reconnect",
    "Chat Example",
    "SQL / Restart",
    "Migration / Regression"
  ];
  return names[index] || phaseId;
}