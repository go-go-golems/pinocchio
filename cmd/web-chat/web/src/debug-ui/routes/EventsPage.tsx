import { useState } from 'react';
import { useGetEventsQuery } from '../api/debugApi';
import { EventCard } from '../components/EventCard';
import { EventInspector } from '../components/EventInspector';
import { useAppSelector } from '../store/hooks';
import type { SemEvent } from '../types';

export function EventsPage() {
  const selectedConvId = useAppSelector((state) => state.ui.selectedConvId);
  const [selectedEvent, setSelectedEvent] = useState<SemEvent | null>(null);

  const { data: eventsData, isLoading } = useGetEventsQuery(
    { convId: selectedConvId ?? '' },
    { skip: !selectedConvId }
  );

  if (!selectedConvId) {
    return (
      <div className="events-empty-state">
        <h2>⚡ Events</h2>
        <p>Select a conversation to view its events.</p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="events-loading-state">
        <p>Loading events...</p>
      </div>
    );
  }

  const events = eventsData?.events ?? [];

  return (
    <div className="events-page">
      <div className="events-page-header">
        <h2>⚡ Events</h2>
        <div className="events-header-meta">
          <span>{events.length} events</span>
          <span>Buffer: {eventsData?.buffer_capacity ?? 0}</span>
        </div>
      </div>

      <div className="events-layout">
        {/* Event list */}
        <div className="events-list">
          {events.length === 0 ? (
            <div className="empty-list">No events recorded</div>
          ) : (
            events.map((event) => (
              <EventCard
                key={event.id}
                event={event}
                onClick={() => setSelectedEvent(event)}
                selected={selectedEvent?.id === event.id}
              />
            ))
          )}
        </div>

        {/* Event detail */}
        {selectedEvent && (
          <div className="event-detail">
            <EventInspector event={selectedEvent} />
          </div>
        )}
      </div>
    </div>
  );
}

export default EventsPage;
