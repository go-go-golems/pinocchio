import type React from 'react';
import type { TurnsEnvelope } from '../debug-contract';
import { EnvelopeMetaCard } from './EnvelopeMetaCard';

export interface TurnsEnvelopeCardProps {
  envelope: TurnsEnvelope;
}

export const TurnsEnvelopeCard: React.FC<TurnsEnvelopeCardProps> = ({ envelope }) => {
  return (
    <EnvelopeMetaCard
      title="Turns Envelope Metadata"
      metadata={{
        conv_id: envelope.conv_id,
        session_id: envelope.session_id,
        phase: envelope.phase,
        since_ms: envelope.since_ms,
        item_count: envelope.items.length,
      }}
    />
  );
};
