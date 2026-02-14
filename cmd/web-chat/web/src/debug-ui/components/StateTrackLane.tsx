
import type { TurnSnapshot } from '../types';
import { formatPhaseShort } from '../ui/format/phase';
import { formatTimeShort } from '../ui/format/time';
import { getBlockPresentation } from '../ui/presentation/blocks';

export interface StateTrackLaneProps {
  turns: TurnSnapshot[];
  selectedTurnId?: string;
  onTurnSelect?: (turn: TurnSnapshot) => void;
}

export function StateTrackLane({ turns, selectedTurnId, onTurnSelect }: StateTrackLaneProps) {
  if (turns.length === 0) {
    return (
      <div className="empty-lane">
        <span className="text-muted">No turn snapshots</span>
      </div>
    );
  }

  return (
    <div className="state-track-lane">
      {turns.map((turn) => (
        <TurnCard
          key={`${turn.turn_id}-${turn.phase}`}
          turn={turn}
          selected={turn.turn_id === selectedTurnId}
          onClick={() => onTurnSelect?.(turn)}
        />
      ))}
    </div>
  );
}

interface TurnCardProps {
  turn: TurnSnapshot;
  selected?: boolean;
  onClick?: () => void;
}

function TurnCard({ turn, selected, onClick }: TurnCardProps) {
  const { turn_id, session_id, phase, turn: turnData, created_at_ms } = turn;
  const blockCount = turnData.blocks.length;
  const time = formatTimeShort(created_at_ms);

  // Get block kind summary
  const kindCounts = turnData.blocks.reduce((acc, block) => {
    acc[block.kind] = (acc[block.kind] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  return (
    <div
      className={`turn-card ${selected ? 'selected' : ''}`}
      onClick={onClick}
    >
      <div className="turn-card-header">
        <span className={`phase-badge phase-${phase}`}>{formatPhaseShort(phase)}</span>
        <span className="turn-time">{time}</span>
      </div>

      <div className="turn-card-id">
        <span className="turn-id" title={turn_id}>
          {turn_id.slice(0, 12)}
        </span>
        <span className="session-id" title={session_id}>
          sess:{session_id.slice(0, 8)}
        </span>
      </div>

      <div className="turn-card-blocks">
        {Object.entries(kindCounts).map(([kind, count]) => (
          <span key={kind} className={`block-chip block-kind-${kind}`}>
            {getBlockPresentation(kind).icon} {count}
          </span>
        ))}
      </div>

      <div className="turn-card-footer">
        <span className="block-count">{blockCount} blocks</span>
      </div>
    </div>
  );
}

export default StateTrackLane;
