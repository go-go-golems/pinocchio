
import type { TimelineEntity } from '../types';
import { TimelineEntityCard } from './TimelineEntityCard';

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
        <TimelineEntityCard
          key={entity.id}
          entity={entity}
          selected={entity.id === selectedEntityId}
          onClick={() => onEntitySelect?.(entity)}
          compact
        />
      ))}
    </div>
  );
}

export default ProjectionLane;
