import type React from 'react';
import type { ProfileInfo } from '../store/profileApi';

export type RenderEntity = {
  id: string;
  kind: string;
  props: any;
  createdAt: number;
  updatedAt?: number;
};

export type ThemeVars = Partial<Record<`--pwchat-${string}`, string>>;

export type ChatPart =
  | 'root'
  | 'header'
  | 'timeline'
  | 'composer'
  | 'statusbar'
  | 'turn'
  | 'bubble'
  | 'content'
  | 'composer-input'
  | 'composer-actions'
  | 'send-button';

export type PartProps = Partial<Record<ChatPart, React.HTMLAttributes<HTMLElement>>>;

export type HeaderSlotProps = {
  title: string;
  profile: string;
  profiles: ProfileInfo[];
  wsStatus: string;
  status: string;
  queueDepth: number;
  lastSeq: number;
  errorCount: number;
  showErrors: boolean;
  onProfileChange: (slug: string) => void;
  onToggleErrors: () => void;
  partProps?: PartProps;
};

export type StatusbarSlotProps = {
  profile: string;
  profiles: ProfileInfo[];
  wsStatus: string;
  status: string;
  queueDepth: number;
  lastSeq: number;
  errorCount: number;
  showErrors: boolean;
  onProfileChange: (slug: string) => void;
  onToggleErrors: () => void;
  partProps?: PartProps;
};

export type ComposerSlotProps = {
  text: string;
  disabled: boolean;
  onChangeText: (next: string) => void;
  onSubmit: () => void;
  onNewConversation: () => void;
  onKeyDown: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
  partProps?: PartProps;
};

export type ChatWidgetComponents = {
  Header: React.ComponentType<HeaderSlotProps>;
  Statusbar: React.ComponentType<StatusbarSlotProps>;
  Composer: React.ComponentType<ComposerSlotProps>;
};

export type ChatWidgetRenderers = Record<string, React.ComponentType<{ e: RenderEntity }>> & {
  default?: React.ComponentType<{ e: RenderEntity }>;
};

export type ChatWidgetProps = {
  unstyled?: boolean;
  theme?: string;
  themeVars?: ThemeVars;
  className?: string;
  rootProps?: React.HTMLAttributes<HTMLDivElement>;
  partProps?: PartProps;
  components?: Partial<ChatWidgetComponents>;
  renderers?: Partial<ChatWidgetRenderers>;
  buildOverrides?: () => Record<string, any> | undefined;
};
