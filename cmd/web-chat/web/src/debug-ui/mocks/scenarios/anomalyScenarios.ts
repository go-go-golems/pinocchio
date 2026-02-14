import type { Anomaly } from '../../components/AnomalyPanel';
import {
  makeAnomalies,
  makeAnomaly,
} from '../factories';

type AnomalyScenarioArgs = {
  anomalies: Anomaly[];
  isOpen: boolean;
};

export interface AnomalyScenario {
  name: string;
  args: AnomalyScenarioArgs;
}

const baseAnomalies = makeAnomalies(5);

export const anomalyScenarios: Record<string, AnomalyScenario> = {
  default: {
    name: 'default',
    args: {
      anomalies: baseAnomalies,
      isOpen: true,
    },
  },
  closed: {
    name: 'closed',
    args: {
      anomalies: baseAnomalies,
      isOpen: false,
    },
  },
  empty: {
    name: 'empty',
    args: {
      anomalies: [],
      isOpen: true,
    },
  },
  errorsOnly: {
    name: 'errorsOnly',
    args: {
      anomalies: baseAnomalies.filter((a) => a.severity === 'error'),
      isOpen: true,
    },
  },
  manyAnomalies: {
    name: 'manyAnomalies',
    args: {
      anomalies: [
        ...baseAnomalies,
        ...makeAnomalies(5, {
          mapOverrides: (i) => ({ id: `dup_${i + 1}_${makeAnomaly({ index: i }).id}` }),
        }),
        ...makeAnomalies(5, {
          mapOverrides: (i) => ({ id: `dup2_${i + 1}_${makeAnomaly({ index: i }).id}` }),
        }),
      ],
      isOpen: true,
    },
  },
};

export function makeAnomalyScenario(name: keyof typeof anomalyScenarios): AnomalyScenario {
  return anomalyScenarios[name];
}
