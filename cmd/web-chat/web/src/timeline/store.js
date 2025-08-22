// Timeline entity store - manages entity lifecycle and ordering

import { EntityLifecycle } from './types.js';

export class TimelineStore {
  constructor() {
    this.entities = new Map(); // entityId -> entity
    this.order = []; // ordered list of entity IDs
    this.subscribers = new Set();
  }

  // Subscribe to timeline changes
  subscribe(callback) {
    this.subscribers.add(callback);
    return () => this.subscribers.delete(callback);
  }

  // Notify subscribers of changes
  notify() {
    this.subscribers.forEach(cb => {
      try { cb(this); } catch(e) { console.error('Timeline subscriber error:', e); }
    });
  }

  // Apply lifecycle event
  applyEvent(event) {
    const { type, entityId, ...data } = event;
    
    switch(type) {
      case EntityLifecycle.CREATED:
        this.onCreate(entityId, data);
        break;
      case EntityLifecycle.UPDATED:
        this.onUpdate(entityId, data);
        break;
      case EntityLifecycle.COMPLETED:
        this.onComplete(entityId, data);
        break;
      case EntityLifecycle.DELETED:
        this.onDelete(entityId);
        break;
      default:
        console.warn('Unknown timeline event type:', type);
    }
    this.notify();
  }

  onCreate(entityId, data) {
    const { kind, renderer, props, startedAt } = data;
    if (this.entities.has(entityId)) {
      console.warn('Entity already exists:', entityId);
      return;
    }
    
    const entity = {
      id: entityId,
      kind,
      renderer: renderer || { kind },
      props: { ...props },
      startedAt: startedAt || Date.now(),
      completed: false,
      result: null,
      version: 0,
      updatedAt: null,
      completedAt: null
    };
    
    this.entities.set(entityId, entity);
    this.order.push(entityId);
    console.log('Timeline: created entity', entityId, kind);
  }

  onUpdate(entityId, data) {
    let entity = this.entities.get(entityId);
    if (!entity) {
      console.warn('Entity not found for update, creating placeholder:', entityId);
      // Best-effort infer kind from patch; default to llm_text
      let inferredKind = 'llm_text';
      if (data && data.patch) {
        if (Object.prototype.hasOwnProperty.call(data.patch, 'exec') || Object.prototype.hasOwnProperty.call(data.patch, 'input')) {
          inferredKind = 'tool_call';
        }
      }
      this.onCreate(entityId, { kind: inferredKind, renderer: { kind: inferredKind }, props: {} });
      entity = this.entities.get(entityId);
    }
    
    const { patch, version, updatedAt } = data;
    Object.assign(entity.props, patch);
    entity.version = version || (entity.version + 1);
    entity.updatedAt = updatedAt || Date.now();
    console.log('Timeline: updated entity', entityId, patch);

    // Special rule: when a tool_call_result (generic) and a corresponding custom <tool>_result exist,
    // suppress the generic result to avoid duplicate widgets.
    if (entity.kind === 'tool_call_result' && typeof entity.id === 'string') {
      const base = entity.id.replace(/:result$/, '');
      // Custom entity id format used in forwarder: <tool_result_id>:custom
      const customId = base + ':custom';
      const custom = this.entities.get(customId);
      if (custom) {
        // Remove generic result entity
        this.entities.delete(entityId);
        const idx = this.order.indexOf(entityId);
        if (idx >= 0) this.order.splice(idx, 1);
        console.log('Timeline: removed generic tool_call_result due to presence of custom result', entityId, 'custom:', customId);
      }
    }
  }

  onComplete(entityId, data) {
    const entity = this.entities.get(entityId);
    if (!entity) {
      console.warn('Entity not found for completion:', entityId);
      return;
    }
    
    const { result } = data;
    entity.completed = true;
    if (result !== null && result !== undefined) {
      entity.result = result;
      // Merge final result into props if it contains display data
      if (typeof result === 'object' && result !== null) {
        Object.assign(entity.props, result);
      }
    }
    entity.completedAt = Date.now();
    console.log('Timeline: completed entity', entityId);
  }

  onDelete(entityId) {
    const entity = this.entities.get(entityId);
    if (!entity) {
      console.warn('Entity not found for deletion:', entityId);
      return;
    }
    
    this.entities.delete(entityId);
    const idx = this.order.indexOf(entityId);
    if (idx >= 0) this.order.splice(idx, 1);
    console.log('Timeline: deleted entity', entityId);
  }

  // Get entity by ID
  getEntity(entityId) {
    return this.entities.get(entityId);
  }

  // Get all entities in order
  getOrderedEntities() {
    return this.order.map(id => this.entities.get(id)).filter(Boolean);
  }

  // Get entities by kind
  getEntitiesByKind(kind) {
    return this.getOrderedEntities().filter(entity => entity.kind === kind);
  }

  // Clear all entities
  clear() {
    this.entities.clear();
    this.order.length = 0;
    this.notify();
  }

  // Get stats
  getStats() {
    const entities = this.getOrderedEntities();
    const byKind = {};
    let completed = 0;
    
    entities.forEach(entity => {
      byKind[entity.kind] = (byKind[entity.kind] || 0) + 1;
      if (entity.completed) completed++;
    });
    
    return {
      total: entities.length,
      completed,
      byKind
    };
  }
}
