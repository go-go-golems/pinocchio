// Timeline system main exports

export { EntityLifecycle, EntityKind, EntityID, RendererDescriptor, UIEntityCreated, UIEntityUpdated, UIEntityCompleted, UIEntityDeleted } from './types.js';
export { TimelineStore } from './store.js';
export { RendererRegistry, EntityRenderer } from './registry.js';
export { TimelineController } from './controller.js';

// Renderers
export { LLMTextRenderer } from './renderers/llm-text.js';
export { ToolCallRenderer, ToolResultRenderer } from './renderers/tool-call.js';
export { AgentModeRenderer } from './renderers/agent-mode.js';
export { LogEventRenderer } from './renderers/log-event.js';

// Create a timeline with default renderers
export function createTimeline(container) {
  const timeline = new TimelineController(container);
  
  // Register default renderers
  timeline.registerRenderer('llm_text', new LLMTextRenderer());
  timeline.registerRenderer('tool_call', new ToolCallRenderer());
  timeline.registerRenderer('tool_call_result', new ToolResultRenderer());
  timeline.registerRenderer('agent_mode', new AgentModeRenderer());
  timeline.registerRenderer('log_event', new LogEventRenderer());
  
  return timeline;
}
