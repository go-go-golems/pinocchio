import { fromJson, type JsonValue } from '@bufbuild/protobuf';
import type {
  AgentModeCommittedUpdate,
  AgentModePreviewCleared,
  AgentModePreviewUpdate,
  ChatProviderCallFinished,
  ChatProviderCallMetadataUpdated,
  ChatProviderCallStarted,
  ChatReasoningDelta,
  ChatReasoningSegmentFinished,
  ChatReasoningSegmentStarted,
  ChatRunFailed,
  ChatRunFinished,
  ChatRunStarted,
  ChatRunStopped,
  ChatTextDelta,
  ChatTextSegmentFinished,
  ChatTextSegmentStarted,
  ChatToolCallArgumentsDelta,
  ChatToolCallFinished,
  ChatToolCallRequested,
  ChatToolCallStarted,
  ChatToolExecutionStarted,
  ChatToolResultReady,
  ChatUserMessageAccepted,
} from '../chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb';
import {
  AgentModeCommittedUpdateSchema,
  AgentModePreviewClearedSchema,
  AgentModePreviewUpdateSchema,
  ChatProviderCallFinishedSchema,
  ChatProviderCallMetadataUpdatedSchema,
  ChatProviderCallStartedSchema,
  ChatReasoningDeltaSchema,
  ChatReasoningSegmentFinishedSchema,
  ChatReasoningSegmentStartedSchema,
  ChatRunFailedSchema,
  ChatRunFinishedSchema,
  ChatRunStartedSchema,
  ChatRunStoppedSchema,
  ChatTextDeltaSchema,
  ChatTextSegmentFinishedSchema,
  ChatTextSegmentStartedSchema,
  ChatToolCallArgumentsDeltaSchema,
  ChatToolCallFinishedSchema,
  ChatToolCallRequestedSchema,
  ChatToolCallStartedSchema,
  ChatToolExecutionStartedSchema,
  ChatToolResultReadySchema,
  ChatUserMessageAcceptedSchema,
} from '../chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb';
import { asRecord, asString, type CanonicalFrame } from './protocol';

export type ChatTextUIEventName = 'ChatUserMessageAccepted' | 'ChatTextSegmentStarted' | 'ChatTextDelta' | 'ChatTextSegmentFinished';
export type ChatRunUIEventName = 'ChatRunStarted' | 'ChatRunFinished' | 'ChatRunStopped' | 'ChatRunFailed';
export type ProviderCallUIEventName = 'ChatProviderCallStarted' | 'ChatProviderCallMetadataUpdated' | 'ChatProviderCallFinished';
export type ReasoningUIEventName = 'ChatReasoningSegmentStarted' | 'ChatReasoningDelta' | 'ChatReasoningSegmentFinished';
export type ToolUIEventName =
  | 'ChatToolCallStarted'
  | 'ChatToolCallArgumentsDelta'
  | 'ChatToolCallRequested'
  | 'ChatToolExecutionStarted'
  | 'ChatToolResultReady'
  | 'ChatToolCallFinished';
export type AgentModeUIEventName = 'ChatAgentModePreviewUpdated' | 'ChatAgentModeCommitted' | 'ChatAgentModePreviewCleared';

export type KnownUIEvent =
  | { name: 'ChatUserMessageAccepted'; payload: ChatUserMessageAccepted }
  | { name: 'ChatRunStarted'; payload: ChatRunStarted }
  | { name: 'ChatRunFinished'; payload: ChatRunFinished }
  | { name: 'ChatRunStopped'; payload: ChatRunStopped }
  | { name: 'ChatRunFailed'; payload: ChatRunFailed }
  | { name: 'ChatProviderCallStarted'; payload: ChatProviderCallStarted }
  | { name: 'ChatProviderCallMetadataUpdated'; payload: ChatProviderCallMetadataUpdated }
  | { name: 'ChatProviderCallFinished'; payload: ChatProviderCallFinished }
  | { name: 'ChatTextSegmentStarted'; payload: ChatTextSegmentStarted }
  | { name: 'ChatTextDelta'; payload: ChatTextDelta }
  | { name: 'ChatTextSegmentFinished'; payload: ChatTextSegmentFinished }
  | { name: 'ChatReasoningSegmentStarted'; payload: ChatReasoningSegmentStarted }
  | { name: 'ChatReasoningDelta'; payload: ChatReasoningDelta }
  | { name: 'ChatReasoningSegmentFinished'; payload: ChatReasoningSegmentFinished }
  | { name: 'ChatToolCallStarted'; payload: ChatToolCallStarted }
  | { name: 'ChatToolCallArgumentsDelta'; payload: ChatToolCallArgumentsDelta }
  | { name: 'ChatToolCallRequested'; payload: ChatToolCallRequested }
  | { name: 'ChatToolExecutionStarted'; payload: ChatToolExecutionStarted }
  | { name: 'ChatToolResultReady'; payload: ChatToolResultReady }
  | { name: 'ChatToolCallFinished'; payload: ChatToolCallFinished }
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
      case 'ChatUserMessageAccepted':
        return { name, payload: fromJson(ChatUserMessageAcceptedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatRunStarted':
        return { name, payload: fromJson(ChatRunStartedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatRunFinished':
        return { name, payload: fromJson(ChatRunFinishedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatRunStopped':
        return { name, payload: fromJson(ChatRunStoppedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatRunFailed':
        return { name, payload: fromJson(ChatRunFailedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatProviderCallStarted':
        return { name, payload: fromJson(ChatProviderCallStartedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatProviderCallMetadataUpdated':
        return { name, payload: fromJson(ChatProviderCallMetadataUpdatedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatProviderCallFinished':
        return { name, payload: fromJson(ChatProviderCallFinishedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatTextSegmentStarted':
        return { name, payload: fromJson(ChatTextSegmentStartedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatTextDelta':
        return { name, payload: fromJson(ChatTextDeltaSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatTextSegmentFinished':
        return { name, payload: fromJson(ChatTextSegmentFinishedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatReasoningSegmentStarted':
        return { name, payload: fromJson(ChatReasoningSegmentStartedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatReasoningDelta':
        return { name, payload: fromJson(ChatReasoningDeltaSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatReasoningSegmentFinished':
        return { name, payload: fromJson(ChatReasoningSegmentFinishedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolCallStarted':
        return { name, payload: fromJson(ChatToolCallStartedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolCallArgumentsDelta':
        return { name, payload: fromJson(ChatToolCallArgumentsDeltaSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolCallRequested':
        return { name, payload: fromJson(ChatToolCallRequestedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolExecutionStarted':
        return { name, payload: fromJson(ChatToolExecutionStartedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolResultReady':
        return { name, payload: fromJson(ChatToolResultReadySchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolCallFinished':
        return { name, payload: fromJson(ChatToolCallFinishedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
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
