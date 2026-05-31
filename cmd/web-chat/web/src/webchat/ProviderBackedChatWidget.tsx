import {
  ChatProvider,
  selectOverlay,
  selectTimelineEntities,
  ToolCallOutlet,
  useChatClient,
  useAppSelector as useChatProviderSelector,
  WidgetOutlet,
} from '@go-go-golems/chat-provider';
import type { KeyboardEvent } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { appSlice } from '../store/appSlice';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { type ProfileInfo, useGetProfileQuery, useGetProfilesQuery, useSetProfileMutation } from '../store/profileApi';
import { basePrefixFromLocation } from '../utils/basePrefix';
import { logWarn } from '../utils/logger';
import { DefaultComposer } from './components/Composer';
import { DefaultHeader } from './components/Header';
import { ChatTimeline } from './components/Timeline';
import { useStickyScrollFollow } from './hooks/useStickyScrollFollow';
import { WebChatProviderCapabilities } from './ProviderDemoPage';
import { getPartProps, mergeClassName, mergeStyle } from './parts';
import { resolveSelectedProfile } from './profileSelection';
import { resolveTimelineRenderers } from './rendererRegistry';
import './styles/theme-default.css';
import './styles/webchat.css';
import type { ChatWidgetComponents, ChatWidgetProps, ChatWidgetRenderers, RenderEntity, StatusbarSlotProps } from './types';
import { fmtShort } from './utils';

function setSessionIdInLocation(sessionId: string | null) {
  try {
    const u = new URL(window.location.href);
    if (!sessionId) u.searchParams.delete('sessionId');
    else u.searchParams.set('sessionId', sessionId);
    window.history.replaceState({}, '', u.toString());
  } catch (err) {
    logWarn('setSessionIdInLocation failed', { scope: 'setSessionIdInLocation' }, err);
  }
}

function toRenderEntity(e: any): RenderEntity {
  const createdAt = Number(e?.createdAt ?? 0);
  return {
    id: String(e?.id ?? ''),
    kind: String(e?.kind ?? ''),
    props: e?.props ?? {},
    createdAt: Number.isFinite(createdAt) ? createdAt : 0,
    updatedAt: e?.updatedAt ? Number(e.updatedAt) : undefined,
  };
}

function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) return value as Record<string, unknown>;
  return {};
}

function ProviderToolCallRenderer({ e }: { e: RenderEntity }) {
  return (
    <ToolCallOutlet
      toolCallId={String(e.props?.toolCallId ?? e.id)}
      toolName={String(e.props?.toolName ?? e.props?.name ?? 'tool')}
      status={String(e.props?.status ?? 'requested')}
      input={e.props?.input}
      result={e.props?.result}
      error={typeof e.props?.error === 'string' ? e.props.error : undefined}
    />
  );
}

function ProviderWidgetRenderer({ e }: { e: RenderEntity }) {
  return (
    <WidgetOutlet
      instanceId={String(e.props?.instanceId ?? e.id)}
      widgetName={String(e.props?.widgetName ?? 'widget')}
      status={String(e.props?.status ?? 'unknown')}
      props={asRecord(e.props?.props)}
    />
  );
}

function ProviderStatusbar(props: StatusbarSlotProps) {
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
        <select data-part="pill-select" value={profile || 'default'} onChange={(e) => void onProfileChange(e.target.value)}>
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
        <button type="button" data-part="pill-button" data-variant="danger" aria-pressed={showErrors} onClick={onToggleErrors}>
          errors: {errorCount}
        </button>
      ) : null}
    </div>
  );
}

type ProviderBackedChatWidgetInnerProps = ChatWidgetProps & {
  selectedProfile: string;
  profileOptions: ProfileInfo[];
  profileTitle: string;
  onProfileChange: (slug: string) => void;
};

