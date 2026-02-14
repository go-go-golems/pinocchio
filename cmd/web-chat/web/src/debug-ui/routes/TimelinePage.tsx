
import { TimelineLanes } from '../components/TimelineLanes';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectEntity, selectEvent, selectSession, selectTurn } from '../store/uiSlice';
import { useLiveLaneData } from './useLaneData';

export function TimelinePage() {
  const dispatch = useAppDispatch();
  const selectedConvId = useAppSelector((state) => state.ui.selectedConvId);
  const selectedTurnId = useAppSelector((state) => state.ui.selectedTurnId);
  const selectedEventSeq = useAppSelector((state) => state.ui.selectedSeq);
  const selectedEntityId = useAppSelector((state) => state.ui.selectedEntityId);
  const laneData = useLiveLaneData(selectedConvId);

  if (!selectedConvId) {
    return (
      <div className="timeline-empty-state">
        <h2>📊 Timeline View</h2>
        <p>Select a conversation to view its timeline.</p>
      </div>
    );
  }

  if (laneData.isLoading) {
    return (
      <div className="timeline-loading-state">
        <p>Loading timeline...</p>
      </div>
    );
  }

  return (
    <div className="timeline-page">
      <div className="timeline-page-header">
        <h2>📊 Timeline</h2>
        <div className="timeline-header-meta">
          <span>{laneData.turns.length} turns</span>
          <span>{laneData.events.length} events</span>
          <span>{laneData.entities.length} entities</span>
        </div>
      </div>

      <TimelineLanes
        turns={laneData.turns}
        events={laneData.events}
        entities={laneData.entities}
        isLive={false}
        selectedTurnId={selectedTurnId ?? undefined}
        selectedEventSeq={selectedEventSeq ?? undefined}
        selectedEntityId={selectedEntityId ?? undefined}
        onTurnSelect={(turn) => {
          dispatch(selectSession(turn.session_id));
          dispatch(selectTurn(turn.turn_id));
        }}
        onEventSelect={(event) => {
          dispatch(selectEvent(event.seq));
        }}
        onEntitySelect={(entity) => dispatch(selectEntity(entity.id))}
      />
    </div>
  );
}

export default TimelinePage;
