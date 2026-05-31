import { logWarn } from '../../utils/logger';

export function setSessionIdInLocation(sessionId: string | null) {
  try {
    const u = new URL(window.location.href);
    if (!sessionId) u.searchParams.delete('sessionId');
    else u.searchParams.set('sessionId', sessionId);
    window.history.replaceState({}, '', u.toString());
  } catch (err) {
    logWarn('setSessionIdInLocation failed', { scope: 'setSessionIdInLocation' }, err);
  }
}
