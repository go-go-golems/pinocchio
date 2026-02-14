import type React from 'react';
import { useGetRunsQuery } from '../debug-api';
import { EnvelopeMetaCard } from '../debug-components';
import { useDebugSelector } from '../debug-state';

export const DebugApp: React.FC = () => {
  const offline = useDebugSelector((s) => s.debugUi.offline);
  const runs = useGetRunsQuery({
    artifactsRoot: offline.artifactsRoot || undefined,
    turnsDB: offline.turnsDB || undefined,
    timelineDB: offline.timelineDB || undefined,
    limit: 50,
  });

  return (
    <main style={{ padding: 16, display: 'grid', gap: 12 }}>
      <h2 style={{ margin: 0 }}>Debug App (Package Extraction Seed)</h2>
      <EnvelopeMetaCard
        title="Offline Source Config"
        metadata={{
          artifacts_root: offline.artifactsRoot || '(unset)',
          turns_db: offline.turnsDB || '(unset)',
          timeline_db: offline.timelineDB || '(unset)',
        }}
      />
      <EnvelopeMetaCard
        title="Runs Query State"
        metadata={{
          isLoading: runs.isLoading,
          isFetching: runs.isFetching,
          hasData: !!runs.data,
          item_count: runs.data?.items.length ?? 0,
        }}
      />
    </main>
  );
};
