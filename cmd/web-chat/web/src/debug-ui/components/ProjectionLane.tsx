
import type { TimelineEntity } from '../types';
import { safeStringify, truncateText } from '../ui/format/text';
import { formatTimeShort } from '../ui/format/time';
import { getTimelineKindPresentation } from '../ui/presentation/timeline';

export interface ProjectionLaneProps {
  entities: TimelineEntity[];
  selectedEntityId?: string;
  onEntitySelect?: (entity: TimelineEntity) => void;
}

export function ProjectionLane({ entities, selectedEntityId, onEntitySelect }: ProjectionLaneProps) {
  if (entities.length === 0) {
    return (
      <div className="empty-lane">
        <span className="text-muted">No timeline entities</span>
      </div>
    );
  }

  return (
    <div className="projection-lane-content">
      {entities.map((entity) => (
        <EntityCard
          key={entity.id}
          entity={entity}
          selected={entity.id === selectedEntityId}
          onClick={() => onEntitySelect?.(entity)}
        />
      ))}
    </div>
  );
}

interface EntityCardProps {
  entity: TimelineEntity;
  selected?: boolean;
  onClick?: () => void;
}

function EntityCard({ entity, selected, onClick }: EntityCardProps) {
  const { id, kind, created_at, version, props } = entity;
  const time = formatTimeShort(created_at);
  const kindInfo = getTimelineKindPresentation(kind);

  return (
    <div
      className={`entity-card ${selected ? 'selected' : ''}`}
      onClick={onClick}
      style={{ borderLeftColor: kindInfo.color }}
    >
      <div className="entity-card-header">
        <span className="entity-kind" style={{ color: kindInfo.color }}>
          {kindInfo.icon} {kind}
        </span>
        <div className="entity-header-right">
          {version && version > 1 && <span className="entity-version">v{version}</span>}
          <span className="entity-time">{time}</span>
        </div>
      </div>

      <div className="entity-card-id">{id.slice(0, 16)}</div>

      {/* Render summary based on kind */}
      {kind === 'message' && (
        <div className="entity-summary">
          <span className={`role-chip role-${String(props.role)}`}>{String(props.role)}</span>
          {Boolean(props.streaming) && <span className="streaming-badge">streaming</span>}
        </div>
      )}

      {kind === 'tool_call' && (
        <div className="entity-summary">
          <span className="tool-name">{String(props.name)}</span>
          {props.done ? (
            <span className="done-badge">✓</span>
          ) : (
            <span className="pending-badge">⏳</span>
          )}
        </div>
      )}

      {kind === 'tool_result' && (
        <div className="entity-summary">
          <span className="result-preview">
            {truncateText(safeStringify(props.result), 30)}
          </span>
        </div>
      )}
    </div>
  );
}

export default ProjectionLane;
