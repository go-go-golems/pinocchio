import { useEffect, useMemo, useRef, useState } from 'react';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectPhase, setComparePhases } from '../store/uiSlice';
import type { ParsedBlock, TurnDetail, TurnPhase } from '../types';
import { formatPhaseLabel } from '../ui/format/phase';
import { formatTimeShort } from '../ui/format/time';
import { BlockCard } from './BlockCard';
import { CorrelationIdBar } from './CorrelationIdBar';
import { SnapshotDiff } from './SnapshotDiff';
import {
  getAvailableTurnPhases,
  resolveBlockSelectionIndex,
  resolveCompareSelection,
  TURN_PHASE_ORDER,
} from './turnInspectorState';

export interface TurnInspectorProps {
  turnDetail: TurnDetail;
}

const PHASES: TurnPhase[] = TURN_PHASE_ORDER;

function toPhaseOrNull(value: string): TurnPhase | null {
  return PHASES.includes(value as TurnPhase) ? (value as TurnPhase) : null;
}

export function TurnInspector({ turnDetail }: TurnInspectorProps) {
  const dispatch = useAppDispatch();
  const selectedPhase = useAppSelector((state) => state.ui.selectedPhase);
  const comparePhaseA = useAppSelector((state) => state.ui.comparePhaseA);
  const comparePhaseB = useAppSelector((state) => state.ui.comparePhaseB);
  const [turnMetadataExpanded, setTurnMetadataExpanded] = useState(false);
  const [selectedBlockIndex, setSelectedBlockIndex] = useState<number | null>(null);

  const { conv_id, session_id, turn_id } = turnDetail;
  const previousTurnIdRef = useRef(turn_id);
  const phases = turnDetail.phases;
  const currentPhaseData = phases[selectedPhase];
  const availablePhases = useMemo(() => getAvailableTurnPhases(turnDetail), [turnDetail]);
  const compareSelection = useMemo(
    () =>
      resolveCompareSelection(availablePhases, {
        a: comparePhaseA,
        b: comparePhaseB,
      }),
    [availablePhases, comparePhaseA, comparePhaseB]
  );
  const compareTurnA = compareSelection.a ? phases[compareSelection.a]?.turn : undefined;
  const compareTurnB = compareSelection.b ? phases[compareSelection.b]?.turn : undefined;

  const handlePhaseClick = (phase: TurnPhase) => {
    dispatch(selectPhase(phase));
    setSelectedBlockIndex(null);
  };

  const handleCompare = (phaseA: TurnPhase | null, phaseB: TurnPhase | null) => {
    dispatch(setComparePhases({ a: phaseA, b: phaseB }));
  };

  const handleDiffBlockSelect = (block: ParsedBlock, side: 'A' | 'B') => {
    const targetPhase = side === 'A' ? compareSelection.a : compareSelection.b;
    if (!targetPhase) {
      return;
    }
    const nextIndex = resolveBlockSelectionIndex(turnDetail, targetPhase, block);
    dispatch(selectPhase(targetPhase));
    setSelectedBlockIndex(nextIndex !== null && nextIndex >= 0 ? nextIndex : null);
  };

  useEffect(() => {
    if (
      compareSelection.a !== comparePhaseA ||
      compareSelection.b !== comparePhaseB
    ) {
      dispatch(setComparePhases(compareSelection));
    }
  }, [comparePhaseA, comparePhaseB, compareSelection, dispatch]);

  useEffect(() => {
    if (previousTurnIdRef.current === turn_id) {
      return;
    }
    previousTurnIdRef.current = turn_id;
    setSelectedBlockIndex(null);
    setTurnMetadataExpanded(false);
  }, [turn_id]);

  // Get inference_id from turn metadata
  const inferenceId = currentPhaseData?.turn.metadata['geppetto.inference_id@v1'] as string | undefined;

  return (
    <div className="turn-inspector">
      {/* Correlation IDs */}
      <CorrelationIdBar
        convId={conv_id}
        sessionId={session_id}
        turnId={turn_id}
        inferenceId={inferenceId}
      />

      {/* Phase tabs */}
      <div className="tabs">
        {PHASES.map((phase) => {
          const available = phases[phase] !== undefined;
          return (
            <button
              key={phase}
              className={`tab ${selectedPhase === phase ? 'active' : ''}`}
              onClick={() => handlePhaseClick(phase)}
              disabled={!available}
              style={{ opacity: available ? 1 : 0.5 }}
            >
              {formatPhaseLabel(phase)}
              {phases[phase] && (
                <span className="text-xs text-muted" style={{ marginLeft: '4px' }}>
                  ({phases[phase]?.turn.blocks.length})
                </span>
              )}
            </button>
          );
        })}
      </div>

      {/* Compare dropdown */}
      {availablePhases.length > 1 && (
        <div className="flex items-center gap-2 mb-4">
          <span className="text-sm text-muted">Compare:</span>
          <select
            className="btn btn-secondary text-sm"
            value={compareSelection.a ?? ''}
            onChange={(e) =>
              handleCompare(
                toPhaseOrNull(e.target.value),
                compareSelection.b ?? availablePhases[availablePhases.length - 1]
              )
            }
            style={{ padding: '4px 8px' }}
          >
            <option value="">Select phase A</option>
            {availablePhases.map((p) => (
              <option key={p} value={p}>
                {formatPhaseLabel(p)}
              </option>
            ))}
          </select>
          <span className="text-muted">↔</span>
          <select
            className="btn btn-secondary text-sm"
            value={compareSelection.b ?? ''}
            onChange={(e) =>
              handleCompare(compareSelection.a ?? availablePhases[0], toPhaseOrNull(e.target.value))
            }
            style={{ padding: '4px 8px' }}
          >
            <option value="">Select phase B</option>
            {availablePhases.map((p) => (
              <option key={p} value={p}>
                {formatPhaseLabel(p)}
              </option>
            ))}
          </select>
        </div>
      )}

      {compareSelection.a && compareSelection.b && compareTurnA && compareTurnB && (
        <SnapshotDiff
          phaseA={compareSelection.a}
          phaseB={compareSelection.b}
          turnA={compareTurnA}
          turnB={compareTurnB}
          onBlockSelect={handleDiffBlockSelect}
        />
      )}

      {/* Turn metadata */}
      {currentPhaseData && (
        <div className="mb-4">
          <div className="text-xs text-muted mb-2">
            Captured: {formatTimeShort(currentPhaseData.captured_at)}
          </div>
        </div>
      )}

      {/* Turn Metadata Card */}
      {currentPhaseData && Object.keys(currentPhaseData.turn.metadata).length > 0 && (
        <TurnMetadataCard
          metadata={currentPhaseData.turn.metadata}
          expanded={turnMetadataExpanded}
          onClick={() => setTurnMetadataExpanded(!turnMetadataExpanded)}
        />
      )}

      {/* Blocks */}
      {currentPhaseData ? (
        <div className="block-list">
          <h4 className="mb-2">
            Blocks ({currentPhaseData.turn.blocks.length})
          </h4>
          {currentPhaseData.turn.blocks.map((block, idx) => (
            <BlockCard 
              key={`${block.kind}-${idx}`} 
              block={block}
              expanded={selectedBlockIndex === idx}
              onClick={() => setSelectedBlockIndex(selectedBlockIndex === idx ? null : idx)}
            />
          ))}
        </div>
      ) : (
        <div className="text-center text-muted p-4">
          No data for this phase
        </div>
      )}
    </div>
  );
}

