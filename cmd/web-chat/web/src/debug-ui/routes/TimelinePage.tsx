import { TimelineLanes } from '../components/TimelineLanes';
import { useAppSelector } from '../store/hooks';
import { useLaneData } from './useLaneData';

export function TimelinePage() {
  const sessionId = useAppSelector((state) => state.ui.selectedSessionId);
  const follow = useAppSelector((state) => state.ui.follow);
  const laneData = useLaneData();
  const isConnected = follow.status === 'connected';

  if (!sessionId) {
    return (
      <div className="timeline-empty-state">
        <h2>📊 Timeline View</h2>
        <p>Enter a session ID to view its timeline.</p>
      </div>
    );
  }

  return (
    <div className="timeline-page">
      <div className="timeline-page-header">
        <h2>📊 Timeline</h2>
        <div className="timeline-header-meta">
          <span>{laneData.entities.length} entities</span>
          <span>{laneData.events.length} events</span>
          <span className={`timeline-live-status status-${isConnected ? 'connected' : 'idle'}`}>
            live: {isConnected ? 'connected' : 'idle'}
          </span>
        </div>
      </div>

      <TimelineLanes
        entities={laneData.entities}
        events={laneData.events}
        isLive={isConnected}
      />
    </div>
  );
}

export default TimelinePage;
