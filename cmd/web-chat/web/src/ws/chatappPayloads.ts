import { fromJson, type JsonValue } from '@bufbuild/protobuf';
import type {
  AgentModeCommittedUpdate,
  AgentModePreviewCleared,
  AgentModePreviewUpdate,
  ChatProviderCallFinished,
  ChatProviderCallMetadataUpdated,
  ChatProviderCallStarted,
  ChatReasoningPatch,
  ChatReasoningSegmentFinished,
  ChatReasoningSegmentStarted,
  ChatRunFailed,
  ChatRunFinished,
  ChatRunStarted,
  ChatRunStopped,
  ChatTextPatch,
  ChatTextSegmentFinished,
  ChatTextSegmentStarted,
  ChatToolArgumentsPatch,
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
  ChatReasoningPatchSchema,
  ChatReasoningSegmentFinishedSchema,
  ChatReasoningSegmentStartedSchema,
  ChatRunFailedSchema,
  ChatRunFinishedSchema,
  ChatRunStartedSchema,
  ChatRunStoppedSchema,
  ChatTextPatchSchema,
  ChatTextSegmentFinishedSchema,
  ChatTextSegmentStartedSchema,
  ChatToolArgumentsPatchSchema,
  ChatToolCallFinishedSchema,
  ChatToolCallRequestedSchema,
  ChatToolCallStartedSchema,
  ChatToolExecutionStartedSchema,
  ChatToolResultReadySchema,
  ChatUserMessageAcceptedSchema,
} from '../chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb';
import { asRecord, asString, type CanonicalFrame } from './protocol';

export type ChatTextUIEventName = 'ChatUserMessageAccepted' | 'ChatTextSegmentStarted' | 'ChatTextPatch' | 'ChatTextSegmentFinished';
export type ChatRunUIEventName = 'ChatRunStarted' | 'ChatRunFinished' | 'ChatRunStopped' | 'ChatRunFailed';
export type ProviderCallUIEventName = 'ChatProviderCallStarted' | 'ChatProviderCallMetadataUpdated' | 'ChatProviderCallFinished';
export type ReasoningUIEventName = 'ChatReasoningSegmentStarted' | 'ChatReasoningPatch' | 'ChatReasoningSegmentFinished';
export type ToolUIEventName =
  | 'ChatToolCallStarted'
  | 'ChatToolArgumentsPatch'
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
  | { name: 'ChatTextPatch'; payload: ChatTextPatch }
  | { name: 'ChatTextSegmentFinished'; payload: ChatTextSegmentFinished }
  | { name: 'ChatReasoningSegmentStarted'; payload: ChatReasoningSegmentStarted }
  | { name: 'ChatReasoningPatch'; payload: ChatReasoningPatch }
  | { name: 'ChatReasoningSegmentFinished'; payload: ChatReasoningSegmentFinished }
  | { name: 'ChatToolCallStarted'; payload: ChatToolCallStarted }
  | { name: 'ChatToolArgumentsPatch'; payload: ChatToolArgumentsPatch }
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
      case 'ChatTextPatch':
        return { name, payload: fromJson(ChatTextPatchSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatTextSegmentFinished':
        return { name, payload: fromJson(ChatTextSegmentFinishedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatReasoningSegmentStarted':
        return { name, payload: fromJson(ChatReasoningSegmentStartedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatReasoningPatch':
        return { name, payload: fromJson(ChatReasoningPatchSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatReasoningSegmentFinished':
        return { name, payload: fromJson(ChatReasoningSegmentFinishedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolCallStarted':
        return { name, payload: fromJson(ChatToolCallStartedSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
      case 'ChatToolArgumentsPatch':
        return { name, payload: fromJson(ChatToolArgumentsPatchSchema, jsonPayload(frame), { ignoreUnknownFields: true }) };
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