function ProviderBackedChatWidgetInner({
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
}: ProviderBackedChatWidgetInnerProps) {
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

  const onKeyDown = useCallback((e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }, [send]);

  const onNewConversation = useCallback(() => {
    client.reset();
  }, [client]);

  const mergedRenderers: ChatWidgetRenderers = useMemo(
    () => resolveTimelineRenderers({
      ...renderers,
      tool_call: ProviderToolCallRenderer,
      widget: ProviderWidgetRenderer,
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
      <WebChatProviderCapabilities />
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
    </div>
  );
}

export function ProviderBackedChatWidget(props: ChatWidgetProps) {
  const dispatch = useAppDispatch();
  const appProfile = useAppSelector((s) => s.app.profile);
  const { data: profileData, refetch: refetchProfile } = useGetProfileQuery();
  const { data: profilesData } = useGetProfilesQuery();
  const [setProfile] = useSetProfileMutation();

  const profileOptions = useMemo(() => {
    const bySlug = new Map<string, ProfileInfo>();
    for (const profile of profilesData ?? []) {
      const slug = String(profile?.slug ?? '').trim();
      if (!slug || bySlug.has(slug)) continue;
      bySlug.set(slug, profile);
    }
    const serverSlug = String(profileData?.slug ?? '').trim();
    if (serverSlug && !bySlug.has(serverSlug)) bySlug.set(serverSlug, { slug: serverSlug });
    if (bySlug.size === 0) bySlug.set('default', { slug: 'default' });
    return Array.from(bySlug.values());
  }, [profileData?.slug, profilesData]);

  const selectedProfile = useMemo(
    () => resolveSelectedProfile({ appProfile, serverProfile: profileData?.slug, profiles: profileOptions }),
    [appProfile, profileData?.slug, profileOptions],
  );

  useEffect(() => {
    if (selectedProfile !== appProfile) dispatch(appSlice.actions.setProfile(selectedProfile));
  }, [appProfile, dispatch, selectedProfile]);

  const onProfileChange = useCallback(async (nextProfile: string) => {
    const profile = nextProfile.trim();
    if (!profile || profile === selectedProfile) return;
    const selectedOption = profileOptions.find((candidate) => String(candidate?.slug ?? '').trim() === profile);
    const registry = String(selectedOption?.registry ?? profileData?.registry ?? 'default').trim() || 'default';
    try {
      const res = await setProfile({ profile, registry }).unwrap();
      const serverSlug = String(res.slug ?? res.profile ?? '').trim();
      if (serverSlug) dispatch(appSlice.actions.setProfile(serverSlug));
    } catch (err) {
      logWarn('profile switch failed', { scope: 'profiles.switch', extra: { profile } }, err);
      try {
        const refreshed = await refetchProfile().unwrap();
        const refreshedSlug = String(refreshed.slug ?? refreshed.profile ?? '').trim();
        if (refreshedSlug) dispatch(appSlice.actions.setProfile(refreshedSlug));
      } catch {
        // Keep current profile if refresh fails.
      }
    }
  }, [dispatch, profileData?.registry, profileOptions, refetchProfile, selectedProfile, setProfile]);

  const basePrefix = basePrefixFromLocation();
  const config = useMemo(() => ({
    basePrefix,
    sessionIdParam: 'sessionId',
    sessionStorageKey: 'pinocchio.web-chat.sessionId',
    onSessionIdChange: setSessionIdInLocation,
    createSessionBody: () => ({ profile: selectedProfile }),
    sendMessageBody: ({ prompt }: { prompt: string }) => ({ prompt, profile: selectedProfile }),
  }), [basePrefix, selectedProfile]);

  const headerTitle = profileData?.slug ? `Web Chat (${profileData.slug})` : 'Web Chat';

  return (
    <ChatProvider config={config}>
      <ProviderBackedChatWidgetInner
        {...props}
        selectedProfile={selectedProfile}
        profileOptions={profileOptions as ProfileInfo[]}
        profileTitle={headerTitle}
        onProfileChange={onProfileChange}
      />
    </ChatProvider>
  );
}
