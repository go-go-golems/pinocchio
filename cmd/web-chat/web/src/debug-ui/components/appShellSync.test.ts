import { describe, expect, it } from 'vitest';
import { shouldDelayUrlSync, type UrlSyncState } from './appShellSync';

const baseState: UrlSyncState = {
  convFromURL: null,
  desiredSession: null,
  desiredTurn: null,
  runFromURL: null,
  artifactsRootFromURL: null,
  turnsDBFromURL: null,
  timelineDBFromURL: null,
  selectedConvId: null,
  selectedSessionId: null,
  selectedTurnId: null,
  selectedRunId: null,
  offlineArtifactsRoot: '',
  offlineTurnsDB: '',
  offlineTimelineDB: '',
};

describe('appShell url sync guard', () => {
  it('waits while conversation/session/turn hydration is still pending', () => {
    expect(
      shouldDelayUrlSync({
        ...baseState,
        convFromURL: 'conv-1',
      })
    ).toBe(true);

    expect(
      shouldDelayUrlSync({
        ...baseState,
        desiredSession: 'session-1',
      })
    ).toBe(true);

    expect(
      shouldDelayUrlSync({
        ...baseState,
        desiredTurn: 'turn-1',
      })
    ).toBe(true);
  });

  it('waits while offline source params have not yet hydrated store state', () => {
    expect(
      shouldDelayUrlSync({
        ...baseState,
        artifactsRootFromURL: '/tmp/artifacts',
      })
    ).toBe(true);

    expect(
      shouldDelayUrlSync({
        ...baseState,
        turnsDBFromURL: '/tmp/turns.db',
      })
    ).toBe(true);

    expect(
      shouldDelayUrlSync({
        ...baseState,
        timelineDBFromURL: '/tmp/timeline.db',
      })
    ).toBe(true);
  });

  it('allows URL writes when state and URL are already aligned', () => {
    expect(
      shouldDelayUrlSync({
        ...baseState,
        convFromURL: 'conv-1',
        desiredSession: 'session-1',
        desiredTurn: 'turn-1',
        runFromURL: 'turns|conv-1|session-1',
        artifactsRootFromURL: '/tmp/artifacts',
        turnsDBFromURL: '/tmp/turns.db',
        timelineDBFromURL: '/tmp/timeline.db',
        selectedConvId: 'conv-1',
        selectedSessionId: 'session-1',
        selectedTurnId: 'turn-1',
        selectedRunId: 'turns|conv-1|session-1',
        offlineArtifactsRoot: '/tmp/artifacts',
        offlineTurnsDB: '/tmp/turns.db',
        offlineTimelineDB: '/tmp/timeline.db',
      })
    ).toBe(false);
  });
});
