import { useCallback, useRef, useState } from 'react';
import { useAppSelector } from '../../store/hooks';
import { basePrefixFromLocation } from '../../utils/basePrefix';

export function ExportMenu() {
  const convId = useAppSelector((s: { app: { convId: string } }) => s.app.convId);
  return <ExportMenuForSession sessionId={convId} />;
}

export function ExportMenuForSession({ sessionId }: { sessionId?: string | null }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const toggle = useCallback(() => setOpen((prev) => !prev), []);
  const close = useCallback(() => setOpen(false), []);

  const downloadUrl = useCallback(
    (suffix: string) => {
      if (!sessionId) return '#';
      const base = basePrefixFromLocation();
      return `${base}/api/chat/sessions/${encodeURIComponent(sessionId)}${suffix}`;
    },
    [sessionId],
  );

  if (!sessionId) return null;

  return (
    <div data-part="export-menu" ref={ref}>
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
          <div data-part="export-backdrop" onClick={close} />
          <div data-part="export-dropdown">
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
      <a href={href} download onClick={close} data-part="export-link">
        {label}
      </a>
    </div>
  );
}
