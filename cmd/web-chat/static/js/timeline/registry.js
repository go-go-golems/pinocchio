// Timeline renderer registry - manages entity renderers

export class RendererRegistry {
  constructor() {
    this.renderers = new Map(); // kind -> renderer factory
  }

  // Register entity renderer
  register(kind, factory) {
    if (typeof factory.render !== 'function') {
      throw new Error(`Renderer for kind '${kind}' must have a render() method`);
    }
    this.renderers.set(kind, factory);
    console.log('Timeline: registered renderer for kind', kind);
  }

  // Get renderer for kind
  get(kind) {
    return this.renderers.get(kind);
  }

  // Check if renderer exists
  has(kind) {
    return this.renderers.has(kind);
  }

  // Get all registered kinds
  getKinds() {
    return Array.from(this.renderers.keys());
  }

  // Unregister renderer
  unregister(kind) {
    return this.renderers.delete(kind);
  }

  // Clear all renderers
  clear() {
    this.renderers.clear();
  }
}

// Base renderer class
export class EntityRenderer {
  constructor(kind) {
    this.kind = kind;
  }
  
  render(entity) {
    throw new Error(`render() must be implemented by ${this.constructor.name}`);
  }

  // Optional: handle entity updates without full re-render
  update(element, entity, patch) {
    // Default: full re-render
    const newElement = this.render(entity);
    if (element.parentNode) {
      element.parentNode.replaceChild(newElement, element);
    }
    return newElement;
  }

  // Optional: cleanup when entity is deleted
  cleanup(element, entity) {
    // Default: remove from DOM
    if (element.parentNode) {
      element.parentNode.removeChild(element);
    }
  }
}
