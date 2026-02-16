import type {
  CorrelatedNodes,
  EventInspectorProps,
  TrustCheck,
} from '../../components/EventInspector';
import type { ParsedBlock } from '../../types';
import {
  makeEvent,
  makeTimelineEntity,
  makeTurnSnapshot,
} from '../factories';

type EventInspectorScenarioArgs = Pick<EventInspectorProps, 'event' | 'correlatedNodes' | 'trustChecks'>;

export interface EventInspectorScenario {
  name: string;
  args: EventInspectorScenarioArgs;
}

function makeReferenceBlock(): ParsedBlock {
  const turn = makeTurnSnapshot({ index: 0 });
  const toolCall = turn.turn.blocks.find((b) => b.kind === 'tool_call');

  if (toolCall) {
    return toolCall;
  }

  return {
    index: 0,
    id: 'tc_fallback',
    kind: 'tool_call',
    payload: { name: 'unknown', args: {} },
    metadata: {},
  };
}

function passingChecks(): TrustCheck[] {
  return [
    { name: 'Correlation ID present', passed: true },
    { name: 'Sequence monotonic', passed: true },
    { name: 'Timestamp valid', passed: true },
  ];
}

function failingChecks(): TrustCheck[] {
  return [
    { name: 'Correlation ID present', passed: true },
    { name: 'Sequence monotonic', passed: false, message: 'Gap detected' },
    { name: 'Timestamp valid', passed: true },
    { name: 'Schema valid', passed: false, message: 'Missing required field' },
  ];
}

function correlatedNodes(): CorrelatedNodes {
  return {
    block: makeReferenceBlock(),
    prevEvent: makeEvent({ index: 0 }),
    nextEvent: makeEvent({ index: 2 }),
    entity: makeTimelineEntity({ index: 1 }),
  };
}

export const eventInspectorScenarios: Record<string, EventInspectorScenario> = {
  llmStart: {
    name: 'llmStart',
    args: { event: makeEvent({ index: 0 }) },
  },
  llmDelta: {
    name: 'llmDelta',
    args: { event: makeEvent({ index: 3 }) },
  },
  llmFinal: {
    name: 'llmFinal',
    args: { event: makeEvent({ index: 5 }) },
  },
  toolStart: {
    name: 'toolStart',
    args: { event: makeEvent({ index: 1 }) },
  },
  toolResult: {
    name: 'toolResult',
    args: { event: makeEvent({ index: 2 }) },
  },
  withCorrelatedNodes: {
    name: 'withCorrelatedNodes',
    args: {
      event: makeEvent({ index: 1 }),
      correlatedNodes: correlatedNodes(),
    },
  },
  withTrustChecks: {
    name: 'withTrustChecks',
    args: {
      event: makeEvent({ index: 0 }),
      trustChecks: passingChecks(),
    },
  },
  withFailedChecks: {
    name: 'withFailedChecks',
    args: {
      event: makeEvent({ index: 0 }),
      trustChecks: failingChecks(),
    },
  },
  fullExample: {
    name: 'fullExample',
    args: {
      event: makeEvent({
        index: 1,
        overrides: {
          data: {
            ...(makeEvent({ index: 1 }).data as Record<string, unknown>),
            session_id: 'sess_01234567',
            inference_id: 'inf_abcdef12',
            turn_id: 'turn_001',
          },
        },
      }),
      correlatedNodes: correlatedNodes(),
      trustChecks: passingChecks(),
    },
  },
};

export function makeEventInspectorScenario(
  name: keyof typeof eventInspectorScenarios
): EventInspectorScenario {
  return eventInspectorScenarios[name];
}
