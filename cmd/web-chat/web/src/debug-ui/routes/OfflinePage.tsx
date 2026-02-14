import { useEffect, useMemo, useState } from 'react';
import { useGetRunDetailQuery } from '../api/debugApi';
import { EventInspector } from '../components/EventInspector';
import { TimelineEntityCard } from '../components/TimelineEntityCard';
import { TimelineLanes } from '../components/TimelineLanes';
import { TurnInspector } from '../components/TurnInspector';
import { useAppSelector } from '../store/hooks';
import type { OfflineInspectorData } from './offlineData';
import { buildTurnDetail, parseOfflineInspectorData } from './offlineData';

function stringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

const EMPTY_INSPECTOR_DATA: OfflineInspectorData = {
  convID: '',
  sessionID: '',
  turns: [],
  events: [],
  entities: [],
};

export function OfflinePage() {
  const [activeTab, setActiveTab] = useState<'turns' | 'events' | 'timeline' | 'raw'>('turns');
  const [selectedTurnId, setSelectedTurnId] = useState<string | null>(null);
  const [selectedEventSeq, setSelectedEventSeq] = useState<number | null>(null);
  const [selectedEntityId, setSelectedEntityId] = useState<string | null>(null);
  const selectedRunId = useAppSelector((state) => state.ui.selectedRunId);
  const offline = useAppSelector((state) => state.ui.offline);

  const hasSource = useMemo(
    () =>
      offline.artifactsRoot.trim().length > 0 ||
      offline.turnsDB.trim().length > 0 ||
      offline.timelineDB.trim().length > 0,
    [offline.artifactsRoot, offline.timelineDB, offline.turnsDB]
  );

  const { data, isLoading, error } = useGetRunDetailQuery(
    {
      runId: selectedRunId ?? '',
      artifactsRoot: offline.artifactsRoot || undefined,
      turnsDB: offline.turnsDB || undefined,
      timelineDB: offline.timelineDB || undefined,
      limit: 500,
    },
    { skip: !hasSource || !selectedRunId }
  );

  const runID = data?.run_id ?? null;
  const inspectorData = useMemo(
    () => (data ? parseOfflineInspectorData(data) : EMPTY_INSPECTOR_DATA),
    [data]
  );
  const selectedEvent = useMemo(
    () => inspectorData.events.find((event) => event.seq === selectedEventSeq) ?? null,
    [inspectorData.events, selectedEventSeq]
  );
  const turnDetail = useMemo(
    () =>
      selectedTurnId
        ? buildTurnDetail(inspectorData.turns, inspectorData.convID, inspectorData.sessionID, selectedTurnId)
        : null,
    [inspectorData.convID, inspectorData.sessionID, inspectorData.turns, selectedTurnId]
  );

  useEffect(() => {
    if (!runID) {
      return;
    }
    setActiveTab('turns');
    setSelectedTurnId(null);
    setSelectedEventSeq(null);
    setSelectedEntityId(null);
  }, [runID]);

  useEffect(() => {
    if (inspectorData.turns.length === 0) {
      if (selectedTurnId !== null) {
        setSelectedTurnId(null);
      }
      return;
    }
    const found = selectedTurnId
      ? inspectorData.turns.some((turn) => turn.turn_id === selectedTurnId)
      : false;
    if (!found) {
      setSelectedTurnId(inspectorData.turns[0].turn_id);
    }
  }, [inspectorData.turns, selectedTurnId]);

  useEffect(() => {
    if (inspectorData.events.length === 0) {
      if (selectedEventSeq !== null) {
        setSelectedEventSeq(null);
      }
      return;
    }
    const found = selectedEventSeq !== null
      ? inspectorData.events.some((event) => event.seq === selectedEventSeq)
      : false;
    if (!found) {
      setSelectedEventSeq(inspectorData.events[0].seq);
    }
  }, [inspectorData.events, selectedEventSeq]);

  useEffect(() => {
    if (inspectorData.entities.length === 0) {
      if (selectedEntityId !== null) {
        setSelectedEntityId(null);
      }
      return;
    }
    const found = selectedEntityId
      ? inspectorData.entities.some((entity) => entity.id === selectedEntityId)
      : false;
    if (!found) {
      setSelectedEntityId(inspectorData.entities[0].id);
    }
  }, [inspectorData.entities, selectedEntityId]);

  if (!hasSource) {
    return (
      <div className="offline-empty-state">
        <h2>Offline Viewer</h2>
        <p>Enter a source in the left sidebar to inspect persisted runs.</p>
      </div>
    );
  }

  if (!selectedRunId) {
    return (
      <div className="offline-empty-state">
        <h2>Offline Viewer</h2>
        <p>Select a run from the sidebar to load detail.</p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="offline-loading-state">
        <p>Loading run detail...</p>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="offline-empty-state">
        <h2>Failed to load run detail</h2>
        <p>Check source paths and run selection.</p>
      </div>
    );
  }

  const detailKeys = Object.keys(data.detail);

  return (
    <div className="offline-page">
      <div className="offline-page-header">
        <h2>Offline Run Inspector</h2>
        <div className="offline-page-meta">
          <span>run_id: {data.run_id}</span>
          <span>kind: {data.kind}</span>
          <span>turns: {inspectorData.turns.length}</span>
          <span>events: {inspectorData.events.length}</span>
          <span>entities: {inspectorData.entities.length}</span>
          <span>detail keys: {detailKeys.length}</span>
        </div>
      </div>

      <section className="offline-detail-section">
        <h3>Inspector Timeline</h3>
        <TimelineLanes
          turns={inspectorData.turns}
          events={inspectorData.events}
          entities={inspectorData.entities}
          isLive={false}
          selectedTurnId={selectedTurnId ?? undefined}
          selectedEventSeq={selectedEventSeq ?? undefined}
          selectedEntityId={selectedEntityId ?? undefined}
          onTurnSelect={(turn) => {
            setSelectedTurnId(turn.turn_id);
            setActiveTab('turns');
          }}
          onEventSelect={(event) => {
            setSelectedEventSeq(event.seq);
            setActiveTab('events');
          }}
          onEntitySelect={(entity) => {
            setSelectedEntityId(entity.id);
            setActiveTab('timeline');
          }}
        />
      </section>

      <section className="offline-detail-section">
        <div className="offline-tab-row">
          <button
            className={`btn btn-secondary ${activeTab === 'turns' ? 'active' : ''}`}
            onClick={() => setActiveTab('turns')}
          >
            Turns
          </button>
          <button
            className={`btn btn-secondary ${activeTab === 'events' ? 'active' : ''}`}
            onClick={() => setActiveTab('events')}
          >
            Events
          </button>
          <button
            className={`btn btn-secondary ${activeTab === 'timeline' ? 'active' : ''}`}
            onClick={() => setActiveTab('timeline')}
          >
            Timeline
          </button>
          <button
            className={`btn btn-secondary ${activeTab === 'raw' ? 'active' : ''}`}
            onClick={() => setActiveTab('raw')}
          >
            Raw
          </button>
        </div>

        {activeTab === 'turns' && (
          <div className="offline-panel-body">
            {turnDetail ? (
              <TurnInspector turnDetail={turnDetail} />
            ) : (
              <div className="offline-empty-state">No turn snapshots available for this run.</div>
            )}
          </div>
        )}

        {activeTab === 'events' && (
          <div className="offline-panel-body">
            {selectedEvent ? (
              <EventInspector event={selectedEvent} />
            ) : (
              <div className="offline-empty-state">No events available for this run.</div>
            )}
          </div>
        )}

        {activeTab === 'timeline' && (
          <div className="offline-panel-body offline-entity-grid">
            {inspectorData.entities.length === 0 ? (
              <div className="offline-empty-state">No timeline entities available for this run.</div>
            ) : (
              inspectorData.entities.map((entity) => (
                <TimelineEntityCard
                  key={entity.id}
                  entity={entity}
                  selected={entity.id === selectedEntityId}
                  onClick={() => setSelectedEntityId(entity.id)}
                />
              ))
            )}
          </div>
        )}

        {activeTab === 'raw' && (
          <div className="offline-panel-body">
            <h3>Detail Payload</h3>
            <pre>{stringify(data.detail)}</pre>
          </div>
        )}
      </section>
    </div>
  );
}

export default OfflinePage;
