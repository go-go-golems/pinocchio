import { useAppSelector } from '../store/hooks';
import { useLaneData } from './useLaneData';

export function EventsPage() {
  const sessionId = useAppSelector((state) => state.ui.selectedSessionId);
  const laneData = useLaneData();

  return (
    <div className="events-page">
      <h2>⚡ Events</h2>
      {sessionId ? (
        <div className="events-list">
          {laneData.events.length === 0 ? (
            <p>No events received yet.</p>
          ) : (
            [...laneData.events].reverse().map((event, i) => (
              <div key={i} className="event-card">
                <div className="event-card-header">
                  <span className="event-name">{event.name}</span>
                  <span className="event-ordinal">#{event.ordinal}</span>
                  <span className="event-time">{event.receivedAt}</span>
                </div>
                <pre className="event-payload">{JSON.stringify(event.payload, null, 2)}</pre>
              </div>
            ))
          )}
        </div>
      ) : (
        <p>Enter a session ID to see events.</p>
      )}
    </div>
  );
}

export default EventsPage;
