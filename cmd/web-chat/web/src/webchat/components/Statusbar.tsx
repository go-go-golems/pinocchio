import { getPartProps, mergeClassName, mergeStyle } from '../parts';
import type { StatusbarSlotProps } from '../types';
import { fmtShort } from '../utils';

export function DefaultStatusbar(props: StatusbarSlotProps) {
  const {
    profile,
    profiles,
    wsStatus,
    status,
    queueDepth,
    lastSeq,
    errorCount,
    showErrors,
    onProfileChange,
    onToggleErrors,
    partProps,
  } = props;

  const statusbarProps = getPartProps('statusbar', partProps);
  const statusbarClassName = mergeClassName(statusbarProps.className);
  const statusbarStyle = mergeStyle(statusbarProps.style);

  return (
    <div
      {...statusbarProps}
      data-part="statusbar"
      data-state={wsStatus || undefined}
      className={statusbarClassName}
      style={statusbarStyle}
    >
      <label data-part="pill">
        profile
        <select
          data-part="pill-select"
          value={profile || 'default'}
          onChange={(e) => void onProfileChange(e.target.value)}
        >
          {profiles.map((p) => (
            <option key={p.slug} value={p.slug}>
              {p.slug}
            </option>
          ))}
        </select>
      </label>
      <span data-part="pill" data-variant={wsStatus === 'connected' ? 'accent' : undefined}>
        ws: {wsStatus}
      </span>
      <span data-part="pill">seq: {fmtShort(lastSeq)}</span>
      <span data-part="pill">q: {fmtShort(queueDepth)}</span>
      <span data-part="pill">{status}</span>
      {errorCount > 0 ? (
        <button
          type="button"
          data-part="pill-button"
          data-variant="danger"
          aria-pressed={showErrors}
          onClick={onToggleErrors}
        >
          errors: {errorCount}
        </button>
      ) : null}
    </div>
  );
}
