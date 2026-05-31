import {
  ChatProvider,
  selectOverlay,
  selectTimelineEntities,
  useChatClient,
  useAppSelector as useChatProviderSelector,
} from '@go-go-golems/chat-provider';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { basePrefixFromLocation } from '../utils/basePrefix';
import './styles/theme-default.css';
import './styles/webchat.css';

function ProviderMultiDemoPanel({ name, prompt }: { name: string; prompt: string }) {
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
      <p>session: <code data-testid={`multi-session-${name}`}>{overlay.sessionId || '(pending)'}</code></p>
      <p>ws: <span data-testid={`multi-ws-${name}`}>{overlay.wsStatus || 'disconnected'}</span></p>
      <p>run: <span data-testid={`multi-run-${name}`}>{overlay.runStatus || 'idle'}</span></p>
      <p>entities: <span data-testid={`multi-count-${name}`}>{entities.length}</span></p>
      <button type="button" data-testid={`multi-send-${name}`} onClick={send} disabled={busy}>
        Send {name}
      </button>
      <ol data-testid={`multi-timeline-${name}`}>
        {entities.map((entity) => (
          <li key={entity.id}>
            <strong>{entity.kind}</strong> {String(entity.props?.role ?? entity.props?.status ?? '')} {String(entity.props?.content ?? entity.props?.prompt ?? '').slice(0, 80)}
          </li>
        ))}
      </ol>
    </section>
  );
}

function ProviderMultiDemoInstance({ name, prompt }: { name: string; prompt: string }) {
  const basePrefix = basePrefixFromLocation();
  const config = useMemo(() => ({
    basePrefix,
    sessionIdParam: '',
    sessionStorageKey: `pinocchio.web-chat.multi.${name}.sessionId`,
    createSessionBody: () => ({ profile: 'default' }),
    sendMessageBody: ({ prompt: text }: { prompt: string }) => ({ prompt: text, profile: 'default' }),
  }), [basePrefix, name]);

  return (
    <ChatProvider config={config}>
      <ProviderMultiDemoPanel name={name} prompt={prompt} />
    </ChatProvider>
  );
}

export function ProviderMultiDemoPage() {
  return (
    <div data-pwchat="" data-part="root" data-theme="default" data-fullscreen="true">
      <header data-part="header">
        <h1>ChatProvider multi-instance smoke</h1>
      </header>
      <main data-part="main" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, padding: 16 }}>
        <ProviderMultiDemoInstance name="left" prompt="hello from left provider" />
        <ProviderMultiDemoInstance name="right" prompt="hello from right provider" />
      </main>
    </div>
  );
}
