import { useEffect, useMemo, useState } from 'react';
import {
  clearStreamDebugEntries,
  exportStreamDebugJSON,
  getStreamDebugEntries,
  type StreamDebugEntry,
  streamDebugEnabled,
  toggleStreamDebug,
  uploadAndDownloadSQLite,
} from '../../ws/streamDebug';

function summarize(entry: StreamDebugEntry): string {
  if (entry.type === 'raw-ws') return `${entry.type} ${entry.size ?? 0} bytes`;
  if (entry.type === 'parsed-frame') return `${entry.type} ${(entry.frameType as string) || ''} ${(entry.name as string) || ''}`;
  if (entry.type === 'snapshot') return `${entry.type} ordinal=${entry.ordinal ?? ''} entities=${entry.entityCount ?? 0} dropped=${entry.droppedCount ?? 0}`;
  if (entry.type === 'ui-event') return `${entry.type} ordinal=${entry.ordinal ?? ''} ${(entry.name as string) || ''}`;
  if (entry.type === 'ws-lifecycle') return `${entry.type} ${(entry.event as string) || ''}`;
  return String(entry.type);
}

export function StreamDebugPanel() {
  const [enabled, setEnabled] = useState(() => streamDebugEnabled());
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState('');
  const [entries, setEntries] = useState<StreamDebugEntry[]>(() => getStreamDebugEntries());

  useEffect(() => {
    const interval = window.setInterval(() => {
      setEnabled(streamDebugEnabled());
      setEntries(getStreamDebugEntries());
    }, 500);
    return () => window.clearInterval(interval);
  }, []);

  useEffect(() => {
    const onKey = (ev: KeyboardEvent) => {
      if ((ev.ctrlKey || ev.metaKey) && ev.shiftKey && ev.key.toLowerCase() === 'd') {
        ev.preventDefault();
        setOpen((prev) => !prev);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const visible = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return entries.slice(-200).reverse();
    return entries.filter((entry) => JSON.stringify(entry).toLowerCase().includes(q)).slice(-200).reverse();
  }, [entries, filter]);

  const handleToggle = () => {
    const nowEnabled = toggleStreamDebug();
    setEnabled(nowEnabled);
    if (!nowEnabled) {
      setOpen(false);
    }
  };

  if (!enabled) {
    return (
      <div style={{ position: 'fixed', right: 12, bottom: 12, zIndex: 9999, fontFamily: 'monospace', fontSize: 12 }}>
        <button
          type="button"
          onClick={handleToggle}
          style={{ padding: '4px 8px', border: '1px dashed #666', background: '#111', color: '#888', cursor: 'pointer' }}
          title="Enable stream debug recording"
        >
          Debug
        </button>
      </div>
    );
  }

  return (
    <div style={{ position: 'fixed', right: 12, bottom: 12, zIndex: 9999, fontFamily: 'monospace', fontSize: 12 }}>
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        style={{ padding: '4px 8px', border: '1px solid #666', background: '#111', color: '#fff' }}
      >
        Stream Debug ({entries.length})
      </button>
      {open ? (
        <div style={{ width: 560, maxWidth: '90vw', height: 360, maxHeight: '50vh', overflow: 'hidden', background: '#101014', color: '#eee', border: '1px solid #555', boxShadow: '0 8px 30px rgba(0,0,0,0.4)' }}>
          <div style={{ display: 'flex', gap: 6, padding: 8, borderBottom: '1px solid #333' }}>
            <input
              value={filter}
              onChange={(ev) => setFilter(ev.target.value)}
              placeholder="filter"
              style={{ flex: 1, background: '#1d1d24', color: '#eee', border: '1px solid #555', padding: 4 }}
            />
            <button type="button" onClick={() => void uploadAndDownloadSQLite()}>Download SQLite</button>
            <button type="button" onClick={exportStreamDebugJSON}>Export</button>
            <button type="button" onClick={() => { clearStreamDebugEntries(); setEntries([]); }}>Clear</button>
            <button type="button" onClick={handleToggle} style={{ color: '#f88' }}>Stop</button>
          </div>
          <div style={{ overflow: 'auto', height: 310 }}>
            {visible.map((entry) => (
              <details key={entry.id} style={{ borderBottom: '1px solid #272730', padding: '4px 8px' }}>
                <summary style={{ cursor: 'pointer', color: '#b8d7ff' }}>{new Date(entry.timestamp).toLocaleTimeString()} {summarize(entry)}</summary>
                <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', color: '#ddd' }}>{JSON.stringify(entry, null, 2)}</pre>
              </details>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
