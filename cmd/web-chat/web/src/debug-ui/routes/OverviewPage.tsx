import { TimelineLanes } from '../components/TimelineLanes';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectSession, setFollowEnabled } from '../store/uiSlice';
import { useLaneData } from './useLaneData';

export function OverviewPage() {
  const dispatch = useAppDispatch();
  const sessionId = useAppSelector((state) => state.ui.selectedSessionId);
  const follow = useAppSelector((state) => state.ui.follow);
  const laneData = useLaneData();
  const isConnected = follow.status === 'connected';

  return (
    <div className="overview-page">
      <div className="overview-conv-header">
        <h2>Debug UI</h2>
        <div className="overview-conv-meta">
          <span>Session: {sessionId || '—'}</span>
          <span>Entities: {laneData.entities.length}</span>
          <span>Events: {laneData.events.length}</span>
          <span className={`timeline-live-status status-${isConnected ? 'connected' : 'idle'}`}>
            live: {isConnected ? 'connected' : 'idle'}
          </span>
        </div>
      </div>

      {!sessionId ? (
        <div className="overview-empty-state">
          <h2>👈 Enter a session ID</h2>
          <p>Type a session ID above and click Follow.</p>
        </div>
      ) : (
        <div className="overview-timeline-section">
          <TimelineLanes
            entities={laneData.entities}
            events={laneData.events}
            isLive={isConnected}
          />
        </div>
      )}
    </div>
  );
}

export default OverviewPage;
