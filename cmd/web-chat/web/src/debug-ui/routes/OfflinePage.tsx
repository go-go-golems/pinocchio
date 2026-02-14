import { useMemo } from 'react';
import { useGetRunDetailQuery } from '../api/debugApi';
import { useAppSelector } from '../store/hooks';

function stringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function OfflinePage() {
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
          <span>detail keys: {detailKeys.length}</span>
        </div>
      </div>

      <section className="offline-detail-section">
        <h3>Summary</h3>
        <ul>
          {detailKeys.map((key) => (
            <li key={key}>
              <code>{key}</code>
            </li>
          ))}
        </ul>
      </section>

      <section className="offline-detail-section">
        <h3>Detail Payload</h3>
        <pre>{stringify(data.detail)}</pre>
      </section>
    </div>
  );
}

export default OfflinePage;
