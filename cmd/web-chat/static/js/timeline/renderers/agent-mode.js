// Agent Mode renderer - similar to agent_mode_model.go

import { EntityRenderer, RendererUtils } from './base.js';

export class AgentModeRenderer extends EntityRenderer {
  constructor() {
    super('agent_mode');
  }
  
  render(entity) {
    const { props } = entity;
    const { title = 'Agent Mode', from, to, analysis } = props;
    
    const div = RendererUtils.createElement('div', 'agent-mode');
    
    // Header with mode transition
    let header = title;
    if (from || to) {
      const fromStr = (from || '').trim();
      const toStr = (to || '').trim();
      if (fromStr || toStr) {
        header += ` — ${fromStr} → ${toStr}`;
      }
    }
    
    const headerDiv = RendererUtils.createElement('div', 'agent-mode-header');
    const icon = RendererUtils.createElement('span', 'mode-icon', '🤖');
    const text = RendererUtils.createElement('span', '', ` ${header}`);
    headerDiv.appendChild(icon);
    headerDiv.appendChild(text);
    div.appendChild(headerDiv);
    
    // Analysis details (collapsible)
    if (analysis && analysis.trim()) {
      const analysisContent = RendererUtils.createElement('div', 'analysis-content', analysis);
      const details = RendererUtils.createDetails('Analysis', analysisContent, false);
      div.appendChild(details);
    }
    
    return div;
  }

  update(element, entity, patch) {
    // Agent mode entities are typically static after creation
    // But we could update analysis or transition info if needed
    if (patch.analysis !== undefined) {
      const details = element.querySelector('details');
      if (details) {
        const content = details.querySelector('.analysis-content');
        if (content) {
          content.textContent = patch.analysis;
        }
      }
    }
    
    return element;
  }
}
