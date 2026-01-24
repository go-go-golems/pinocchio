import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { wsManager } from '../ws/wsManager';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectTimelineEntities } from '../store/timelineSlice';
import { appSlice } from '../store/appSlice';

type RenderEntity = {
  id: string;
  kind: string;
  props: any;
  createdAt: number;
  updatedAt?: number;
};

function basePrefixFromLocation(): string {
  const segs = window.location.pathname.split('/').filter(Boolean);
  return segs.length > 0 ? `/${segs[0]}` : '';
}

export function ChatWidget() {
  const dispatch = useAppDispatch();
  const app = useAppSelector((s) => s.app);
  const entities = useAppSelector(selectTimelineEntities) as RenderEntity[];

  const [text, setText] = useState('');
  const basePrefix = useMemo(() => basePrefixFromLocation(), []);

  useEffect(() => {
    if (!app.convId) return;
    void wsManager.connect({
      convId: app.convId,
      basePrefix,
      dispatch,
      onStatus: (status) => dispatch(appSlice.actions.setStatus(status)),
    });
    return () => {
      wsManager.disconnect();
    };
  }, [app.convId, basePrefix, dispatch]);

  const send = useCallback(async () => {
    const prompt = text.trim();
    if (!prompt) return;
    setText('');

    dispatch(appSlice.actions.setStatus('sending...'));
    const body = app.convId ? { prompt, conv_id: app.convId } : { prompt };
    const res = await fetch(`${basePrefix}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const j = await res.json();

    const convId = (j && j.conv_id) || app.convId || '';
    const runId = (j && (j.session_id || j.run_id)) || app.runId || '';
    dispatch(appSlice.actions.setConvId(convId));
    dispatch(appSlice.actions.setRunId(runId));

    const st = (j && j.status) || 'sent';
    dispatch(appSlice.actions.setStatus(`${st}`));
  }, [app.convId, app.runId, basePrefix, dispatch, text]);

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        void send();
      }
    },
    [send],
  );

  return (
    <div
      style={{
        fontFamily: 'system-ui, sans-serif',
        margin: 0,
        padding: 0,
        background: '#0b0b0b',
        color: '#e6e6e6',
        height: '100vh',
        display: 'grid',
        gridTemplateRows: 'auto 1fr auto',
      }}
    >
      <header
        style={{
          padding: '12px 16px',
          borderBottom: '1px solid #333',
          display: 'flex',
          gap: 8,
          alignItems: 'center',
        }}
      >
        <strong>Web Chat</strong>
        <span style={{ fontSize: 12, color: '#9aa0a6', marginLeft: 'auto' }}>{app.status}</span>
      </header>

      <main style={{ overflow: 'auto', padding: 16 }}>
        {entities.map((e) => {
          if (e.kind === 'message') {
            const role = e.props?.role ?? 'assistant';
            const content = e.props?.content ?? '';
            const streaming = !!e.props?.streaming;
            return (
              <div key={e.id} style={{ padding: '10px 12px', border: '1px solid #222', borderRadius: 10, marginBottom: 8 }}>
                <div style={{ fontSize: 12, color: '#9aa0a6', marginBottom: 6 }}>
                  {role} {streaming ? '(streaming)' : ''}
                </div>
                <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{content}</pre>
              </div>
            );
          }
          if (e.kind === 'tool_call') {
            return (
              <div key={e.id} style={{ padding: '10px 12px', border: '1px solid #222', borderRadius: 10, marginBottom: 8 }}>
                <div style={{ fontSize: 12, color: '#9aa0a6' }}>tool: {e.props?.name ?? 'unknown'}</div>
                <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{JSON.stringify(e.props?.input ?? {}, null, 2)}</pre>
              </div>
            );
          }
          if (e.kind === 'tool_result') {
            return (
              <div key={e.id} style={{ padding: '10px 12px', border: '1px solid #222', borderRadius: 10, marginBottom: 8 }}>
                <div style={{ fontSize: 12, color: '#9aa0a6' }}>tool result</div>
                <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{String(e.props?.result ?? '')}</pre>
              </div>
            );
          }
          if (e.kind === 'log') {
            return (
              <div key={e.id} style={{ padding: '10px 12px', border: '1px solid #222', borderRadius: 10, marginBottom: 8 }}>
                <div style={{ fontSize: 12, color: '#9aa0a6' }}>log ({e.props?.level ?? 'info'})</div>
                <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{String(e.props?.message ?? '')}</pre>
              </div>
            );
          }
          return (
            <div key={e.id} style={{ padding: '10px 12px', border: '1px solid #222', borderRadius: 10, marginBottom: 8 }}>
              <div style={{ fontSize: 12, color: '#9aa0a6' }}>{e.kind}</div>
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{JSON.stringify(e.props ?? {}, null, 2)}</pre>
            </div>
          );
        })}
      </main>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          void send();
        }}
        style={{ display: 'flex', gap: 8, padding: '12px 16px', borderTop: '1px solid #333' }}
      >
        <input
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder="Ask something..."
          style={{
            flex: 1,
            padding: '10px 12px',
            borderRadius: 8,
            border: '1px solid #333',
            background: '#121212',
            color: '#e6e6e6',
          }}
        />
        <button
          type="submit"
          style={{ padding: '10px 12px', borderRadius: 8, border: '1px solid #333', background: '#222', color: '#eee', cursor: 'pointer' }}
        >
          Send
        </button>
      </form>
    </div>
  );
}

