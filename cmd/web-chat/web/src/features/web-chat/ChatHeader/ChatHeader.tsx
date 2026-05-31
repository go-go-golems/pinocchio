import { getPartProps, mergeClassName, mergeStyle } from '../../../webchat/parts';
import type { ChatHeaderProps } from './types';

export function DefaultHeader({
  Statusbar,
  partProps,
  title,
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
}: ChatHeaderProps) {
  const headerProps = getPartProps('header', partProps);
  const headerClassName = mergeClassName(headerProps.className);
  const headerStyle = mergeStyle(headerProps.style);

  return (
    <header {...headerProps} data-part="header" className={headerClassName} style={headerStyle}>
      <div data-part="header-title">{title}</div>
      <Statusbar
        profile={profile}
        profiles={profiles}
        wsStatus={wsStatus}
        status={status}
        queueDepth={queueDepth}
        lastSeq={lastSeq}
        errorCount={errorCount}
        showErrors={showErrors}
        onProfileChange={onProfileChange}
        onToggleErrors={onToggleErrors}
        partProps={partProps}
      />
    </header>
  );
}