interface TurnMetadataCardProps {
  metadata: Record<string, unknown>;
  expanded: boolean;
  onClick: () => void;
}

function TurnMetadataCard({ metadata, expanded, onClick }: TurnMetadataCardProps) {
  const entries = Object.entries(metadata);
  
  return (
    <div 
      className={`card mb-4 ${expanded ? 'selected' : ''}`}
      onClick={onClick}
      style={{ cursor: 'pointer' }}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="badge badge-purple">📋 Turn Metadata</span>
          <span className="text-xs text-muted">
            {entries.length} {entries.length === 1 ? 'field' : 'fields'}
          </span>
        </div>
        <span className="text-xs text-muted">{expanded ? '▼' : '▶'}</span>
      </div>
      
      {!expanded && (
        <div className="text-xs text-secondary mt-2" style={{ 
          overflow: 'hidden', 
          textOverflow: 'ellipsis', 
          whiteSpace: 'nowrap' 
        }}>
          {entries.slice(0, 3).map(([k]) => k).join(', ')}
          {entries.length > 3 && ` +${entries.length - 3} more`}
        </div>
      )}
      
      {expanded && (
        <div className="mt-3" onClick={(e) => e.stopPropagation()}>
          {entries.map(([key, value]) => (
            <MetadataField key={key} name={key} value={value} />
          ))}
        </div>
      )}
    </div>
  );
}

interface MetadataFieldProps {
  name: string;
  value: unknown;
}

function MetadataField({ name, value }: MetadataFieldProps) {
  const [expanded, setExpanded] = useState(false);
  const isComplex = typeof value === 'object' && value !== null;
  const displayValue = isComplex ? JSON.stringify(value) : String(value);
  const isLong = displayValue.length > 60;

  return (
    <div 
      className="metadata-field mb-2 p-2" 
      style={{ 
        background: 'var(--bg-secondary)', 
        borderRadius: '4px',
        cursor: isLong || isComplex ? 'pointer' : 'default',
      }}
      onClick={(e) => { 
        e.stopPropagation(); 
        (isLong || isComplex) && setExpanded(!expanded); 
      }}
    >
      <div className="flex items-center justify-between">
        <span className="text-xs" style={{ color: 'var(--accent-cyan)', fontFamily: 'monospace' }}>
          {name}
        </span>
        {(isLong || isComplex) && (
          <span className="text-xs text-muted">{expanded ? '▼' : '▶'}</span>
        )}
      </div>
      {expanded ? (
        <pre className="text-xs mt-1" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
          {isComplex ? JSON.stringify(value, null, 2) : displayValue}
        </pre>
      ) : (
        <div className="text-xs text-secondary mt-1" style={{ 
          overflow: 'hidden', 
          textOverflow: 'ellipsis', 
          whiteSpace: 'nowrap' 
        }}>
          {isLong ? `${displayValue.slice(0, 60)}...` : displayValue}
        </div>
      )}
    </div>
  );
}

export default TurnInspector;
