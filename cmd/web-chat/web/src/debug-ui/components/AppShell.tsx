import { useEffect, useState } from 'react';
import {
  NavLink,
  Outlet,
  useLocation,
  useParams,
  useSearchParams,
} from 'react-router-dom';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import {
  selectConversation,
  selectRun,
  selectSession,
  selectTurn,
  setOfflineConfig,
} from '../store/uiSlice';
import { type Anomaly, AnomalyPanel } from './AnomalyPanel';
import { FilterBar, type FilterState } from './FilterBar';
import { OfflineSourcesPanel } from './OfflineSourcesPanel';
import { SessionList } from './SessionList';

const defaultFilters: FilterState = {
  blockKinds: [],
  eventTypes: [],
  searchQuery: '',
  showEmpty: true,
};

export interface AppShellProps {
  /** Mock anomalies for Storybook */
  anomalies?: Anomaly[];
}

export function AppShell({ anomalies = [] }: AppShellProps) {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [filterOpen, setFilterOpen] = useState(false);
  const [anomalyOpen, setAnomalyOpen] = useState(false);
  const [filters, setFilters] = useState<FilterState>(defaultFilters);

  const dispatch = useAppDispatch();
  const selectedConvId = useAppSelector((state) => state.ui.selectedConvId);
  const selectedSessionId = useAppSelector((state) => state.ui.selectedSessionId);
  const selectedTurnId = useAppSelector((state) => state.ui.selectedTurnId);
  const selectedRunId = useAppSelector((state) => state.ui.selectedRunId);
  const offline = useAppSelector((state) => state.ui.offline);
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const { sessionId, turnId } = useParams();
  const offlineRoute = location.pathname.startsWith('/offline');

  const activeFilterCount =
    filters.blockKinds.length +
    filters.eventTypes.length +
    (filters.searchQuery ? 1 : 0);

  useEffect(() => {
    const convFromURL = searchParams.get('conv');
    const sessionFromURL = searchParams.get('session');
    const turnFromURL = searchParams.get('turn');
    const runFromURL = searchParams.get('run');
    const artifactsRootFromURL = searchParams.get('artifacts_root');
    const turnsDBFromURL = searchParams.get('turns_db');
    const timelineDBFromURL = searchParams.get('timeline_db');

    if (convFromURL && convFromURL !== selectedConvId) {
      dispatch(selectConversation(convFromURL));
      return;
    }

    if (!convFromURL && !selectedConvId) {
      const persistedConv = window.localStorage.getItem('debug-ui:selected-conv');
      if (persistedConv) {
        dispatch(selectConversation(persistedConv));
      }
    }

    if (sessionId && sessionId !== selectedSessionId) {
      dispatch(selectSession(sessionId));
      return;
    }
    if (!sessionId && sessionFromURL && sessionFromURL !== selectedSessionId) {
      dispatch(selectSession(sessionFromURL));
      return;
    }

    if (turnId && turnId !== selectedTurnId) {
      dispatch(selectTurn(turnId));
      return;
    }
    if (!turnId && turnFromURL && turnFromURL !== selectedTurnId) {
      dispatch(selectTurn(turnFromURL));
    }

    if (runFromURL && runFromURL !== selectedRunId) {
      dispatch(selectRun(runFromURL));
    }

    const nextArtifactsRoot =
      artifactsRootFromURL ??
      (!offline.artifactsRoot
        ? window.localStorage.getItem('debug-ui:offline:artifacts_root')
        : null);
    const nextTurnsDB =
      turnsDBFromURL ??
      (!offline.turnsDB ? window.localStorage.getItem('debug-ui:offline:turns_db') : null);
    const nextTimelineDB =
      timelineDBFromURL ??
      (!offline.timelineDB
        ? window.localStorage.getItem('debug-ui:offline:timeline_db')
        : null);

    const patch: Partial<typeof offline> = {};
    if (nextArtifactsRoot !== null && nextArtifactsRoot !== offline.artifactsRoot) {
      patch.artifactsRoot = nextArtifactsRoot;
    }
    if (nextTurnsDB !== null && nextTurnsDB !== offline.turnsDB) {
      patch.turnsDB = nextTurnsDB;
    }
    if (nextTimelineDB !== null && nextTimelineDB !== offline.timelineDB) {
      patch.timelineDB = nextTimelineDB;
    }
    if (Object.keys(patch).length > 0) {
      dispatch(setOfflineConfig(patch));
    }
  }, [
    dispatch,
    offline.artifactsRoot,
    offline.timelineDB,
    offline.turnsDB,
    searchParams,
    selectedConvId,
    selectedRunId,
    selectedSessionId,
    selectedTurnId,
    sessionId,
    turnId,
  ]);

  useEffect(() => {
    const nextParams = new URLSearchParams(searchParams);
    let changed = false;

    const applyParam = (key: string, value: string | null) => {
      const current = nextParams.get(key);
      if (!value) {
        if (current !== null) {
          nextParams.delete(key);
          changed = true;
        }
        return;
      }
      if (current !== value) {
        nextParams.set(key, value);
        changed = true;
      }
    };

    applyParam('conv', selectedConvId);
    applyParam('session', selectedSessionId ?? sessionId ?? null);
    applyParam('turn', selectedTurnId ?? turnId ?? null);
    applyParam('run', selectedRunId);
    applyParam('artifacts_root', offline.artifactsRoot || null);
    applyParam('turns_db', offline.turnsDB || null);
    applyParam('timeline_db', offline.timelineDB || null);

    if (selectedConvId) {
      window.localStorage.setItem('debug-ui:selected-conv', selectedConvId);
    }
    window.localStorage.setItem('debug-ui:offline:artifacts_root', offline.artifactsRoot);
    window.localStorage.setItem('debug-ui:offline:turns_db', offline.turnsDB);
    window.localStorage.setItem('debug-ui:offline:timeline_db', offline.timelineDB);

    if (changed) {
      setSearchParams(nextParams, { replace: true });
    }
  }, [
    offline.artifactsRoot,
    offline.timelineDB,
    offline.turnsDB,
    searchParams,
    selectedConvId,
    selectedRunId,
    selectedSessionId,
    selectedTurnId,
    sessionId,
    setSearchParams,
    turnId,
  ]);

  return (
    <div className="app-shell">
      {/* Top nav bar */}
      <header className="app-header">
        <div className="app-header-left">
          <button 
            className="btn app-btn-icon"
            onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
            title={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            {sidebarCollapsed ? '☰' : '◀'}
          </button>
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
          <NavLink
            to={{ pathname: '/offline', search: location.search }}
            className={({ isActive }) => `app-nav-link ${isActive ? 'active' : ''}`}
          >
            Offline
          </NavLink>
        </nav>

        <div className="app-header-right">
          <button
            className={`btn app-btn-icon ${activeFilterCount > 0 ? 'has-badge' : ''}`}
            onClick={() => setFilterOpen(!filterOpen)}
            title="Filters"
            disabled={offlineRoute}
          >
            🔍
            {activeFilterCount > 0 && <span className="app-btn-badge">{activeFilterCount}</span>}
          </button>
          <button
            className={`btn app-btn-icon ${anomalies.length > 0 ? 'has-badge' : ''}`}
            onClick={() => setAnomalyOpen(!anomalyOpen)}
            title="Anomalies"
            disabled={offlineRoute}
          >
            ⚠️
            {anomalies.length > 0 && <span className="app-btn-badge">{anomalies.length}</span>}
          </button>
        </div>
      </header>

      <div className="app-body">
        {/* Sidebar */}
        <aside className={`app-sidebar ${sidebarCollapsed ? 'collapsed' : ''}`}>
          {!sidebarCollapsed && (offlineRoute ? <OfflineSourcesPanel /> : <SessionList />)}
        </aside>

        {/* Main content */}
        <main className="app-main">
          {/* Breadcrumb */}
          <div className="app-breadcrumb">
            {offlineRoute ? (
              <span className="app-breadcrumb-crumb">
                {selectedRunId ? `run: ${selectedRunId}` : 'No run selected'}
              </span>
            ) : (
              <span className="app-breadcrumb-crumb">
                {selectedConvId ? `conv: ${selectedConvId.slice(0, 8)}...` : 'No conversation selected'}
              </span>
            )}
            {!offlineRoute && sessionId && (
              <>
                <span className="app-breadcrumb-sep">/</span>
                <span className="app-breadcrumb-crumb">session: {sessionId.slice(0, 8)}...</span>
              </>
            )}
          </div>

          {/* Router outlet */}
          <div className="app-main-content">
            <Outlet context={{ filters }} />
          </div>
        </main>

        {/* Filter sidebar (right) */}
        {!offlineRoute && filterOpen && (
          <aside className="app-filter-sidebar">
            <FilterBar
              filters={filters}
              onFiltersChange={setFilters}
              onClose={() => setFilterOpen(false)}
            />
          </aside>
        )}
      </div>

      {/* Anomaly panel overlay */}
      <AnomalyPanel
        anomalies={anomalies}
        isOpen={!offlineRoute && anomalyOpen}
        onClose={() => setAnomalyOpen(false)}
      />
    </div>
  );
}

export default AppShell;
