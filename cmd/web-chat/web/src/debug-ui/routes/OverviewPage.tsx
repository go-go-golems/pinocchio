
import { useParams } from 'react-router-dom';
import { useGetConversationQuery, useGetTurnDetailQuery } from '../api/debugApi';
import { TimelineLanes } from '../components/TimelineLanes';
import { TurnInspector } from '../components/TurnInspector';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectEntity, selectEvent, selectSession, selectTurn } from '../store/uiSlice';
import { useLiveLaneData } from './useLaneData';

export function OverviewPage() {
  const dispatch = useAppDispatch();
  const { sessionId } = useParams();
  const selectedConvId = useAppSelector((state) => state.ui.selectedConvId);
  const selectedSessionId = useAppSelector((state) => state.ui.selectedSessionId);
  const selectedTurnId = useAppSelector((state) => state.ui.selectedTurnId);
  const selectedEventSeq = useAppSelector((state) => state.ui.selectedSeq);
  const selectedEntityId = useAppSelector((state) => state.ui.selectedEntityId);
  const follow = useAppSelector((state) => state.ui.follow);

  const { data: conversation, isLoading: convLoading } = useGetConversationQuery(
    selectedConvId ?? '',
    { skip: !selectedConvId }
  );

  const laneData = useLiveLaneData(selectedConvId, sessionId ?? selectedSessionId ?? undefined);
  const followOnSelectedConv =
    !!selectedConvId &&
    follow.enabled &&
    (follow.targetConvId ?? selectedConvId) === selectedConvId;

  const { data: turnDetail, isLoading: turnDetailLoading } = useGetTurnDetailQuery(
    { 
      convId: selectedConvId ?? '', 
      sessionId: sessionId ?? selectedSessionId ?? conversation?.session_id ?? '',
      turnId: selectedTurnId ?? '' 
    },
    { skip: !selectedConvId || !selectedTurnId }
  );

  if (!selectedConvId) {
    return (
      <div className="overview-empty-state">
        <h2>👈 Select a conversation</h2>
        <p>Choose a conversation from the sidebar to view its details.</p>
      </div>
    );
  }

  if (convLoading || laneData.isLoading) {
    return (
      <div className="overview-loading-state">
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="overview-page">
      {/* Conversation header */}
      <div className="overview-conv-header">
        <h2>Conversation {selectedConvId.slice(0, 8)}</h2>
        <div className="overview-conv-meta">
          <span>Session: {sessionId || conversation?.session_id || '—'}</span>
          <span>Turns: {laneData.turns.length}</span>
          <span>Events: {laneData.events.length}</span>
          <span>Entities: {laneData.entities.length}</span>
          <span className={`timeline-live-status status-${followOnSelectedConv ? follow.status : 'idle'}`}>
            live: {followOnSelectedConv ? follow.status : 'idle'}
          </span>
        </div>
      </div>

      {/* Timeline view */}
      <div className="overview-timeline-section">
        <h3>Timeline</h3>
        <TimelineLanes
          turns={laneData.turns}
          events={laneData.events}
          entities={laneData.entities}
          isLive={followOnSelectedConv && (follow.status === 'connected' || follow.status === 'bootstrapping')}
          selectedTurnId={selectedTurnId ?? undefined}
          selectedEventSeq={selectedEventSeq ?? undefined}
          selectedEntityId={selectedEntityId ?? undefined}
          onTurnSelect={(turn) => {
            dispatch(selectSession(turn.session_id));
            dispatch(selectTurn(turn.turn_id));
          }}
          onEventSelect={(event) => dispatch(selectEvent(event.seq))}
          onEntitySelect={(entity) => dispatch(selectEntity(entity.id))}
        />
      </div>

      {/* Turn inspector (if turn selected) */}
      {selectedTurnId && turnDetail && !turnDetailLoading && (
        <div className="overview-turn-section">
          <h3>Turn Detail</h3>
          <TurnInspector turnDetail={turnDetail} />
        </div>
      )}
    </div>
  );
}

export default OverviewPage;
