import type { OfflineRunSummary, RunDetailResponse } from '../../types';

export const mockOfflineRuns: OfflineRunSummary[] = [
  {
    run_id: 'artifact|run-2026-02-13T10-00-00Z',
    kind: 'artifact',
    display: 'run-2026-02-13T10-00-00Z',
    source_path: '/tmp/artifacts/run-2026-02-13T10-00-00Z',
    timestamp_ms: 1770986400000,
    counts: {
      turns: 2,
      event_files: 1,
      has_logs: true,
    },
  },
  {
    run_id: 'turns|conv_8a3f|sess_01',
    kind: 'turns',
    display: 'conv_8a3f / sess_01',
    source_path: '/tmp/turns.db',
    timestamp_ms: 1770987000000,
    conv_id: 'conv_8a3f',
    session_id: 'sess_01',
    counts: {
      turn_rows: 6,
      first_created_ms: 1770986400000,
      last_created_ms: 1770987000000,
    },
  },
  {
    run_id: 'timeline|conv_8a3f',
    kind: 'timeline',
    display: 'conv_8a3f',
    source_path: '/tmp/timeline.db',
    conv_id: 'conv_8a3f',
    counts: {
      version: 42,
    },
  },
];

export const mockOfflineRunDetails: Record<string, RunDetailResponse> = {
  'artifact|run-2026-02-13T10-00-00Z': {
    run_id: 'artifact|run-2026-02-13T10-00-00Z',
    kind: 'artifact',
    detail: {
      run_name: 'run-2026-02-13T10-00-00Z',
      path: '/tmp/artifacts/run-2026-02-13T10-00-00Z',
      turns: [
        {
          name: 'final_turn.yaml',
          parsed: {
            id: 'turn_01',
            blocks: [],
          },
        },
      ],
      events: [
        {
          name: 'events.ndjson',
          items: [{ type: 'llm.final', event: { type: 'llm.final' } }],
        },
      ],
      logs: [{ level: 'info', message: 'completed' }],
    },
  },
  'turns|conv_8a3f|sess_01': {
    run_id: 'turns|conv_8a3f|sess_01',
    kind: 'turns',
    detail: {
      conv_id: 'conv_8a3f',
      session_id: 'sess_01',
      source_db: '/tmp/turns.db',
      items: [
        {
          conv_id: 'conv_8a3f',
          session_id: 'sess_01',
          turn_id: 'turn_01',
          phase: 'final',
          created_at_ms: 1770986400000,
          parsed: {
            id: 'turn_01',
            blocks: [],
          },
        },
      ],
    },
  },
  'timeline|conv_8a3f': {
    run_id: 'timeline|conv_8a3f',
    kind: 'timeline',
    detail: {
      conv_id: 'conv_8a3f',
      source_db: '/tmp/timeline.db',
      snapshot: {
        convId: 'conv_8a3f',
        version: 42,
        entities: [],
      },
    },
  },
};
