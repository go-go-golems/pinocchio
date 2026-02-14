import { useState } from 'react';
import {
  useGetConversationsQuery,
  useGetRunDetailQuery,
  useGetRunsQuery,
  useGetTurnsQuery,
} from '../debug-api';
import { EnvelopeMetaCard, TurnsEnvelopeCard } from '../debug-components';
import {
  selectConversation,
  selectRun,
  setOfflineConfig,
  useDebugDispatch,
  useDebugSelector,
} from '../debug-state';

const cardStyle: React.CSSProperties = {
  border: '1px solid #d0d7de',
  borderRadius: 8,
  padding: 12,
  background: '#fff',
};

export const DebugApp: React.FC = () => {
  const dispatch = useDebugDispatch();
  const [mode, setMode] = useState<'live' | 'offline'>('live');

  const selectedConvId = useDebugSelector((s) => s.debugUi.selectedConvId);
  const selectedRunId = useDebugSelector((s) => s.debugUi.selectedRunId);
  const offline = useDebugSelector((s) => s.debugUi.offline);

  const hasOfflineSource =
    !!offline.artifactsRoot.trim() || !!offline.turnsDB.trim() || !!offline.timelineDB.trim();

  const conversations = useGetConversationsQuery(undefined, {
    skip: mode !== 'live',
  });

  const turns = useGetTurnsQuery(
    { convId: selectedConvId ?? '', limit: 50 },
    {
      skip: mode !== 'live' || !selectedConvId,
    }
  );

  const runs = useGetRunsQuery(
    {
      artifactsRoot: offline.artifactsRoot || undefined,
      turnsDB: offline.turnsDB || undefined,
      timelineDB: offline.timelineDB || undefined,
      limit: 100,
    },
    {
      skip: mode !== 'offline' || !hasOfflineSource,
    }
  );

  const runDetail = useGetRunDetailQuery(
    {
      runId: selectedRunId ?? '',
      artifactsRoot: offline.artifactsRoot || undefined,
      turnsDB: offline.turnsDB || undefined,
      timelineDB: offline.timelineDB || undefined,
      limit: 200,
    },
    {
      skip: mode !== 'offline' || !selectedRunId,
    }
  );

  return (
    <main style={{ padding: 16, display: 'grid', gap: 12, background: '#f3f4f6', minHeight: '100vh' }}>
      <header style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <h2 style={{ margin: 0, fontSize: 20 }}>Pinocchio Debug Shell</h2>
        <button type="button" onClick={() => setMode('live')} disabled={mode === 'live'}>
          Live Level-2
        </button>
        <button type="button" onClick={() => setMode('offline')} disabled={mode === 'offline'}>
          Offline
        </button>
      </header>

      <EnvelopeMetaCard
        title="Mode"
        metadata={{
          mode,
          selected_conv_id: selectedConvId ?? '(none)',
          selected_run_id: selectedRunId ?? '(none)',
        }}
      />

      {mode === 'live' ? (
        <section style={cardStyle}>
          <h3 style={{ marginTop: 0 }}>Live Conversations</h3>
          {conversations.isLoading ? <p>Loading conversations...</p> : null}
          {conversations.error ? <p>Failed to load conversations</p> : null}
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            {(conversations.data?.items ?? []).map((c) => (
              <li key={c.conv_id}>
                <button type="button" onClick={() => dispatch(selectConversation(c.conv_id))}>
                  attach {c.conv_id} ({c.session_id})
                </button>
              </li>
            ))}
          </ul>
          {turns.data ? <TurnsEnvelopeCard envelope={turns.data} /> : null}
        </section>
      ) : (
        <section style={cardStyle}>
          <h3 style={{ marginTop: 0 }}>Offline Sources</h3>
          <div style={{ display: 'grid', gap: 8 }}>
            <label>
              artifacts_root
              <input
                type="text"
                value={offline.artifactsRoot}
                onChange={(e) => dispatch(setOfflineConfig({ artifactsRoot: e.target.value }))}
                style={{ width: '100%' }}
              />
            </label>
            <label>
              turns_db
              <input
                type="text"
                value={offline.turnsDB}
                onChange={(e) => dispatch(setOfflineConfig({ turnsDB: e.target.value }))}
                style={{ width: '100%' }}
              />
            </label>
            <label>
              timeline_db
              <input
                type="text"
                value={offline.timelineDB}
                onChange={(e) => dispatch(setOfflineConfig({ timelineDB: e.target.value }))}
                style={{ width: '100%' }}
              />
            </label>
          </div>

          <EnvelopeMetaCard
            title="Offline Query Status"
            metadata={{
              has_source: hasOfflineSource,
              is_loading_runs: runs.isLoading,
              run_count: runs.data?.items.length ?? 0,
              has_run_detail: !!runDetail.data,
            }}
          />

          <ul style={{ margin: 0, paddingLeft: 20 }}>
            {(runs.data?.items ?? []).map((r) => (
              <li key={r.run_id}>
                <button type="button" onClick={() => dispatch(selectRun(r.run_id))}>
                  inspect {r.kind}: {r.display}
                </button>
              </li>
            ))}
          </ul>

          {runDetail.data ? (
            <EnvelopeMetaCard
              title="Run Detail"
              metadata={{
                run_id: runDetail.data.run_id,
                kind: runDetail.data.kind,
                detail_keys: Object.keys(runDetail.data.detail).join(', '),
              }}
            />
          ) : null}
        </section>
      )}
    </main>
  );
};
