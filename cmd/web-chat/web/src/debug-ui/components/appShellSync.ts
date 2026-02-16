export interface UrlSyncState {
  convFromURL: string | null;
  desiredSession: string | null | undefined;
  desiredTurn: string | null | undefined;
  runFromURL: string | null;
  artifactsRootFromURL: string | null;
  turnsDBFromURL: string | null;
  timelineDBFromURL: string | null;
  selectedConvId: string | null;
  selectedSessionId: string | null;
  selectedTurnId: string | null;
  selectedRunId: string | null;
  offlineArtifactsRoot: string;
  offlineTurnsDB: string;
  offlineTimelineDB: string;
}

export function shouldDelayUrlSync(state: UrlSyncState): boolean {
  return (
    (!!state.convFromURL && state.convFromURL !== state.selectedConvId) ||
    (!!state.desiredSession && state.desiredSession !== state.selectedSessionId) ||
    (!!state.desiredTurn && state.desiredTurn !== state.selectedTurnId) ||
    (!!state.runFromURL && state.runFromURL !== state.selectedRunId) ||
    (state.artifactsRootFromURL !== null &&
      state.artifactsRootFromURL !== state.offlineArtifactsRoot) ||
    (state.turnsDBFromURL !== null &&
      state.turnsDBFromURL !== state.offlineTurnsDB) ||
    (state.timelineDBFromURL !== null &&
      state.timelineDBFromURL !== state.offlineTimelineDB)
  );
}
