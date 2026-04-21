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

  // Store view state
  const viewState = {
    trace: 'rendered',
    session: 'rendered',
    snapshot: 'rendered'
  };

  // Setup toggle buttons
  document.querySelectorAll('.toggle-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const panel = btn.dataset.toggle;
      const view = btn.dataset.view;
      viewState[panel] = view;
      
      // Update active state
      document.querySelectorAll(`.toggle-btn[data-toggle="${panel}"]`).forEach(b => {
        b.classList.toggle('active', b.dataset.view === view);
      });
      
      // Re-render if we have data
      const currentData = traceOutput.dataset.lastResult;
      if (currentData) {
        const data = JSON.parse(currentData);
        if (panel === 'trace') {
          renderTrace(traceOutput, data.trace || data, view);
        } else if (panel === 'session') {
          renderSession(sessionOutput, { session: data.session, uiEvents: data.uiEvents }, view);
        } else if (panel === 'snapshot') {
          renderSnapshot(snapshotOutput, data.snapshot || data, view);
        }
      }
    });
  });

  document.querySelector('[data-action="run-phase1"]')?.addEventListener("click", async () => {
    try {
      const data = await runPhase1({
        sessionId: sessionInput?.value,
        prompt: promptInput?.value,
        commandName: "LabStart",
      });
      
      // Store for re-rendering
      traceOutput.dataset.lastResult = JSON.stringify(data);
      
      // Render in current view modes
      renderTrace(traceOutput, data.trace || data, viewState.trace);
      renderSession(sessionOutput, { session: data.session, uiEvents: data.uiEvents }, viewState.session);
      renderSnapshot(snapshotOutput, data.snapshot || data, viewState.snapshot);
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
    traceOutput.textContent = "No run yet.";
    sessionOutput.textContent = "No run yet.";
    snapshotOutput.textContent = "No run yet.";
    checksOutput.innerHTML = "";
    delete traceOutput.dataset.lastResult;
  });

  document.querySelector('[data-action="export-phase1-json"]')?.addEventListener("click", () => {
    openExport(sessionInput?.value, "json");
  });

  document.querySelector('[data-action="export-phase1-markdown"]')?.addEventListener("click", () => {
    openExport(sessionInput?.value, "markdown");
  });
}

// Render functions for different views
function renderTrace(el, data, view) {
  if (view === 'json') {
    setJSON(el, data);
  } else {
    el.className = 'trace-rendered';
    el.innerHTML = renderTraceRendered(data);
  }
}

function renderTraceRendered(trace) {
  if (!trace || !Array.isArray(trace) || trace.length === 0) {
    return '<span class="empty">No trace yet.</span>';
  }
  
  return trace.map(step => {
    const kindClass = step.kind || 'unknown';
    return `
      <div class="trace-step">
        <span class="trace-step-num">${step.step}</span>
        <span class="trace-step-kind ${kindClass}">${step.kind}</span>
        <span class="trace-step-message">${step.message}</span>
      </div>
    `;
  }).join('');
}

function renderSession(el, data, view) {
  if (view === 'json') {
    setJSON(el, data);
  } else {
    el.className = 'session-rendered';
    el.innerHTML = renderSessionRendered(data);
  }
}

function renderSessionRendered(data) {
  if (!data) return '<span class="empty">No session data.</span>';
  
  const session = data.session || {};
  const uiEvents = data.uiEvents || [];
  
  let html = '<div class="session-header">';
  html += `<div class="session-header-label">Session</div>`;
  html += `<div class="session-header-value">${session.sessionId || 'unknown'}</div>`;
  html += '</div>';
  
  html += '<div class="ui-events-list">';
  uiEvents.forEach(evt => {
    const icon = getEventIcon(evt.name);
    const iconClass = getEventIconClass(evt.name);
    html += `
      <div class="ui-event-item">
        <span class="ui-event-icon ${iconClass}">${icon}</span>
        <div>
          <div class="ui-event-name">${formatEventName(evt.name)}</div>
          <div class="ui-event-detail">${formatEventDetail(evt)}</div>
        </div>
      </div>
    `;
  });
  html += '</div>';
  
  return html;
}

function renderSnapshot(el, data, view) {
  if (view === 'json') {
    setJSON(el, data);
  } else {
    el.className = 'snapshot-rendered';
    el.innerHTML = renderSnapshotRendered(data);
  }
}

function renderSnapshotRendered(data) {
  if (!data) return '<span class="empty">No snapshot yet.</span>';
  
  const entities = data.entities || [];
  const sessionId = data.sessionId || 'unknown';
  const ordinal = data.ordinal || 0;
  
  let html = `<div class="snapshot-session">`;
  html += `<div class="session-header-label">Session: ${sessionId}</div>`;
  html += `<div class="session-header-label">Ordinal: ${ordinal}</div>`;
  html += '</div>';
  
  entities.forEach(entity => {
    const icon = entity.status === 'finished' ? '✓' : '●';
    const iconClass = entity.status === 'finished' ? 'finished' : 'pending';
    html += `
      <div class="snapshot-entity">
        <div class="snapshot-entity-header">
          <span class="snapshot-entity-icon ${iconClass}">${icon}</span>
          <span class="snapshot-entity-name">${entity.kind || entity.id}</span>
        </div>
        <div class="snapshot-entity-props">
    `;
    
    // Add properties
    if (entity.payload) {
      Object.entries(entity.payload).forEach(([key, value]) => {
        html += `<div class="snapshot-prop"><span class="snapshot-prop-label">${key}:</span> <span class="snapshot-prop-value">${value}</span></div>`;
      });
    }
    
    html += '</div></div>';
  });
  
  return html;
}

// Helper functions
function getEventIcon(name) {
  if (name === 'LabMessageStarted') return '●';
  if (name === 'LabMessageAppended') return '→';
  if (name === 'LabMessageFinished') return '✓';
  return '○';
}

function getEventIconClass(name) {
  if (name === 'LabMessageStarted') return 'started';
  if (name === 'LabMessageAppended') return 'appended';
  if (name === 'LabMessageFinished') return 'finished';
  return '';
}

function formatEventName(name) {
  return name.replace(/([A-Z])/g, ' $1').trim();
}

function formatEventDetail(evt) {
  if (evt.payload) {
    if (evt.payload.text) return `"${evt.payload.text}"`;
    if (evt.payload.msgId) return `id: ${evt.payload.msgId}`;
  }
  return '';
}

function openExport(sessionId, format) {
  window.open(phase1ExportURL(sessionId, format), "_blank");
}