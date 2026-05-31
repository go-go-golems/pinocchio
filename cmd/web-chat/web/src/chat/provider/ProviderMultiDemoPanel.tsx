import { selectOverlay, selectTimelineEntities, useChatClient, useAppSelector as useChatProviderSelector } from '@go-go-golems/chat-provider';
import { useCallback, useEffect, useState } from 'react';

export function ProviderMultiDemoPanel({ name, prompt }: { name: string; prompt: string }) {
  const client = useChatClient();
  const overlay = useChatProviderSelector(selectOverlay);
  const entities = useChatProviderSelector(selectTimelineEntities);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void client.connect();
  }, [client]);

  const send = useCallback(async () => {
    setBusy(true);
    try {
      await client.send(prompt);
    } finally {
      setBusy(false);
    }
  }, [client, prompt]);

  return (
    <section data-testid={`multi-panel-${name}`} data-session-id={overlay.sessionId} data-entity-count={entities.length}>
      <h2>{name}</h2>
      <p>
        session: <code data-testid={`multi-session-${name}`}>{overlay.sessionId || '(pending)'}</code>
      </p>
      <p>
        ws: <span data-testid={`multi-ws-${name}`}>{overlay.wsStatus || 'disconnected'}</span>
      </p>
      <p>
        run: <span data-testid={`multi-run-${name}`}>{overlay.runStatus || 'idle'}</span>
      </p>
      <p>
        entities: <span data-testid={`multi-count-${name}`}>{entities.length}</span>
      </p>
      <button type="button" data-testid={`multi-send-${name}`} onClick={send} disabled={busy}>
        Send {name}
      </button>
      <ol data-testid={`multi-timeline-${name}`}>
        {entities.map((entity) => (
          <li key={entity.id}>
            <strong>{entity.kind}</strong> {String(entity.props?.role ?? entity.props?.status ?? '')}{' '}
            {String(entity.props?.content ?? entity.props?.prompt ?? '').slice(0, 80)}
          </li>
        ))}
      </ol>
    </section>
  );
}
