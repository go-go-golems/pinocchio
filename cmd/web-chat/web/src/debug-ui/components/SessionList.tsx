
import { useLocation, useNavigate } from 'react-router-dom';
import { useGetConversationsQuery } from '../api/debugApi';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import {
  pauseFollow,
  requestFollowReconnect,
  resumeFollow,
  selectConversation,
  selectSession,
  selectTurn,
  setFollowTarget,
  startFollow,
} from '../store/uiSlice';
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
  const follow = useAppSelector((state) => state.ui.follow);
  
  const { data: apiConversations, isLoading: apiLoading, error: apiError } = useGetConversationsQuery();

  // Use overrides if provided (for Storybook), otherwise use API data
  const conversations = overrideConversations ?? apiConversations;
  const isLoading = overrideLoading ?? apiLoading;
  const error = overrideError ?? (apiError ? 'Failed to load conversations' : undefined);
  const selectedConversation =
    selectedConvId && conversations
      ? conversations.find((conversation: ConversationSummary) => conversation.id === selectedConvId) ?? null
      : null;
  const canFollowSelected = !!selectedConversation && selectedConversation.ws_connections > 0;
  const followingSelected = !!selectedConvId && follow.targetConvId === selectedConvId;
  const canResumeSelected = followingSelected && !follow.enabled;

  const handleSelect = (conversation: ConversationSummary) => {
    dispatch(selectConversation(conversation.id));
    dispatch(selectSession(conversation.session_id));
    dispatch(selectTurn(null));
    if (follow.enabled) {
      dispatch(startFollow(conversation.id));
    } else if (follow.targetConvId) {
      dispatch(setFollowTarget(conversation.id));
    }

    const next = new URLSearchParams(location.search);
    next.set('conv', conversation.id);
    next.set('session', conversation.session_id);
    next.delete('turn');
    navigate({ pathname: '/', search: `?${next.toString()}` });
  };

  const toggleFollow = () => {
    if (!selectedConvId) {
      return;
    }
    if (followingSelected && follow.enabled) {
      dispatch(pauseFollow());
      return;
    }
    if (canResumeSelected) {
      dispatch(resumeFollow());
      return;
    }
    dispatch(startFollow(selectedConvId));
  };

  return (
    <div className="session-list flex-1 overflow-auto p-3">
      <div className="flex items-center justify-between mb-3">
        <h3>Conversations</h3>
        <span className="text-xs text-muted">
          {conversations?.length ?? 0} active
        </span>
      </div>

      <div className="session-follow-controls">
        <button
          className="btn btn-sm"
          type="button"
          disabled={!canFollowSelected && !canResumeSelected}
          onClick={toggleFollow}
          title={!canFollowSelected ? 'Select a conversation with active websocket sockets' : ''}
        >
          {followingSelected && follow.enabled
            ? 'Pause Follow'
            : canResumeSelected
              ? 'Resume Follow'
              : 'Follow Live'}
        </button>
        <button
          className="btn btn-sm"
          type="button"
          disabled={!followingSelected}
          onClick={() => dispatch(requestFollowReconnect())}
        >
          Reconnect
        </button>
        <span className={`follow-status-chip status-${follow.status}`}>
          {follow.status}
        </span>
      </div>

      {follow.lastError && (
        <div className="session-follow-error" role="alert">
          {follow.lastError}
        </div>
      )}

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
