import {
  selectOverlay,
  selectTimelineEntities,
  useChatClient,
  useAppSelector as useChatProviderSelector,
} from '@go-go-golems/chat-provider';
import type { KeyboardEvent } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { StreamDebugPanel } from '../../../webchat/components/StreamDebugPanel';
import { getPartProps, mergeClassName, mergeStyle } from '../../../webchat/parts';
import { createWebChatRenderers } from '../../../webchat/renderers';
import type { ChatWidgetComponents, ChatWidgetRenderers } from '../../../webchat/types';
import { DefaultComposer } from '../ChatComposer';
import { DefaultHeader } from '../ChatHeader';
import { ChatTimeline, useStickyScrollFollow } from '../ChatTimeline';
import { toRenderEntity } from '../provider-support/providerTimeline';
import { ProviderStatusbar } from './ProviderStatusbar';
import { ProviderToolCallRenderer } from './ProviderToolCallRenderer';
import { ProviderWidgetRenderer } from './ProviderWidgetRenderer';
import type { WebChatAppProps } from './types';

export function WebChatApp({
  unstyled,
  theme = 'default',
  themeVars,
  className,
  rootProps,
  partProps,
  components,
  renderers,
  selectedProfile,
  profileOptions,
  profileTitle,
  onProfileChange,
}: WebChatAppProps) {
  const client = useChatClient();
  const overlay = useChatProviderSelector(selectOverlay);
  const entitiesRaw = useChatProviderSelector(selectTimelineEntities);
  const entities = useMemo(() => entitiesRaw.map(toRenderEntity), [entitiesRaw]);
  const entityCount = entities.length;
  const [text, setText] = useState('');
  const [showErrors, setShowErrors] = useState(false);

  useEffect(() => {
    void client.connect();
  }, [client]);

  const {
    containerRef: mainRef,
    tailRef: bottomRef,
    mode: scrollMode,
    jumpToLatest,
    onScroll: onMainScroll,
    onWheel: onMainWheel,
  } = useStickyScrollFollow({
    enabled: entityCount > 0,
    isStreaming: overlay.runStatus === 'streaming',
    contentVersion: `${entityCount}:${overlay.runStatus}`,
    resetKey: overlay.sessionId,
  });

  const send = useCallback(() => {
    const prompt = text.trim();
    if (!prompt) return;
    setText('');
    void client.send(prompt);
  }, [client, text]);

  const onKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        send();
      }
    },
    [send],
  );

  const onNewConversation = useCallback(() => {
    client.reset();
  }, [client]);

  const mergedRenderers: ChatWidgetRenderers = useMemo(
    () =>
      createWebChatRenderers({
        overrides: {
          ...renderers,
          tool_call: ProviderToolCallRenderer,
          widget: ProviderWidgetRenderer,
        },
      }),
    [renderers],
  );

  const HeaderOverride = (components as ChatWidgetComponents | undefined)?.Header;
  const StatusbarComponent = (components as ChatWidgetComponents | undefined)?.Statusbar ?? ProviderStatusbar;
  const ComposerComponent = (components as ChatWidgetComponents | undefined)?.Composer ?? DefaultComposer;

  const rootPartProps = getPartProps('root', partProps);
  const rootClassName = mergeClassName(className, rootPartProps.className, rootProps?.className);
  const rootStyle = mergeStyle(themeVars, rootPartProps.style, rootProps?.style);
  const status = overlay.runStatus || 'idle';
  const wsStatus = overlay.wsStatus || 'disconnected';
  const displayedWsStatus = wsStatus === 'subscribed' || wsStatus === 'hydrated' ? 'connected' : wsStatus;

  return (
    <div
      {...rootPartProps}
      {...rootProps}
      data-pwchat=""
      data-part="root"
      data-theme={unstyled ? undefined : theme || 'default'}
      data-fullscreen="true"
      className={rootClassName}
      style={rootStyle}
    >
      {HeaderOverride ? (
        <HeaderOverride
          title={profileTitle}
          profile={selectedProfile}
          profiles={profileOptions}
          wsStatus={displayedWsStatus}
          status={status}
          queueDepth={0}
          lastSeq={0}
          errorCount={overlay.error ? 1 : 0}
          showErrors={showErrors}
          onProfileChange={onProfileChange}
          onToggleErrors={() => setShowErrors((prev) => !prev)}
          partProps={partProps}
        />
      ) : (
        <DefaultHeader
          title={profileTitle}
          profile={selectedProfile}
          profiles={profileOptions}
          wsStatus={displayedWsStatus}
          status={status}
          queueDepth={0}
          lastSeq={0}
          errorCount={overlay.error ? 1 : 0}
          showErrors={showErrors}
          onProfileChange={onProfileChange}
          onToggleErrors={() => setShowErrors((prev) => !prev)}
          Statusbar={StatusbarComponent as any}
          partProps={partProps}
        />
      )}

      <main data-part="main" ref={mainRef} onScroll={onMainScroll} onWheel={onMainWheel}>
        <ChatTimeline
          entities={entities}
          errors={overlay.error ? [{ id: 'provider-error', scope: 'chat-provider', message: overlay.error, time: Date.now() }] : []}
          showErrors={showErrors}
          errorCount={overlay.error ? 1 : 0}
          onClearErrors={() => setShowErrors(false)}
          onToggleErrors={() => setShowErrors((prev) => !prev)}
          renderers={mergedRenderers}
          bottomRef={bottomRef}
          partProps={partProps}
          state={status}
        />
        {scrollMode === 'detached' ? (
          <div data-part="jump-to-latest-wrap">
            <button type="button" data-part="pill-button" data-variant="accent" onClick={jumpToLatest}>
              Jump to latest
            </button>
          </div>
        ) : null}
      </main>

      <ComposerComponent
        text={text}
        disabled={!text.trim()}
        onChangeText={setText}
        onSubmit={send}
        onNewConversation={onNewConversation}
        onKeyDown={onKeyDown}
        partProps={partProps}
      />
      <StreamDebugPanel />
    </div>
  );
}
