// Timeline entity types and constants

export const EntityLifecycle = {
  CREATED: 'created',
  UPDATED: 'updated', 
  COMPLETED: 'completed',
  DELETED: 'deleted'
};

export const EntityKind = {
  LLM_TEXT: 'llm_text',
  TOOL_CALL: 'tool_call',
  TOOL_RESULT: 'tool_call_result',
  AGENT_MODE: 'agent_mode',
  LOG_EVENT: 'log_event'
};

// Entity ID structure
export class EntityID {
  constructor(localId, kind, runId = null, turnId = null, blockId = null) {
    this.localId = localId;
    this.kind = kind;
    this.runId = runId;
    this.turnId = turnId;
    this.blockId = blockId;
  }

  toString() {
    return this.localId;
  }

  equals(other) {
    return other && this.localId === other.localId && this.kind === other.kind;
  }
}

// Renderer descriptor
export class RendererDescriptor {
  constructor(kind, key = null) {
    this.kind = kind;
    this.key = key || `renderer.${kind}.v1`;
  }
}

// Lifecycle event types
export class UIEntityCreated {
  constructor(id, renderer, props = {}, startedAt = null) {
    this.type = EntityLifecycle.CREATED;
    this.entityId = id.toString();
    this.id = id;
    this.kind = id.kind;
    this.renderer = renderer;
    this.props = props;
    this.startedAt = startedAt || Date.now();
  }
}

export class UIEntityUpdated {
  constructor(id, patch = {}, version = null, updatedAt = null) {
    this.type = EntityLifecycle.UPDATED;
    this.entityId = id.toString();
    this.id = id;
    this.patch = patch;
    this.version = version || Date.now();
    this.updatedAt = updatedAt || Date.now();
  }
}

export class UIEntityCompleted {
  constructor(id, result = null) {
    this.type = EntityLifecycle.COMPLETED;
    this.entityId = id.toString();
    this.id = id;
    this.result = result;
  }
}

export class UIEntityDeleted {
  constructor(id) {
    this.type = EntityLifecycle.DELETED;
    this.entityId = id.toString();
    this.id = id;
  }
}
