import type { ConversationDetail, ConversationSummary, SessionSummary } from '../../types';
import {
  mockConversationDetail,
  mockConversations,
  mockSessions,
} from '../fixtures/conversations';
import { pickByIndex } from './common';
import {
  makeDeterministicId,
  makeDeterministicIsoTime,
  shouldApplyDeterministicOverrides,
} from './deterministic';

const CONVERSATION_ACTIVITY_BASE_MS = Date.parse(
  mockConversations[0]?.last_activity ?? '2026-02-06T14:30:00.000Z'
);
const SESSION_FIRST_TURN_BASE_MS = Date.parse(
  mockSessions[0]?.first_turn_at ?? '2026-02-06T14:30:00.000Z'
);

export function makeConversation(options: {
  index?: number;
  overrides?: Partial<ConversationSummary>;
} = {}): ConversationSummary {
  const { index = 0, overrides = {} } = options;
  return {
    ...pickByIndex(mockConversations, index),
    ...overrides,
  };
}

export function makeConversations(
  count: number,
  options: {
    startIndex?: number;
    mapOverrides?: (listIndex: number) => Partial<ConversationSummary>;
  } = {}
): ConversationSummary[] {
  const { startIndex = 0, mapOverrides } = options;
  return Array.from({ length: count }, (_, listIndex) => {
    const absoluteIndex = startIndex + listIndex;
    const synthetic = shouldApplyDeterministicOverrides(absoluteIndex, mockConversations.length);

    const deterministicOverrides: Partial<ConversationSummary> = synthetic
      ? {
        id: makeDeterministicId('conv', absoluteIndex, 4),
        session_id: makeDeterministicId('sess', absoluteIndex, 2),
        last_activity: makeDeterministicIsoTime(CONVERSATION_ACTIVITY_BASE_MS, absoluteIndex, 45_000),
        turn_count: absoluteIndex + 1,
      }
      : {};

    return makeConversation({
      index: absoluteIndex,
      overrides: {
        ...deterministicOverrides,
        ...(mapOverrides?.(listIndex) ?? {}),
      },
    });
  });
}

export function makeConversationDetail(options: {
  index?: number;
  overrides?: Partial<ConversationDetail>;
} = {}): ConversationDetail {
  const { index = 0, overrides = {} } = options;
  const baseDetail = pickByIndex([mockConversationDetail], 0);
  const baseSummary = pickByIndex(mockConversations, index);

  return {
    ...baseDetail,
    ...baseSummary,
    ...overrides,
    engine_config: {
      ...baseDetail.engine_config,
      ...(overrides.engine_config ?? {}),
    },
  };
}

export function makeSession(options: {
  index?: number;
  overrides?: Partial<SessionSummary>;
} = {}): SessionSummary {
  const { index = 0, overrides = {} } = options;
  return {
    ...pickByIndex(mockSessions, index),
    ...overrides,
  };
}

export function makeSessions(
  count: number,
  options: {
    startIndex?: number;
    mapOverrides?: (listIndex: number) => Partial<SessionSummary>;
  } = {}
): SessionSummary[] {
  const { startIndex = 0, mapOverrides } = options;
  return Array.from({ length: count }, (_, listIndex) => {
    const absoluteIndex = startIndex + listIndex;
    const synthetic = shouldApplyDeterministicOverrides(absoluteIndex, mockSessions.length);

    const deterministicOverrides: Partial<SessionSummary> = synthetic
      ? {
        session_id: makeDeterministicId('sess', absoluteIndex, 2),
        first_turn_at: makeDeterministicIsoTime(SESSION_FIRST_TURN_BASE_MS, absoluteIndex, 90_000),
        last_turn_at: makeDeterministicIsoTime(SESSION_FIRST_TURN_BASE_MS + 45_000, absoluteIndex, 90_000),
        turn_count: absoluteIndex + 1,
      }
      : {};

    return makeSession({
      index: absoluteIndex,
      overrides: {
        ...deterministicOverrides,
        ...(mapOverrides?.(listIndex) ?? {}),
      },
    });
  });
}
