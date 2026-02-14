import React from 'react';

export interface EnvelopeMetaCardProps {
  title: string;
  metadata: Record<string, unknown>;
}

export const EnvelopeMetaCard: React.FC<EnvelopeMetaCardProps> = ({ title, metadata }) => {
  const keys = Object.keys(metadata);
  return (
    <section style={{ border: '1px solid #d0d7de', borderRadius: 8, padding: 12, background: '#f6f8fa' }}>
      <h4 style={{ margin: '0 0 8px 0', fontSize: 14 }}>{title}</h4>
      {keys.length === 0 ? (
        <p style={{ margin: 0, fontSize: 12, opacity: 0.7 }}>No metadata</p>
      ) : (
        <dl style={{ margin: 0, display: 'grid', gridTemplateColumns: 'max-content 1fr', gap: '4px 10px', fontSize: 12 }}>
          {keys.map((k) => (
            <React.Fragment key={k}>
              <dt style={{ fontWeight: 600 }}>{k}</dt>
              <dd style={{ margin: 0 }}>{String(metadata[k])}</dd>
            </React.Fragment>
          ))}
        </dl>
      )}
    </section>
  );
};
