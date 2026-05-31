import type React from 'react';
import type { ChatWidgetRenderers, PartProps, RenderEntity } from '../../../webchat/types';

export type ChatTimelineError = {
  id: string;
  scope: string;
  message: string;
  detail?: string;
  extra?: unknown;
  time: number;
};

export type ChatTimelineProps = {
  entities: RenderEntity[];
  errors: ChatTimelineError[];
  showErrors: boolean;
  errorCount: number;
  onClearErrors: () => void;
  onToggleErrors: () => void;
  renderers: ChatWidgetRenderers;
  bottomRef: React.RefObject<HTMLDivElement>;
  partProps?: PartProps;
  state?: string;
};
