// Log Event renderer

import { EntityRenderer, RendererUtils } from './base.js';

export class LogEventRenderer extends EntityRenderer {
  constructor() {
    super('log_event');
  }
  
  render(entity) {
    const { props } = entity;
    const { level = 'info', message, fields } = props;
    
    const div = RendererUtils.createElement('div', `log-event log-${level}`);
    
    // Log content with level and message
    const content = RendererUtils.createElement('div', 'log-content');
    
    const levelSpan = RendererUtils.createElement('span', 'log-level', `[${level.toUpperCase()}]`);
    const messageSpan = RendererUtils.createElement('span', 'log-message', ` ${message}`);
    
    content.appendChild(levelSpan);
    content.appendChild(messageSpan);
    div.appendChild(content);
    
    // Fields as collapsible details
    if (fields && Object.keys(fields).length > 0) {
      const fieldsContent = RendererUtils.createElement('pre', 'log-fields', RendererUtils.formatJSON(fields));
      const details = RendererUtils.createDetails('Fields', fieldsContent, false);
      div.appendChild(details);
    }
    
    return div;
  }

  update(element, entity, patch) {
    // Log events are typically static
    // But we could update fields if they're dynamic
    if (patch.fields !== undefined) {
      const details = element.querySelector('details');
      if (details) {
        const fieldsContent = details.querySelector('.log-fields');
        if (fieldsContent) {
          fieldsContent.textContent = RendererUtils.formatJSON(patch.fields);
        }
      } else if (patch.fields && Object.keys(patch.fields).length > 0) {
        // Add fields if they didn't exist before
        const fieldsContent = RendererUtils.createElement('pre', 'log-fields', RendererUtils.formatJSON(patch.fields));
        const newDetails = RendererUtils.createDetails('Fields', fieldsContent, false);
        element.appendChild(newDetails);
      }
    }
    
    return element;
  }
}
