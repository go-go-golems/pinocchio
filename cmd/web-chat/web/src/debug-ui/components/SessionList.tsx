import React from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useGetConversationsQuery } from '../api/debugApi';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectConversation, selectSession, selectTurn } from '../store/uiSlice';
import type { ConversationSummary } from '../types';
import { ConversationCard } from './ConversationCard';

export interface SessionListProps {
  /** Override with custom conversations (for Storybook) */
  conversations?: ReturnType<typeof useGetConversationsQuery>['data'];
  /** Loading state override (for Storybook) */
  isLoading?: boolean;
  /** Error state override (for Storybook) */
  error?: string;
}

export function SessionList({ conversations: overrideConversations, isLoading: overrideLoading, error: overrideError }: SessionListProps) {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const location = useLocation();
  const selectedConvId = useAppSelector((state) => state.ui.selectedConvId);
  
  const { data: apiConversations, isLoading: apiLoading, error: apiError } = useGetConversationsQuery();

  // Use overrides if provided (for Storybook), otherwise use API data
  const conversations = overrideConversations ?? apiConversations;
  const isLoading = overrideLoading ?? apiLoading;
  const error = overrideError ?? (apiError ? 'Failed to load conversations' : undefined);

  const handleSelect = (conversation: ConversationSummary) => {
    dispatch(selectConversation(conversation.id));
    dispatch(selectSession(conversation.session_id));
    dispatch(selectTurn(null));

    const next = new URLSearchParams(location.search);
    next.set('conv', conversation.id);
    next.set('session', conversation.session_id);
    next.delete('turn');
    navigate({ pathname: '/', search: `?${next.toString()}` });
  };

  return (
    <div className="session-list flex-1 overflow-auto p-3">
      <div className="flex items-center justify-between mb-3">
        <h3>Conversations</h3>
        <span className="text-xs text-muted">
          {conversations?.length ?? 0} active
        </span>
      </div>

      {isLoading && (
        <div className="text-center text-muted p-4">
          Loading conversations...
        </div>
      )}

      {error && (
        <div className="text-center text-sm p-4" style={{ color: 'var(--accent-red)' }}>
          {error}
        </div>
      )}

      {!isLoading && !error && conversations?.length === 0 && (
        <div className="text-center text-muted p-4">
          No active conversations
        </div>
      )}

      <div className="list">
        {conversations?.map((conv: ConversationSummary) => (
          <ConversationCard
            key={conv.id}
            conversation={conv}
            selected={conv.id === selectedConvId}
            onClick={() => handleSelect(conv)}
          />
        ))}
      </div>
    </div>
  );
}

export default SessionList;
