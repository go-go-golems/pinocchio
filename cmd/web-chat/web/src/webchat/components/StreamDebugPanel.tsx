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
      <div data-part="stream-debug">
        <button type="button" onClick={handleToggle} data-part="stream-debug-toggle" data-state="disabled" title="Enable stream debug recording">
          Debug
        </button>
      </div>
    );
  }

  return (
    <div data-part="stream-debug">
      <button type="button" onClick={() => setOpen((prev) => !prev)} data-part="stream-debug-toggle">
        Stream Debug ({entries.length})
      </button>
      {open ? (
        <div data-part="stream-debug-panel">
          <div data-part="stream-debug-toolbar">
            <input value={filter} onChange={(ev) => setFilter(ev.target.value)} placeholder="filter" data-part="stream-debug-filter" />
            <button type="button" onClick={() => void uploadAndDownloadSQLite()}>
              Download SQLite
            </button>
            <button type="button" onClick={exportStreamDebugJSON}>
              Export
            </button>
            <button
              type="button"
              onClick={() => {
                clearStreamDebugEntries();
                setEntries([]);
              }}
            >
              Clear
            </button>
            <button type="button" onClick={handleToggle} data-part="stream-debug-stop">
              Stop
            </button>
          </div>
          <div data-part="stream-debug-list">
            {visible.map((entry) => (
              <details key={entry.id} data-part="stream-debug-entry">
                <summary data-part="stream-debug-summary">
                  {new Date(entry.timestamp).toLocaleTimeString()} {summarize(entry)}
                </summary>
                <pre data-part="stream-debug-json">{JSON.stringify(entry, null, 2)}</pre>
              </details>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
