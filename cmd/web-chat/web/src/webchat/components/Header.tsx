import type React from 'react';
import { getPartProps, mergeClassName, mergeStyle } from '../parts';
import type { HeaderSlotProps, StatusbarSlotProps } from '../types';

export type DefaultHeaderProps = HeaderSlotProps & {
  Statusbar: React.ComponentType<StatusbarSlotProps>;
  partProps?: HeaderSlotProps['partProps'];
};

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
}: DefaultHeaderProps) {
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
