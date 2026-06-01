import type React from 'react';
import type { HeaderSlotProps, StatusbarSlotProps } from '../types';

export type ChatHeaderProps = HeaderSlotProps & {
  Statusbar: React.ComponentType<StatusbarSlotProps>;
  partProps?: HeaderSlotProps['partProps'];
};
