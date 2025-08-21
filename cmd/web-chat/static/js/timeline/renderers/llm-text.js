// LLM Text renderer - similar to llm_text_model.go

import { EntityRenderer, RendererUtils } from './base.js';

export class LLMTextRenderer extends EntityRenderer {
  constructor() {
    super('llm_text');
  }
  
  render(entity) {
    const { props } = entity;
    const { role = 'assistant', text = '', streaming = false, metadata } = props;
    
    const div = RendererUtils.createElement('div', `msg ${role === 'user' ? 'user' : 'assistant'}`);
    
    // Role label
    const roleLabel = RendererUtils.createElement('span', 'role-label', `(${role}):`);
    div.appendChild(roleLabel);
    
    // Text content
    const textContent = RendererUtils.createElement('div', 'text-content', text);
    div.appendChild(textContent);
    
    // Status line (streaming indicator + metadata)
    const statusLine = this.createStatusLine(streaming, metadata);
    if (statusLine) {
      div.appendChild(statusLine);
    }
    
    return div;
  }

  createStatusLine(streaming, metadata) {
    const statusLine = RendererUtils.createElement('div', 'status-line');
    let hasContent = false;
    
    // Streaming indicator
    if (streaming) {
      const spinner = RendererUtils.createElement('span', 'spinner', 'Generating...');
      statusLine.appendChild(spinner);
      hasContent = true;
    }
    
    // Metadata
    if (metadata) {
      const metaText = RendererUtils.formatMetadata(metadata);
      if (metaText) {
        const metaSpan = RendererUtils.createElement('span', 'metadata', metaText);
        statusLine.appendChild(metaSpan);
        hasContent = true;
      }
    }
    
    return hasContent ? statusLine : null;
  }

  // Optimized update for text changes
  update(element, entity, patch) {
    const { text, streaming, metadata } = patch;
    
    // Update text content if changed
    if (text !== undefined) {
      const textContent = element.querySelector('.text-content');
      if (textContent) {
        textContent.textContent = text;
      }
    }
    
    // Update status line if streaming or metadata changed
    if (streaming !== undefined || metadata !== undefined) {
      const oldStatusLine = element.querySelector('.status-line');
      const newStatusLine = this.createStatusLine(
        streaming !== undefined ? streaming : entity.props.streaming,
        metadata !== undefined ? metadata : entity.props.metadata
      );
      
      if (oldStatusLine && newStatusLine) {
        element.replaceChild(newStatusLine, oldStatusLine);
      } else if (oldStatusLine && !newStatusLine) {
        element.removeChild(oldStatusLine);
      } else if (!oldStatusLine && newStatusLine) {
        element.appendChild(newStatusLine);
      }
    }
    
    return element;
  }
}
