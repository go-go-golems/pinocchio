import type { DebugEntity } from '../store/debugSlice';

export interface ProjectionLaneProps {
  entities: DebugEntity[];
}

export function ProjectionLane({ entities }: ProjectionLaneProps) {
  if (entities.length === 0) {
    return <div className="lane-empty">No entities.</div>;
  }

  return (
    <div className="projection-lane">
      {entities.map((entity) => (
        <div key={entity.id} className="projection-item">
          <div className="projection-item-header">
            <span className="entity-kind">{entity.kind}</span>
            <span className="entity-id">{entity.id}</span>
          </div>
          <div className="projection-item-props">
            {Object.entries(entity.props).slice(0, 3).map(([k, v]) => (
              <span key={k} className="prop-preview">{k}: {String(v)}</span>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

export default ProjectionLane;
