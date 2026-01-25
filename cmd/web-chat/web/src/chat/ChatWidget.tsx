import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { wsManager } from '../ws/wsManager';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { selectTimelineEntities } from '../store/timelineSlice';
import { appSlice } from '../store/appSlice';
import { Markdown } from './Markdown';
import './chat.css';
import { timelineSlice } from '../store/timelineSlice';

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

function convIdFromLocation(): string {
  try {
    const u = new URL(window.location.href);
    const q = u.searchParams.get('conv_id') || u.searchParams.get('convId') || '';
    return q.trim();
  } catch {
    return '';
  }
}

function setConvIdInLocation(convId: string | null) {
  try {
    const u = new URL(window.location.href);
    if (!convId) {
      u.searchParams.delete('conv_id');
      u.searchParams.delete('convId');
    } else {
      u.searchParams.set('conv_id', convId);
      u.searchParams.delete('convId');
    }
    window.history.replaceState({}, '', u.toString());
  } catch {
    // ignore
  }
}

function fmtShort(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0';
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1)}k`;
  return `${(n / 1_000_000).toFixed(1)}m`;
}

function MessageCard({ e }: { e: RenderEntity }) {
  const role = String(e.props?.role ?? 'assistant');
  const content = String(e.props?.content ?? '');
  const streaming = !!e.props?.streaming;
  const roleClass =
    role === 'user'
      ? 'messageRoleUser'
      : role === 'thinking'
        ? 'messageRoleThinking'
        : 'messageRoleAssistant';

  return (
    <div className="card">
      <div className="cardHeader">
        <div className={`messageRole ${roleClass}`}>{role}</div>
        {streaming ? <div className="streamingDot" /> : null}
        <div className="cardHeaderMeta">#{fmtShort(e.createdAt)}</div>
      </div>
      <div className="cardBody">
        {content ? <Markdown text={content} /> : <div className="pill">...</div>}
      </div>
    </div>
  );
}

function ToolCallCard({ e }: { e: RenderEntity }) {
  const name = String(e.props?.name ?? 'tool');
  const input = e.props?.input ?? {};
  const done = !!e.props?.done;
  const title = done ? `${name} (done)` : name;
  return (
    <div className="card">
      <div className="cardHeader">
        <div className="cardHeaderTitle">Tool</div>
        <div className="pill pillAccent mono">{title}</div>
        <div className="cardHeaderMeta">#{fmtShort(e.createdAt)}</div>
      </div>
      <div className="cardBody">
        <div className="toolbar">
          <button
            type="button"
            className="btn btnGhost"
            onClick={() => void navigator.clipboard.writeText(JSON.stringify(input ?? {}, null, 2)).catch(() => {})}
          >
            Copy args
          </button>
        </div>
        <pre className="mono" style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {JSON.stringify(input ?? {}, null, 2)}
        </pre>
      </div>
    </div>
  );
}

function ToolResultCard({ e }: { e: RenderEntity }) {
  const customKind = e.props?.customKind ? String(e.props.customKind) : '';
  const result = String(e.props?.result ?? '');
  return (
    <div className="card">
      <div className="cardHeader">
        <div className="cardHeaderTitle">Result</div>
        {customKind ? <div className="pill mono">{customKind}</div> : null}
        <div className="cardHeaderMeta">#{fmtShort(e.createdAt)}</div>
      </div>
      <div className="cardBody">
        <div className="toolbar">
          <button type="button" className="btn btnGhost" onClick={() => void navigator.clipboard.writeText(result).catch(() => {})}>
            Copy
          </button>
        </div>
        <pre className="mono" style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {result}
        </pre>
      </div>
    </div>
  );
}

function LogCard({ e }: { e: RenderEntity }) {
  const level = String(e.props?.level ?? 'info');
  const message = String(e.props?.message ?? '');
  return (
    <div className="card" style={{ background: '#0f0f0f' }}>
      <div className="cardBody">
        <div className="row" style={{ justifyContent: 'space-between' }}>
          <div className="pill mono">{level}</div>
          <div className="cardHeaderMeta">#{fmtShort(e.createdAt)}</div>
        </div>
        <div style={{ marginTop: 8, color: 'var(--muted)', fontSize: 13 }}>{message}</div>
      </div>
    </div>
  );
}

function ThinkingModeCard({ e }: { e: RenderEntity }) {
  const mode = String(e.props?.mode ?? '');
  const phase = String(e.props?.phase ?? '');
  const status = String(e.props?.status ?? '');
  const success = e.props?.success;
  const error = e.props?.error ? String(e.props.error) : '';
  const reasoning = e.props?.reasoning ? String(e.props.reasoning) : '';
  const header = mode ? `Thinking mode: ${mode}` : 'Thinking mode';

  return (
    <div className="card">
      <div className="cardHeader">
        <div className="cardHeaderTitle">{header}</div>
        {phase ? <div className="pill mono">{phase}</div> : null}
        {status ? <div className="pill">{status}</div> : null}
        {typeof success === 'boolean' ? <div className={`pill ${success ? 'ok' : 'error'}`}>{success ? 'ok' : 'fail'}</div> : null}
        <div className="cardHeaderMeta">#{fmtShort(e.createdAt)}</div>
      </div>
      <div className="cardBody">
        {reasoning ? <Markdown text={reasoning} /> : <div className="pill">No reasoning</div>}
        {error ? <div className="error" style={{ marginTop: 10 }}>{error}</div> : null}
      </div>
    </div>
  );
}

function PlanningCard({ e }: { e: RenderEntity }) {
  const runId = String(e.props?.runId ?? e.id);
  const provider = String(e.props?.provider ?? '');
  const plannerModel = String(e.props?.plannerModel ?? '');
  const maxIterations = e.props?.maxIterations;
  const iterations = Array.isArray(e.props?.iterations) ? (e.props.iterations as any[]) : [];
  const reflectionByIter = e.props?.reflectionByIter ?? {};
  const completed = e.props?.completed ?? null;
  const execution = e.props?.execution ?? null;

  return (
    <div className="card">
      <div className="cardHeader">
        <div className="cardHeaderTitle">Planning</div>
        <div className="pill mono">{runId}</div>
        {provider ? <div className="pill">{provider}</div> : null}
        {plannerModel ? <div className="pill mono">{plannerModel}</div> : null}
        {typeof maxIterations === 'number' ? <div className="pill">max {maxIterations}</div> : null}
        <div className="cardHeaderMeta">#{fmtShort(e.createdAt)}</div>
      </div>
      <div className="cardBody">
        <div className="kv" style={{ marginBottom: 10 }}>
          <div className="kvKey">Iterations</div>
          <div>{iterations.length}</div>
          {execution ? (
            <>
              <div className="kvKey">Execution</div>
              <div className={execution.status === 'completed' ? 'ok' : execution.status === 'error' ? 'error' : ''}>{String(execution.status ?? '')}</div>
            </>
          ) : null}
          {completed ? (
            <>
              <div className="kvKey">Decision</div>
              <div>{String(completed.finalDecision ?? '')}</div>
            </>
          ) : null}
        </div>

        {iterations.map((it: any) => {
          const idx = Number(it.index ?? 0);
          const reflection = reflectionByIter?.[idx];
          const title = `Iteration ${idx}: ${String(it.action ?? '')}`.trim();
          const toolName = String(it.toolName ?? '');
          const progress = String(it.progress ?? '');
          const strategy = String(it.strategy ?? '');
          const reasoning = String(it.reasoning ?? '');
          const reflText = reflection?.text ? String(reflection.text) : String(it.reflectionText ?? '');
          const reflScore = reflection?.score;

          return (
            <div key={`iter-${idx}`} style={{ marginBottom: 10, borderTop: '1px solid var(--border)', paddingTop: 10 }}>
              <div className="row" style={{ justifyContent: 'space-between' }}>
                <div className="pill pillAccent">{title}</div>
                {toolName ? <div className="pill mono">{toolName}</div> : null}
              </div>
              {progress ? <div style={{ marginTop: 8, color: 'var(--muted)', fontSize: 13 }}>{progress}</div> : null}
              {strategy ? <div style={{ marginTop: 8 }}><span className="pill mono">strategy</span> <span style={{ color: 'var(--muted)', fontSize: 13 }}>{strategy}</span></div> : null}
              {reasoning ? <div style={{ marginTop: 8 }}><Markdown text={reasoning} /></div> : null}
              {reflText ? (
                <div style={{ marginTop: 8 }}>
                  <div className="row" style={{ gap: 8, marginBottom: 6 }}>
                    <span className="pill mono">reflection</span>
                    {typeof reflScore === 'number' ? <span className="pill">score {reflScore.toFixed(2)}</span> : null}
                  </div>
                  <Markdown text={reflText} />
                </div>
              ) : null}
            </div>
          );
        })}

        {completed ? (
          <div style={{ marginTop: 10, borderTop: '1px solid var(--border)', paddingTop: 10 }}>
            <div className="row" style={{ justifyContent: 'space-between' }}>
              <div className="pill">completed</div>
              <div className="pill">{String(completed.statusReason ?? '')}</div>
            </div>
            {completed.finalDirective ? (
              <div style={{ marginTop: 10 }}>
                <div className="row" style={{ gap: 8, marginBottom: 6 }}>
                  <span className="pill mono">directive</span>
                </div>
                <Markdown text={String(completed.finalDirective)} />
              </div>
            ) : null}
          </div>
        ) : null}

        {execution?.errorMessage ? <div className="error" style={{ marginTop: 10 }}>{String(execution.errorMessage)}</div> : null}
      </div>
    </div>
  );
}

function GenericCard({ e }: { e: RenderEntity }) {
  return (
    <div className="card">
      <div className="cardHeader">
        <div className="cardHeaderTitle mono">{e.kind}</div>
        <div className="cardHeaderMeta">#{fmtShort(e.createdAt)}</div>
      </div>
      <div className="cardBody">
        <pre className="mono" style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {JSON.stringify(e.props ?? {}, null, 2)}
        </pre>
      </div>
    </div>
  );
}

export function ChatWidget() {
  const dispatch = useAppDispatch();
  const app = useAppSelector((s) => s.app);
  const entities = useAppSelector(selectTimelineEntities) as RenderEntity[];

  const [text, setText] = useState('');
  const basePrefix = useMemo(() => basePrefixFromLocation(), []);
  const initialUrlConvId = useMemo(() => convIdFromLocation(), []);
  const mainRef = useRef<HTMLDivElement | null>(null);
  const bottomRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!app.convId && initialUrlConvId) {
      dispatch(appSlice.actions.setConvId(initialUrlConvId));
    }
  }, [app.convId, dispatch, initialUrlConvId]);

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

  useEffect(() => {
    // naive “always scroll” works for a chat widget; we can make this smarter later.
    bottomRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [entities.length]);

  const send = useCallback(async () => {
    const prompt = text.trim();
    if (!prompt) return;
    setText('');

    // optimistic user echo
    dispatch(
      timelineSlice.actions.addEntity({
        id: `user-${Date.now()}`,
        kind: 'message',
        createdAt: Date.now(),
        props: { role: 'user', content: prompt, streaming: false },
      }),
    );

    dispatch(appSlice.actions.setStatus('sending...'));
    const idempotencyKey = (globalThis.crypto && 'randomUUID' in globalThis.crypto && typeof globalThis.crypto.randomUUID === 'function')
      ? globalThis.crypto.randomUUID()
      : `idem-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    const body = app.convId ? { prompt, conv_id: app.convId } : { prompt };
    const res = await fetch(`${basePrefix}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(body),
    });
    const j = await res.json().catch(() => null);
    if (!res.ok) {
      dispatch(appSlice.actions.setStatus(`error (${res.status})`));
      return;
    }

    const convId = (j && j.conv_id) || app.convId || '';
    const runId = (j && (j.session_id || j.run_id)) || app.runId || '';
    if (!app.convId && convId) {
      setConvIdInLocation(convId);
    }
    dispatch(appSlice.actions.setConvId(convId));
    dispatch(appSlice.actions.setRunId(runId));

    const st = (j && j.status) || (res.status === 202 ? 'queued' : 'sent');
    const qp = j && typeof j.queue_position === 'number' ? ` (pos ${j.queue_position})` : '';
    dispatch(appSlice.actions.setStatus(`${st}${qp}`));
  }, [app.convId, app.runId, basePrefix, dispatch, text]);

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey && !e.metaKey) {
        e.preventDefault();
        void send();
      }
    },
    [send],
  );

  const onNewConversation = useCallback(() => {
    wsManager.disconnect();
    dispatch(appSlice.actions.setConvId(''));
    dispatch(appSlice.actions.setRunId(''));
    dispatch(appSlice.actions.setStatus('idle'));
    dispatch(appSlice.actions.setWsStatus('disconnected'));
    dispatch(appSlice.actions.setLastSeq(0));
    dispatch(appSlice.actions.setQueueDepth(0));
    dispatch(timelineSlice.actions.clear());
    setConvIdInLocation(null);
  }, [dispatch]);

  return (
    <div className="chatRoot">
      <header className="chatHeader">
        <div className="chatHeaderTitle">Web Chat</div>
        <div className="chatStatus">
          <span className={`pill ${app.wsStatus === 'connected' ? 'pillAccent' : ''}`}>ws: {app.wsStatus}</span>
          <span className="pill">seq: {fmtShort(app.lastSeq)}</span>
          <span className="pill">q: {fmtShort(app.queueDepth)}</span>
          <span className="pill">{app.status}</span>
        </div>
      </header>

      <main ref={mainRef} className="chatMain">
        <div className="timeline">
          {entities.map((e) => {
            if (e.kind === 'message') return <MessageCard key={e.id} e={e} />;
            if (e.kind === 'tool_call') return <ToolCallCard key={e.id} e={e} />;
            if (e.kind === 'tool_result') return <ToolResultCard key={e.id} e={e} />;
            if (e.kind === 'log') return <LogCard key={e.id} e={e} />;
            if (e.kind === 'thinking_mode') return <ThinkingModeCard key={e.id} e={e} />;
            if (e.kind === 'planning') return <PlanningCard key={e.id} e={e} />;
            return <GenericCard key={e.id} e={e} />;
          })}
          <div ref={bottomRef} />
        </div>
      </main>

      <form
        className="chatComposer"
        onSubmit={(e) => {
          e.preventDefault();
          void send();
        }}
      >
        <div className="composerInner">
          <div>
            <textarea
              className="textarea"
              value={text}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={onKeyDown}
              placeholder="Ask something…"
            />
            <div className="hint">Enter to send · Shift+Enter for newline</div>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <button type="submit" className="btn" disabled={!text.trim()}>
              Send
            </button>
            <button
              type="button"
              className="btn btnGhost"
              onClick={onNewConversation}
            >
              New conv
            </button>
          </div>
        </div>
      </form>
    </div>
  );
}
