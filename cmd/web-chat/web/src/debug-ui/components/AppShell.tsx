import { useEffect, useMemo, useState } from 'react';
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
  selectSession,
  selectTurn,
} from '../store/uiSlice';
import { useDebugTimelineFollow } from '../ws/useDebugTimelineFollow';
import { type Anomaly, AnomalyPanel } from './AnomalyPanel';
import { FilterBar, type FilterState } from './FilterBar';
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
  const follow = useAppSelector((state) => state.ui.follow);
  const location = useLocation();
  const [, setSearchParams] = useSearchParams();
  const { sessionId, turnId } = useParams();
  const params = useMemo(() => new URLSearchParams(location.search), [location.search]);

  const activeFilterCount =
    filters.blockKinds.length +
    filters.eventTypes.length +
    (filters.searchQuery ? 1 : 0);

  useDebugTimelineFollow();

  const convFromURL = params.get('conv');
  const sessionFromURL = params.get('session');
  const turnFromURL = params.get('turn');
  const desiredSession = sessionId ?? sessionFromURL;
  const desiredTurn = turnId ?? turnFromURL;

  useEffect(() => {
    if (convFromURL && convFromURL !== selectedConvId) {
      dispatch(selectConversation(convFromURL));
    }

    if (!convFromURL && !selectedConvId) {
      const persistedConv = window.localStorage.getItem('debug-ui:selected-conv');
      if (persistedConv) {
        dispatch(selectConversation(persistedConv));
      }
    }

    if (desiredSession && desiredSession !== selectedSessionId) {
      dispatch(selectSession(desiredSession));
    }

    if (desiredTurn && desiredTurn !== selectedTurnId) {
      dispatch(selectTurn(desiredTurn));
    }
  }, [
    dispatch,
    desiredSession,
    desiredTurn,
    convFromURL,
    selectedConvId,
    selectedSessionId,
    selectedTurnId,
  ]);

  useEffect(() => {
    const pendingHydration =
      (!!convFromURL && convFromURL !== selectedConvId) ||
      (!!desiredSession && desiredSession !== selectedSessionId) ||
      (!!desiredTurn && desiredTurn !== selectedTurnId);

    if (pendingHydration) {
      return;
    }

    const nextParams = new URLSearchParams(location.search);
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

    if (selectedConvId) {
      window.localStorage.setItem('debug-ui:selected-conv', selectedConvId);
    }

    if (changed) {
      setSearchParams(nextParams, { replace: true });
    }
  }, [
    convFromURL,
    desiredSession,
    desiredTurn,
    location.search,
    selectedConvId,
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
        </nav>

        <div className="app-header-right">
          <span
            className={`app-follow-badge status-${follow.status}`}
            title={follow.lastError ?? 'Realtime follow status'}
          >
            live: {follow.status}
          </span>
          <button
            className={`btn app-btn-icon ${activeFilterCount > 0 ? 'has-badge' : ''}`}
            onClick={() => setFilterOpen(!filterOpen)}
            title="Filters"
          >
            🔍
            {activeFilterCount > 0 && <span className="app-btn-badge">{activeFilterCount}</span>}
          </button>
          <button
            className={`btn app-btn-icon ${anomalies.length > 0 ? 'has-badge' : ''}`}
            onClick={() => setAnomalyOpen(!anomalyOpen)}
            title="Anomalies"
          >
            ⚠️
            {anomalies.length > 0 && <span className="app-btn-badge">{anomalies.length}</span>}
          </button>
        </div>
      </header>

      <div className="app-body">
        {/* Sidebar */}
        <aside className={`app-sidebar ${sidebarCollapsed ? 'collapsed' : ''}`}>
          {!sidebarCollapsed && <SessionList />}
        </aside>

        {/* Main content */}
        <main className="app-main">
          {/* Breadcrumb */}
          <div className="app-breadcrumb">
            <span className="app-breadcrumb-crumb">
              {selectedConvId ? `conv: ${selectedConvId.slice(0, 8)}...` : 'No conversation selected'}
            </span>
            {sessionId && (
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
        {filterOpen && (
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
        isOpen={anomalyOpen}
        onClose={() => setAnomalyOpen(false)}
      />
    </div>
  );
}

export default AppShell;
