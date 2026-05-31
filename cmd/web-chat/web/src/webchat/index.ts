export { DefaultComposer } from '../features/web-chat/ChatComposer';
export { DefaultHeader } from '../features/web-chat/ChatHeader';
export { DefaultStatusbar } from '../features/web-chat/ChatStatusbar';
export { ChatTimeline } from '../features/web-chat/ChatTimeline';
export { ChatWidget as LegacyChatWidget } from './ChatWidget';
export * from './cards';
export { ProviderBackedChatWidget as ChatWidget } from './ProviderBackedChatWidget';
export {
  clearRegisteredTimelineRenderers,
  registerTimelineRenderer,
  resolveTimelineRenderers,
  unregisterTimelineRenderer,
} from './rendererRegistry';
export {
  clearRegisteredTimelinePropsNormalizers,
  normalizeTimelineProps,
  registerTimelinePropsNormalizer,
  unregisterTimelinePropsNormalizer,
} from './timelinePropsRegistry';
export type {
  ChatPart,
  ChatWidgetComponents,
  ChatWidgetProps,
  ChatWidgetRenderers,
  ComposerSlotProps,
  HeaderSlotProps,
  PartProps,
  RenderEntity,
  StatusbarSlotProps,
  ThemeVars,
} from './types';
