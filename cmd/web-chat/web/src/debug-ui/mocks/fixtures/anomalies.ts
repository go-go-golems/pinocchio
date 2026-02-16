import type { Anomaly } from '../../components/AnomalyPanel';

export const mockAnomalies: Anomaly[] = [
  {
    id: 'anom_001',
    type: 'orphan_event',
    severity: 'error',
    message: 'Event has no matching turn snapshot',
    details:
      'Event seq 1707053365400000000 references turn_id "turn_99" which does not exist in the turn store.',
    timestamp: '2026-02-06T14:32:15.000Z',
    relatedIds: { eventId: 'msg-orphan' },
  },
  {
    id: 'anom_002',
    type: 'missing_correlation',
    severity: 'warning',
    message: 'Tool result missing correlation to tool call',
    details:
      'tool.result event lacks tool_call_id field. Cannot correlate to originating tool.start event.',
    timestamp: '2026-02-06T14:32:11.200Z',
    relatedIds: { eventId: 'tc_missing' },
  },
  {
    id: 'anom_003',
    type: 'timing_outlier',
    severity: 'warning',
    message: 'Unusually long inference latency detected',
    details: 'Inference took 15.2s which is >3 standard deviations from mean (2.4s).',
    timestamp: '2026-02-06T14:32:25.000Z',
    relatedIds: { turnId: 'turn_slow' },
  },
  {
    id: 'anom_004',
    type: 'sequence_gap',
    severity: 'info',
    message: 'Gap detected in event sequence',
    details: 'Expected seq 1707053365350000000, got 1707053365400000000. 50ms gap.',
    timestamp: '2026-02-06T14:32:14.000Z',
  },
  {
    id: 'anom_005',
    type: 'schema_error',
    severity: 'error',
    message: 'Invalid block metadata schema',
    details:
      'Block at index 3 has metadata key "geppetto.usage@v1" with invalid value type. Expected object, got string.',
    timestamp: '2026-02-06T14:32:18.000Z',
    relatedIds: { turnId: 'turn_01', blockIndex: 3 },
  },
];

export const mockAppShellAnomalies: Anomaly[] = [
  {
    id: 'anom_app_001',
    type: 'orphan_event',
    severity: 'error',
    message: 'Event has no matching turn',
    timestamp: new Date().toISOString(),
  },
  {
    id: 'anom_app_002',
    type: 'timing_outlier',
    severity: 'warning',
    message: 'Slow inference detected',
    timestamp: new Date().toISOString(),
  },
];
