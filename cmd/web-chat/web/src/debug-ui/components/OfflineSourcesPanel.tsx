import { useMemo } from 'react';
import { useGetRunsQuery } from '../api/debugApi';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectRun, setOfflineConfig } from '../store/uiSlice';

export function OfflineSourcesPanel() {
  const dispatch = useAppDispatch();
  const selectedRunId = useAppSelector((state) => state.ui.selectedRunId);
  const offline = useAppSelector((state) => state.ui.offline);

  const hasSource = useMemo(
    () =>
      offline.artifactsRoot.trim().length > 0 ||
      offline.turnsDB.trim().length > 0 ||
      offline.timelineDB.trim().length > 0,
    [offline.artifactsRoot, offline.timelineDB, offline.turnsDB]
  );

  const { data, isLoading, error } = useGetRunsQuery(
    {
      artifactsRoot: offline.artifactsRoot || undefined,
      turnsDB: offline.turnsDB || undefined,
      timelineDB: offline.timelineDB || undefined,
      limit: 200,
    },
    { skip: !hasSource }
  );

  const updateSource = (
    key: 'artifactsRoot' | 'turnsDB' | 'timelineDB',
    value: string
  ) => {
    dispatch(setOfflineConfig({ [key]: value }));
    dispatch(selectRun(null));
  };

  return (
    <div className="offline-sources-panel">
      <div className="offline-sources-header">
        <h3>Offline Sources</h3>
      </div>

      <div className="offline-sources-form">
        <label>
          artifacts_root
          <input
            type="text"
            value={offline.artifactsRoot}
            onChange={(event) => updateSource('artifactsRoot', event.target.value)}
            placeholder="/path/to/artifacts"
          />
        </label>

        <label>
          turns_db
          <input
            type="text"
            value={offline.turnsDB}
            onChange={(event) => updateSource('turnsDB', event.target.value)}
            placeholder="/path/to/turns.db"
          />
        </label>

        <label>
          timeline_db
          <input
            type="text"
            value={offline.timelineDB}
            onChange={(event) => updateSource('timelineDB', event.target.value)}
            placeholder="/path/to/timeline.db"
          />
        </label>
      </div>

      {!hasSource ? (
        <div className="offline-hint">
          Set at least one source path to load offline runs.
        </div>
      ) : null}

      {isLoading ? (
        <div className="offline-hint">Loading runs...</div>
      ) : null}

      {error ? (
        <div className="offline-error">Failed to load offline runs.</div>
      ) : null}

      <div className="offline-run-list">
        {(data?.items ?? []).map((run) => (
          <button
            key={run.run_id}
            type="button"
            className={`offline-run-item ${selectedRunId === run.run_id ? 'selected' : ''}`}
            onClick={() => dispatch(selectRun(run.run_id))}
          >
            <div className="offline-run-title">{run.display || run.run_id}</div>
            <div className="offline-run-meta">
              <span>{run.kind}</span>
              {run.timestamp_ms ? <span>{new Date(run.timestamp_ms).toISOString()}</span> : null}
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}

export default OfflineSourcesPanel;
