// Tool Call renderer

import { EntityRenderer, RendererUtils } from './base.js';

export class ToolCallRenderer extends EntityRenderer {
  constructor() {
    super('tool_call');
  }
  
  render(entity) {
    const { props } = entity;
    const { name, input, exec = false } = props;
    
    const div = RendererUtils.createElement('div', 'tool-call');
    
    // Header with tool name and execution status
    const header = RendererUtils.createElement('div', 'tool-header');
    
    const icon = RendererUtils.createElement('span', 'tool-icon', '🔧');
    const nameSpan = RendererUtils.createElement('span', 'tool-name', name || 'Unknown Tool');
    
    header.appendChild(icon);
    header.appendChild(nameSpan);
    
    if (exec) {
      const execIndicator = RendererUtils.createElement('span', 'exec-indicator', 'executing...');
      header.appendChild(execIndicator);
    }
    
    div.appendChild(header);
    
    // Input parameters
    if (input) {
      const inputDiv = RendererUtils.createElement('div', 'tool-input');
      
      if (typeof input === 'string') {
        inputDiv.textContent = input;
      } else {
        inputDiv.textContent = RendererUtils.formatJSON(input);
      }
      
      div.appendChild(inputDiv);
    }
    
    return div;
  }

  update(element, entity, patch) {
    // Update execution status if changed
    if (patch.exec !== undefined) {
      const header = element.querySelector('.tool-header');
      const existingIndicator = header.querySelector('.exec-indicator');
      
      if (patch.exec && !existingIndicator) {
        const execIndicator = RendererUtils.createElement('span', 'exec-indicator', 'executing...');
        header.appendChild(execIndicator);
      } else if (!patch.exec && existingIndicator) {
        header.removeChild(existingIndicator);
      }
    }
    
    // Update input if changed
    if (patch.input !== undefined) {
      const inputDiv = element.querySelector('.tool-input');
      if (inputDiv) {
        if (typeof patch.input === 'string') {
          inputDiv.textContent = patch.input;
        } else {
          inputDiv.textContent = RendererUtils.formatJSON(patch.input);
        }
      }
    }
    
    return element;
  }
}

export class ToolResultRenderer extends EntityRenderer {
  constructor() {
    super('tool_call_result');
  }
  
  render(entity) {
    const { props } = entity;
    const { result } = props;
    
    const div = RendererUtils.createElement('div', 'tool-result');
    
    // Header
    const header = RendererUtils.createElement('div', 'result-header');
    const icon = RendererUtils.createElement('span', 'result-icon', '📋');
    const label = RendererUtils.createElement('span', '', ' Result:');
    header.appendChild(icon);
    header.appendChild(label);
    div.appendChild(header);
    
    // Result content
    const content = RendererUtils.createElement('pre', 'result-content');
    if (typeof result === 'string') {
      content.textContent = result;
    } else {
      content.textContent = RendererUtils.formatJSON(result);
    }
    div.appendChild(content);
    
    return div;
  }
}
