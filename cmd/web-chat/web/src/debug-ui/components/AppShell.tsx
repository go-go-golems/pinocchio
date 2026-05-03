import { useEffect, useState } from 'react';
import { NavLink, Outlet, useLocation } from 'react-router-dom';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectSession, setFollowEnabled } from '../store/uiSlice';
import { useDebugTimelineFollow } from '../ws/useDebugTimelineFollow';

export function AppShell() {
  const dispatch = useAppDispatch();
  const sessionId = useAppSelector((state) => state.ui.selectedSessionId);
  const follow = useAppSelector((state) => state.ui.follow);
  const location = useLocation();
  const [inputId, setInputId] = useState(sessionId ?? '');

  useDebugTimelineFollow();

  useEffect(() => {
    if (sessionId) {
      setInputId(sessionId);
    }
  }, [sessionId]);

  const handleFollow = () => {
    const trimmed = inputId.trim();
    if (!trimmed) return;
    dispatch(selectSession(trimmed));
    dispatch(setFollowEnabled(true));
  };

  const handleDisconnect = () => {
    dispatch(setFollowEnabled(false));
  };

  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="app-header-left">
          <h1 className="app-title">🔍 Debug UI</h1>
        </div>

        <nav className="app-header-nav">
          <NavLink
            to={{ pathname: '/', search: location.search }}
            className={({ isActive }) => `app-nav-link ${isActive ? 'active' : ''}`}
            end
          >
            Overview
          </NavLink>
          <NavLink
            to={{ pathname: '/timeline', search: location.search }}
            className={({ isActive }) => `app-nav-link ${isActive ? 'active' : ''}`}
          >
            Timeline
          </NavLink>
          <NavLink
            to={{ pathname: '/events', search: location.search }}
            className={({ isActive }) => `app-nav-link ${isActive ? 'active' : ''}`}
          >
            Events
          </NavLink>
        </nav>

        <div className="app-header-right">
          <input
            type="text"
            placeholder="Session ID"
            value={inputId}
            onChange={(e) => setInputId(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') handleFollow(); }}
            className="session-input"
          />
          {follow.enabled ? (
            <button className="btn btn-disconnect" onClick={handleDisconnect}>Disconnect</button>
          ) : (
            <button className="btn btn-follow" onClick={handleFollow}>Follow</button>
          )}
          <span className={`app-follow-badge status-${follow.status}`}>
            live: {follow.status}
          </span>
        </div>
      </header>

      <div className="app-body">
        <main className="app-main">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

export default AppShell;
