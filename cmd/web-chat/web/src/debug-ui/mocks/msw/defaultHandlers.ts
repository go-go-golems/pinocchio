import {
  mockConversationDetail,
  mockConversations,
  mockSessions,
} from '../fixtures/conversations';
import { mockEvents, mockMwTrace } from '../fixtures/events';
import { mockOfflineRunDetails, mockOfflineRuns } from '../fixtures/offline';
import { mockTimelineEntities } from '../fixtures/timeline';
import { mockTurnDetail, mockTurns } from '../fixtures/turns';
import {
  type CreateDebugHandlersOptions,
  createDebugHandlers,
  type DebugHandlerData,
} from './createDebugHandlers';

export const defaultDebugHandlerData: DebugHandlerData = {
  conversations: mockConversations,
  conversationDetail: mockConversationDetail,
  sessions: mockSessions,
  turns: mockTurns,
  turnDetail: mockTurnDetail,
  events: mockEvents,
  timelineEntities: mockTimelineEntities,
  mwTrace: mockMwTrace,
  offlineRuns: mockOfflineRuns,
  runDetails: mockOfflineRunDetails,
};

type DefaultDebugHandlerOptions = Omit<CreateDebugHandlersOptions, 'data'>;

export function createDefaultDebugHandlers(
  dataOverrides: Partial<DebugHandlerData> = {},
  options: DefaultDebugHandlerOptions = {}
) {
  return createDebugHandlers({
    ...options,
    data: {
      ...defaultDebugHandlerData,
      ...dataOverrides,
    },
  });
}

export const defaultHandlers = createDefaultDebugHandlers();
