import { useEffect } from 'react';
import { appendEvent, clear, deleteEntity, setSnapshotOrdinal, upsertEntity } from '../store/debugSlice';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectSession, setFollowStatus } from '../store/uiSlice';
import { debugWsManager, setOnFrame } from '../ws/debugWsManager';

export function useDebugTimelineFollow() {
  const dispatch = useAppDispatch();
  const follow = useAppSelector((state) => state.ui.follow);
  const sessionId = useAppSelector((state) => state.ui.selectedSessionId);

  useEffect(() => {
    if (!follow.enabled || !sessionId) {
      debugWsManager.disconnect();
      return;
    }

    // Wire frame handler
    setOnFrame((frame) => {
      const type = String(frame.type ?? '');

      if (type === 'snapshot') {
        dispatch(clear());
        const entities = Array.isArray(frame.entities) ? frame.entities : [];
        for (const entity of entities) {
          const e = entity as Record<string, unknown>;
          dispatch(upsertEntity({
            id: String(e.id ?? ''),
            kind: String(e.kind ?? ''),
            tombstone: e.tombstone === true,
            props: (e.payload ?? {}) as Record<string, unknown>,
          }));
        }
        dispatch(setSnapshotOrdinal(Number(frame.ordinal ?? 0)));
      }

      if (type === 'ui-event') {
        dispatch(appendEvent({
          name: String(frame.name ?? ''),
          ordinal: Number(frame.ordinal ?? 0),
          sessionId: String(frame.sessionId ?? ''),
          payload: (frame.payload ?? {}) as Record<string, unknown>,
          receivedAt: new Date().toISOString(),
        }));
      }
    });

    void debugWsManager.connect({
      sessionId,
      basePrefix: '',
      dispatch,
    });

    return () => {
      setOnFrame(null);
      debugWsManager.disconnect();
      dispatch(setFollowStatus('closed'));
    };
  }, [dispatch, follow.enabled, sessionId]);
}
