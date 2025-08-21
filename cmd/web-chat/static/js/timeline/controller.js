// Timeline controller - orchestrates store, registry, and rendering

import { TimelineStore } from './store.js';
import { RendererRegistry } from './registry.js';

export class TimelineController {
  constructor(container) {
    this.container = container;
    this.store = new TimelineStore();
    this.registry = new RendererRegistry();
    this.elementMap = new Map(); // entityId -> DOM element
    
    // Subscribe to store changes
    this.store.subscribe(() => this.render());
  }

  // Register renderer
  registerRenderer(kind, factory) {
    this.registry.register(kind, factory);
  }

  // Apply lifecycle event
  applyEvent(event) {
    this.store.applyEvent(event);
  }

  // Render all entities
  render() {
    const entities = this.store.getOrderedEntities();
    
    // Track which entities should be rendered
    const currentEntityIds = new Set(entities.map(e => e.id));
    
    // Remove deleted entities
    for (const [entityId, element] of this.elementMap.entries()) {
      if (!currentEntityIds.has(entityId)) {
        const entity = { id: entityId }; // minimal entity for cleanup
        const renderer = this.registry.get(element.dataset.entityKind);
        if (renderer && typeof renderer.cleanup === 'function') {
          renderer.cleanup(element, entity);
        } else if (element.parentNode) {
          element.parentNode.removeChild(element);
        }
        this.elementMap.delete(entityId);
      }
    }

    // Render entities in order
    let lastElement = null;
    for (const entity of entities) {
      const existingElement = this.elementMap.get(entity.id);
      
      if (existingElement) {
        // Update existing element
        this.updateEntity(entity, existingElement);
        lastElement = existingElement;
      } else {
        // Create new element
        const element = this.createElement(entity);
        if (element) {
          this.insertAfter(element, lastElement);
          this.elementMap.set(entity.id, element);
          lastElement = element;
        }
      }
    }

    // Auto-scroll to bottom
    this.scrollToBottom();
  }

  // Create DOM element for entity
  createElement(entity) {
    const renderer = this.registry.get(entity.kind);
    if (!renderer) {
      console.warn('No renderer for kind:', entity.kind);
      return this.createFallbackElement(entity);
    }
    
    try {
      const element = renderer.render(entity);
      if (element) {
        element.dataset.entityId = entity.id;
        element.dataset.entityKind = entity.kind;
      }
      return element;
    } catch (error) {
      console.error('Renderer error for kind', entity.kind, error);
      return this.createErrorElement(entity, error);
    }
  }

  // Update existing DOM element
  updateEntity(entity, element) {
    const renderer = this.registry.get(entity.kind);
    if (!renderer) return;

    try {
      if (typeof renderer.update === 'function') {
        // Use renderer's update method if available
        const newElement = renderer.update(element, entity, entity.props);
        if (newElement !== element) {
          this.elementMap.set(entity.id, newElement);
        }
      } else {
        // Fall back to full re-render
        const newElement = renderer.render(entity);
        if (newElement) {
          newElement.dataset.entityId = entity.id;
          newElement.dataset.entityKind = entity.kind;
          element.parentNode.replaceChild(newElement, element);
          this.elementMap.set(entity.id, newElement);
        }
      }
    } catch (error) {
      console.error('Update error for entity', entity.id, error);
    }
  }

  // Insert element after reference element
  insertAfter(element, referenceElement) {
    if (!referenceElement) {
      // Insert at beginning
      this.container.insertBefore(element, this.container.firstChild);
    } else {
      // Insert after reference
      this.container.insertBefore(element, referenceElement.nextSibling);
    }
  }

  // Create fallback element for unknown kinds
  createFallbackElement(entity) {
    const div = document.createElement('div');
    div.className = 'timeline-entity timeline-unknown';
    div.innerHTML = `
      <div class="unknown-header">Unknown entity type: ${entity.kind}</div>
      <pre class="unknown-props">${JSON.stringify(entity.props, null, 2)}</pre>
    `;
    return div;
  }

  // Create error element for renderer failures
  createErrorElement(entity, error) {
    const div = document.createElement('div');
    div.className = 'timeline-entity timeline-error';
    div.innerHTML = `
      <div class="error-header">Renderer error for ${entity.kind}</div>
      <div class="error-message">${error.message}</div>
      <pre class="error-props">${JSON.stringify(entity.props, null, 2)}</pre>
    `;
    return div;
  }

  // Scroll to bottom
  scrollToBottom() {
    if (this.container) {
      this.container.scrollTop = this.container.scrollHeight;
    }
  }

  // Clear timeline
  clear() {
    this.store.clear();
    this.elementMap.clear();
    if (this.container) {
      this.container.innerHTML = '';
    }
  }

  // Get stats
  getStats() {
    return this.store.getStats();
  }
}
