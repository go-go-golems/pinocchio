import React from 'react';

export interface CorrelationIdBarProps {
  convId?: string;
  sessionId?: string;
  inferenceId?: string;
  turnId?: string;
  seq?: number;
  streamId?: string;
}

export function CorrelationIdBar({
  convId,
  sessionId,
  inferenceId,
  turnId,
  seq,
  streamId,
}: CorrelationIdBarProps) {
  const chips = [
    convId && { label: 'conv', value: convId },
    sessionId && { label: 'session', value: sessionId },
    inferenceId && { label: 'inference', value: inferenceId },
    turnId && { label: 'turn', value: turnId },
    seq && { label: 'seq', value: String(seq) },
    streamId && { label: 'stream', value: streamId },
  ].filter(Boolean) as { label: string; value: string }[];

  if (chips.length === 0) return null;

  return (
    <div className="correlation-bar">
      {chips.map(({ label, value }) => (
        <CorrelationChip key={label} label={label} value={value} />
      ))}
    </div>
  );
}

interface CorrelationChipProps {
  label: string;
  value: string;
}

function CorrelationChip({ label, value }: CorrelationChipProps) {
  const copyToClipboard = () => {
    navigator.clipboard.writeText(value);
  };

  // Truncate long values
  const displayValue = value.length > 16 ? value.slice(0, 16) + '...' : value;

  return (
    <div
      className="correlation-chip"
      onClick={copyToClipboard}
      title={`Click to copy: ${value}`}
      style={{ cursor: 'pointer' }}
    >
      <label>{label}:</label>
      <span className="value">{displayValue}</span>
    </div>
  );
}

export default CorrelationIdBar;
