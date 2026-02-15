import { useEffect } from 'react';
import { basePrefixFromLocation } from '../../utils/basePrefix';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { setFollowStatus } from '../store/uiSlice';
import { debugTimelineWsManager } from './debugTimelineWsManager';

export function useDebugTimelineFollow() {
  const dispatch = useAppDispatch();
  const selectedConvId = useAppSelector((state) => state.ui.selectedConvId);
  const follow = useAppSelector((state) => state.ui.follow);
  const followConvId = follow.targetConvId ?? selectedConvId;
  const reconnectToken = follow.reconnectToken;

  useEffect(() => {
    if (!follow.enabled || !followConvId) {
      debugTimelineWsManager.disconnect();
      return;
    }

    if (reconnectToken > 0) {
      debugTimelineWsManager.disconnect();
    }

    const basePrefix = basePrefixFromLocation();
    void debugTimelineWsManager.connect({
      convId: followConvId,
      basePrefix,
      dispatch,
    });

    return () => {
      debugTimelineWsManager.disconnect();
      dispatch(setFollowStatus('closed'));
    };
  }, [dispatch, follow.enabled, reconnectToken, followConvId]);
}
