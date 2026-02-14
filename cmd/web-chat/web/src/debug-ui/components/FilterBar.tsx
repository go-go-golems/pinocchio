
import type { BlockKind } from '../types';
import { getBlockPresentation } from '../ui/presentation/blocks';

export interface FilterState {
  blockKinds: BlockKind[];
  eventTypes: string[];
  timeRange?: { start: number; end: number };
  searchQuery: string;
  showEmpty: boolean;
}

export interface FilterBarProps {
  filters: FilterState;
  onFiltersChange: (filters: FilterState) => void;
  onClose?: () => void;
}

const ALL_BLOCK_KINDS: BlockKind[] = ['system', 'user', 'llm_text', 'tool_call', 'tool_use', 'reasoning'];
const ALL_EVENT_TYPES = ['llm.start', 'llm.delta', 'llm.final', 'tool.start', 'tool.result', 'tool.done', 'log'];

export function FilterBar({ filters, onFiltersChange, onClose }: FilterBarProps) {
  const toggleBlockKind = (kind: BlockKind) => {
    const current = filters.blockKinds;
    const updated = current.includes(kind)
      ? current.filter(k => k !== kind)
      : [...current, kind];
    onFiltersChange({ ...filters, blockKinds: updated });
  };

  const toggleEventType = (type: string) => {
    const current = filters.eventTypes;
    const updated = current.includes(type)
      ? current.filter(t => t !== type)
      : [...current, type];
    onFiltersChange({ ...filters, eventTypes: updated });
  };

  const setSearchQuery = (query: string) => {
    onFiltersChange({ ...filters, searchQuery: query });
  };

  const toggleShowEmpty = () => {
    onFiltersChange({ ...filters, showEmpty: !filters.showEmpty });
  };

  const clearAll = () => {
    onFiltersChange({
      blockKinds: [],
      eventTypes: [],
      timeRange: undefined,
      searchQuery: '',
      showEmpty: true,
    });
  };

  const selectAllBlocks = () => {
    onFiltersChange({ ...filters, blockKinds: [...ALL_BLOCK_KINDS] });
  };

  const selectAllEvents = () => {
    onFiltersChange({ ...filters, eventTypes: [...ALL_EVENT_TYPES] });
  };

  const activeFilterCount = 
    filters.blockKinds.length + 
    filters.eventTypes.length + 
    (filters.searchQuery ? 1 : 0) +
    (filters.timeRange ? 1 : 0);

  return (
    <div className="filter-bar">
      {/* Header */}
      <div className="filter-header">
        <div className="filter-title">
          <h3>Filters</h3>
          {activeFilterCount > 0 && (
            <span className="filter-count">{activeFilterCount} active</span>
          )}
        </div>
        <div className="filter-actions">
          <button className="btn btn-ghost" onClick={clearAll}>Clear All</button>
          {onClose && (
            <button className="btn btn-ghost" onClick={onClose}>✕</button>
          )}
        </div>
      </div>

      {/* Search */}
      <div className="filter-section">
        <label className="filter-label">Search</label>
        <input
          type="text"
          className="filter-input"
          placeholder="Search blocks, events, metadata..."
          value={filters.searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
      </div>

      {/* Block kinds */}
      <div className="filter-section">
        <div className="filter-section-header">
          <label className="filter-label">Block Kinds</label>
          <button className="btn btn-ghost text-xs" onClick={selectAllBlocks}>Select All</button>
        </div>
        <div className="filter-chips">
          {ALL_BLOCK_KINDS.map(kind => (
            <FilterChip
              key={kind}
              label={kind}
              icon={getBlockPresentation(kind).icon}
              active={filters.blockKinds.includes(kind)}
              onClick={() => toggleBlockKind(kind)}
              colorClass={`kind-${kind}`}
            />
          ))}
        </div>
      </div>

      {/* Event types */}
      <div className="filter-section">
        <div className="filter-section-header">
          <label className="filter-label">Event Types</label>
          <button className="btn btn-ghost text-xs" onClick={selectAllEvents}>Select All</button>
        </div>
        <div className="filter-chips">
          {ALL_EVENT_TYPES.map(type => (
            <FilterChip
              key={type}
              label={type}
              active={filters.eventTypes.includes(type)}
              onClick={() => toggleEventType(type)}
              colorClass={`event-${type.replace('.', '-')}`}
            />
          ))}
        </div>
      </div>

      {/* Options */}
      <div className="filter-section">
        <label className="filter-label">Options</label>
        <label className="filter-checkbox">
          <input
            type="checkbox"
            checked={filters.showEmpty}
            onChange={toggleShowEmpty}
          />
          <span>Show empty phases/lanes</span>
        </label>
      </div>
    </div>
  );
}

interface FilterChipProps {
  label: string;
  icon?: string;
  active: boolean;
  onClick: () => void;
  colorClass?: string;
}

function FilterChip({ label, icon, active, onClick, colorClass }: FilterChipProps) {
  return (
    <button
      className={`filter-chip ${active ? 'active' : ''} ${colorClass || ''}`}
      onClick={onClick}
    >
      {icon && <span className="chip-icon">{icon}</span>}
      <span className="chip-label">{label}</span>
    </button>
  );
}

export default FilterBar;
