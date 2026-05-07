import { fromJson, type JsonValue } from '@bufbuild/protobuf';
import type {
  AgentModeCommittedUpdate,
  AgentModePreviewCleared,
  AgentModePreviewUpdate,
  ChatMessageUpdate,
  ReasoningUpdate,
  ToolCallUpdate,
  ToolResultUpdate,
} from '../chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb';
import {
  AgentModeCommittedUpdateSchema,
  AgentModePreviewClearedSchema,
  AgentModePreviewUpdateSchema,
  ChatMessageUpdateSchema,
  ReasoningUpdateSchema,
  ToolCallUpdateSchema,
  ToolResultUpdateSchema,
} from '../chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb';
import { asRecord, asString, type CanonicalFrame } from './protocol';

export type ChatMessageUIEventName =
  | 'ChatMessageAccepted'
  | 'ChatMessageStarted'
  | 'ChatMessageAppended'
  | 'ChatMessageFinished'
  | 'ChatMessageStopped';

export type ReasoningUIEventName = 'ChatReasoningStarted' | 'ChatReasoningAppended' | 'ChatReasoningFinished';

export type ToolUIEventName = 'ChatToolCallStarted' | 'ChatToolCallUpdated' | 'ChatToolCallFinished' | 'ChatToolResultReady';

export type AgentModeUIEventName = 'ChatAgentModePreviewUpdated' | 'ChatAgentModeCommitted' | 'ChatAgentModePreviewCleared';

export type KnownUIEvent =
  | { name: ChatMessageUIEventName; payload: ChatMessageUpdate }
  | { name: ReasoningUIEventName; payload: ReasoningUpdate }
  | { name: Exclude<ToolUIEventName, 'ChatToolResultReady'>; payload: ToolCallUpdate }
  | { name: 'ChatToolResultReady'; payload: ToolResultUpdate }
  | { name: 'ChatAgentModePreviewUpdated'; payload: AgentModePreviewUpdate }
  | { name: 'ChatAgentModeCommitted'; payload: AgentModeCommittedUpdate }
  | { name: 'ChatAgentModePreviewCleared'; payload: AgentModePreviewCleared };

function jsonPayload(frame: CanonicalFrame): JsonValue {
  return asRecord(frame.payload) as JsonValue;
}

export function decodeKnownUIEvent(frame: CanonicalFrame): KnownUIEvent | null {
  const name = asString(frame.name);
  try {
    switch (name) {
      case 'ChatMessageAccepted':
      case 'ChatMessageStarted':
      case 'ChatMessageAppended':
      case 'ChatMessageFinished':
      case 'ChatMessageStopped':
        return { name, payload: fromJson(ChatMessageUpdateSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatReasoningStarted':
      case 'ChatReasoningAppended':
      case 'ChatReasoningFinished':
        return { name, payload: fromJson(ReasoningUpdateSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolCallStarted':
      case 'ChatToolCallUpdated':
      case 'ChatToolCallFinished':
        return { name, payload: fromJson(ToolCallUpdateSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolResultReady':
        return { name, payload: fromJson(ToolResultUpdateSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatAgentModePreviewUpdated':
        return { name, payload: fromJson(AgentModePreviewUpdateSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatAgentModeCommitted':
        return { name, payload: fromJson(AgentModeCommittedUpdateSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatAgentModePreviewCleared':
        return { name, payload: fromJson(AgentModePreviewClearedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      default:
        return null;
    }
  } catch {
    return null;
  }
}
