import type React from 'react';
import type { ProfileInfo } from '../../store/profileApi';

export type JsonObject = Record<string, unknown>;

export type MessageEntityProps = JsonObject & {
  role?: 'user' | 'assistant' | 'thinking' | string;
  content?: string;
  error?: string;
  streaming?: boolean;
  status?: string;
};

export type ToolCallEntityProps = JsonObject & {
  name?: string;
  toolName?: string;
  toolCallId?: string;
  sessionId?: string;
  status?: string;
  input?: unknown;
  result?: unknown;
  error?: string;
  done?: boolean;
  mode?: string;
};

export type ToolResultEntityProps = JsonObject & {
  customKind?: string;
  result?: unknown;
  resultRaw?: unknown;
  error?: string;
  status?: string;
  toolName?: string;
  toolCallId?: string;
};

export type AgentModeEntityProps = JsonObject & {
  title?: string;
  data?: JsonObject;
  preview?: boolean;
  messageId?: string;
};

export type WidgetEntityProps = JsonObject & {
  instanceId?: string;
  widgetName?: string;
  widget_name?: string;
  status?: string;
  props?: JsonObject;
};

export type LogEntityProps = JsonObject & {
  level?: string;
  message?: string;
};

export type GenericEntityProps = JsonObject;

export type RenderEntityKind =
  | 'message'
  | 'tool_call'
  | 'tool_result'
  | 'agent_mode'
  | 'agent_mode_preview'
  | 'widget'
  | 'widget_instance'
  | 'ChatFrontendToolCall'
  | 'ChatWidgetInstance'
  | 'log'
  | (string & {});

export type RenderEntityProps =
  | MessageEntityProps
  | ToolCallEntityProps
  | ToolResultEntityProps
  | AgentModeEntityProps
  | WidgetEntityProps
  | LogEntityProps
  | GenericEntityProps;

export type RenderEntity<K extends RenderEntityKind = RenderEntityKind, P extends RenderEntityProps = RenderEntityProps> = {
  id: string;
  kind: K;
  props: P;
  createdAt: number;
  updatedAt?: number;
};

export type ThemeVars = Partial<Record<`--pwchat-${string}`, string>>;

export type ChatPart =
  | 'root'
  | 'main'
  | 'header'
  | 'header-title'
  | 'statusbar'
  | 'timeline'
  | 'turn'
  | 'bubble'
  | 'content'
  | 'composer'
  | 'composer-inner'
  | 'composer-input'
  | 'composer-hint'
  | 'composer-actions'
  | 'send-button'
  | 'button'
  | 'callout'
  | 'callout-body'
  | 'card'
  | 'card-body'
  | 'card-header'
  | 'card-header-meta'
  | 'card-header-title'
  | 'error-panel'
  | 'error-panel-header'
  | 'error-panel-body'
  | 'error-item'
  | 'error-item-message'
  | 'error-item-detail'
  | 'export-menu'
  | 'export-backdrop'
  | 'export-dropdown'
  | 'export-link'
  | 'jump-to-latest-wrap'
  | 'log-message'
  | 'markdown'
  | 'message-role'
  | 'mono'
  | 'pill'
  | 'pill-button'
  | 'pill-select'
  | 'row'
  | 'streaming-dot'
  | 'toolbar'
  | 'unsafe-link';

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
};
