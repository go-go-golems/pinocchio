import { useCallback, useRef, useState } from 'react';
import { useAppSelector } from '../../store/hooks';
import { basePrefixFromLocation } from '../../utils/basePrefix';

export function ExportMenu() {
  const convId = useAppSelector((s: { app: { convId: string } }) => s.app.convId);
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const toggle = useCallback(() => setOpen((prev) => !prev), []);
  const close = useCallback(() => setOpen(false), []);

  const downloadUrl = useCallback(
    (suffix: string) => {
      if (!convId) return '#';
      const base = basePrefixFromLocation();
      return `${base}/api/chat/sessions/${encodeURIComponent(convId)}${suffix}`;
    },
    [convId],
  );

  if (!convId) return null;

  return (
    <div
      data-part="export-menu"
      ref={ref}
      style={{ position: 'relative', display: 'inline-block' }}
    >
      <button
        type="button"
        data-part="pill-button"
        onClick={toggle}
        aria-haspopup="true"
        aria-expanded={open}
      >
        Export ▾
      </button>
      {open ? (
        <>
          <div
            data-part="export-backdrop"
            style={{
              position: 'fixed',
              inset: 0,
              zIndex: 998,
            }}
            onClick={close}
          />
          <div
            data-part="export-dropdown"
            style={{
              position: 'absolute',
              right: 0,
              top: '100%',
              marginTop: 4,
              zIndex: 999,
              padding: '4px 0',
              background: 'var(--pwchat-surface-1, #fff)',
              border: '1px solid var(--pwchat-border, #ddd)',
              borderRadius: 6,
              minWidth: 220,
              boxShadow: '0 4px 12px rgba(0,0,0,0.12)',
            }}
          >
            <ExportItem
              label="Timeline JSON"
              href={downloadUrl('/timeline?format=json&download=true')}
              close={close}
            />
            <ExportItem
              label="Timeline YAML"
              href={downloadUrl('/timeline?format=yaml&download=true')}
              close={close}
            />
            <ExportItem
              label="Turns JSON"
              href={downloadUrl('/turns?format=json&download=true')}
              close={close}
            />
            <ExportItem
              label="Turns YAML"
              href={downloadUrl('/turns?format=yaml&download=true')}
              close={close}
            />
            <ExportItem
              label="Turns Minitrace"
              href={downloadUrl('/turns?format=minitrace&download=true')}
              close={close}
            />
            <ExportItem
              label="Full Export (JSON)"
              href={downloadUrl('/export?format=json&download=true')}
              close={close}
            />
            <ExportItem
              label="Full Export (YAML)"
              href={downloadUrl('/export?format=yaml&download=true')}
              close={close}
            />
          </div>
        </>
      ) : null}
    </div>
  );
}

function ExportItem({
  label,
  href,
  close,
}: {
  label: string;
  href: string;
  close: () => void;
}) {
  return (
    <div>
      <a
        href={href}
        download
        onClick={close}
        style={{
          display: 'block',
          padding: '6px 14px',
          color: 'var(--pwchat-fg, #222)',
          textDecoration: 'none',
          fontSize: 13,
          whiteSpace: 'nowrap',
        }}
      >
        {label}
      </a>
    </div>
  );
}
