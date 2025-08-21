// Base renderer class and utilities

import { EntityRenderer } from '../registry.js';

export { EntityRenderer };

// Utility functions for renderers
export const RendererUtils = {
  // Create element with class and content
  createElement(tag, className, content = '') {
    const element = document.createElement(tag);
    if (className) element.className = className;
    if (content) {
      if (typeof content === 'string') {
        element.textContent = content;
      } else {
        element.appendChild(content);
      }
    }
    return element;
  },

  // Create element with HTML
  createElementWithHTML(tag, className, html = '') {
    const element = document.createElement(tag);
    if (className) element.className = className;
    if (html) element.innerHTML = html;
    return element;
  },

  // Format metadata for display
  formatMetadata(metadata) {
    if (!metadata) return '';
    const parts = [];
    
    if (metadata.model) parts.push(metadata.model);
    if (metadata.usage) {
      const { input_tokens, output_tokens } = metadata.usage;
      if (input_tokens || output_tokens) {
        parts.push(`in: ${input_tokens || 0} out: ${output_tokens || 0}`);
      }
    }
    if (metadata.duration_ms) {
      parts.push(`${metadata.duration_ms}ms`);
    }
    
    return parts.join(' ');
  },

  // Format JSON for display
  formatJSON(obj, indent = 2) {
    try {
      return JSON.stringify(obj, null, indent);
    } catch (e) {
      return String(obj);
    }
  },

  // Escape HTML
  escapeHTML(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  },

  // Truncate text
  truncate(text, maxLength = 100) {
    if (!text || text.length <= maxLength) return text;
    return text.substring(0, maxLength - 3) + '...';
  },

  // Create collapsible details element
  createDetails(summary, content, open = false) {
    const details = document.createElement('details');
    if (open) details.open = true;

    const summaryEl = document.createElement('summary');
    summaryEl.textContent = summary;
    details.appendChild(summaryEl);

    if (typeof content === 'string') {
      const contentEl = document.createElement('div');
      contentEl.textContent = content;
      details.appendChild(contentEl);
    } else {
      details.appendChild(content);
    }

    return details;
  }
};
