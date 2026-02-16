import type { ParsedBlock, TurnDetail, TurnPhase } from '../types';

export const TURN_PHASE_ORDER: TurnPhase[] = [
  'draft',
  'pre_inference',
  'post_inference',
  'post_tools',
  'final',
];

export interface CompareSelection {
  a: TurnPhase | null;
  b: TurnPhase | null;
}

function includesPhase(phases: TurnPhase[], value: TurnPhase | null): value is TurnPhase {
  return value !== null && phases.includes(value);
}

export function getAvailableTurnPhases(turnDetail: TurnDetail): TurnPhase[] {
  return TURN_PHASE_ORDER.filter((phase) => turnDetail.phases[phase] !== undefined);
}

export function resolveCompareSelection(
  availablePhases: TurnPhase[],
  current: CompareSelection
): CompareSelection {
  if (availablePhases.length < 2) {
    return { a: null, b: null };
  }

  const fallbackA = availablePhases[0];
  const fallbackB = availablePhases[availablePhases.length - 1] ?? availablePhases[1];

  const normalizedA = includesPhase(availablePhases, current.a) ? current.a : fallbackA;
  const normalizedB = includesPhase(availablePhases, current.b) ? current.b : fallbackB;

  if (normalizedA === normalizedB) {
    const next = availablePhases.find((phase) => phase !== normalizedA);
    return { a: normalizedA, b: next ?? null };
  }

  return { a: normalizedA, b: normalizedB };
}

export function resolveBlockSelectionIndex(
  turnDetail: TurnDetail,
  phase: TurnPhase,
  block: ParsedBlock
): number | null {
  const phaseData = turnDetail.phases[phase];
  if (!phaseData) {
    return null;
  }
  const blocks = phaseData.turn.blocks;
  if (block.id) {
    const byID = blocks.findIndex((candidate) => candidate.id === block.id);
    if (byID >= 0) {
      return byID;
    }
  }
  if (block.index >= 0 && block.index < blocks.length) {
    return block.index;
  }
  return blocks.findIndex(
    (candidate) =>
      candidate.kind === block.kind &&
      JSON.stringify(candidate.payload) === JSON.stringify(block.payload) &&
      JSON.stringify(candidate.metadata) === JSON.stringify(block.metadata)
  );
}

