import type { KeyboardEvent } from 'react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { appSlice } from '../store/appSlice';
import { errorsSlice, makeAppError } from '../store/errorsSlice';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { type ProfileInfo, useGetProfileQuery, useGetProfilesQuery, useSetProfileMutation } from '../store/profileApi';
import { selectTimelineEntities, timelineSlice } from '../store/timelineSlice';
import { basePrefixFromLocation } from '../utils/basePrefix';
import { logWarn } from '../utils/logger';
import { wsManager } from '../ws/wsManager';
import {
  GenericCard,
  LogCard,
  MessageCard,
  ThinkingModeCard,
  ToolCallCard,
  ToolResultCard,
} from './cards';
import { DefaultComposer } from './components/Composer';
import { DefaultHeader } from './components/Header';
import { DefaultStatusbar } from './components/Statusbar';
import { ChatTimeline } from './components/Timeline';
import { getPartProps, mergeClassName, mergeStyle } from './parts';
import type {
  ChatWidgetComponents,
  ChatWidgetProps,
  ChatWidgetRenderers,
  RenderEntity,
} from './types';
import './styles/theme-default.css';
import './styles/webchat.css';

function convIdFromLocation(): string {
  try {
    const u = new URL(window.location.href);
    const q = u.searchParams.get('conv_id') || u.searchParams.get('convId') || '';
    return q.trim();
  } catch (err) {
    logWarn('convIdFromLocation failed', { scope: 'convIdFromLocation' }, err);
    return '';
  }
}

function setConvIdInLocation(convId: string | null) {
  try {
    const u = new URL(window.location.href);
    if (!convId) {
      u.searchParams.delete('conv_id');
      u.searchParams.delete('convId');
    } else {
      u.searchParams.set('conv_id', convId);
      u.searchParams.delete('convId');
    }
    window.history.replaceState({}, '', u.toString());
  } catch (err) {
    logWarn('setConvIdInLocation failed', { scope: 'setConvIdInLocation' }, err);
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

export function ChatWidget({
  unstyled,
  theme = 'default',
  themeVars,
  className,
  rootProps,
  partProps,
  components,
  renderers,
  buildOverrides,
}: ChatWidgetProps) {
  const dispatch = useAppDispatch();
  const app = useAppSelector((s) => s.app);
  const errors = useAppSelector((s) => s.errors);
  const errorCount = errors.length;
  const entitiesRaw = useAppSelector(selectTimelineEntities);

  const entities = useMemo(() => entitiesRaw.map(toRenderEntity), [entitiesRaw]);
  const entityCount = entities.length;

  const [text, setText] = useState('');
  const [showErrors, setShowErrors] = useState(false);
  const [statusText, setStatusText] = useState('idle');
  const bottomRef = useRef<HTMLDivElement>(null);

  const { data: profileData } = useGetProfileQuery();
  const { data: profilesData } = useGetProfilesQuery();
  const [setProfile] = useSetProfileMutation();

  const profileOptions = useMemo(() => {
    const data = profilesData ?? [];
    if (data.length) return data;
    return [{ slug: 'default' }, { slug: 'agent' }];
  }, [profilesData]);

  useEffect(() => {
    const convId = convIdFromLocation();
    if (convId && convId !== app.convId) {
      dispatch(appSlice.actions.setConvId(convId));
    }
  }, [app.convId, dispatch]);

  useEffect(() => {
    const convId = app.convId || convIdFromLocation();
    if (!convId) return;
    if (convId !== app.convId) {
      dispatch(appSlice.actions.setConvId(convId));
      return;
    }

    const basePrefix = basePrefixFromLocation();
    void wsManager.connect({
      convId,
      basePrefix,
      dispatch,
      onStatus: (s) => setStatusText(s),
      hydrate: true,
    });
    return () => {
      wsManager.disconnect();
    };
  }, [app.convId, dispatch]);

  useEffect(() => {
    if (!entityCount) return;
    if (bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth', block: 'end' });
    }
  }, [entityCount]);

  const send = useCallback(() => {
    if (!text.trim()) return;
    if (!app.convId) {
      const next = crypto.randomUUID();
      dispatch(appSlice.actions.setConvId(next));
      setConvIdInLocation(next);
    }
    const basePrefix = basePrefixFromLocation();
    const overrides = buildOverrides?.();
    const payload: Record<string, any> = {
      conv_id: app.convId || convIdFromLocation(),
      prompt: text,
    };
    if (overrides && Object.keys(overrides).length > 0) {
      payload.overrides = overrides;
    }
    setText('');
    void fetch(`${basePrefix}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
      .then(async (res) => {
        if (!res.ok) {
          const msg = await res.text();
          dispatch(errorsSlice.actions.reportError(makeAppError(msg, 'send', undefined, { status: res.status })));
        }
      })
      .catch((err) => {
        dispatch(errorsSlice.actions.reportError(makeAppError('send failed', 'send', err)));
      });
  }, [app.convId, buildOverrides, dispatch, text]);

  const onKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        send();
      }
    },
    [send],
  );

  const onProfileChange = useCallback(
    async (nextProfile: string) => {
      const profile = nextProfile.trim();
      if (!profile || profile === app.profile) return;
      try {
        const res = await setProfile({ slug: profile }).unwrap();
        dispatch(appSlice.actions.setProfile(res.slug));
      } catch (err) {
        logWarn('profile switch failed', { scope: 'profiles.switch', extra: { profile } }, err);
      }
    },
    [app.profile, dispatch, setProfile],
  );

  const toggleErrors = useCallback(() => {
    setShowErrors((prev) => !prev);
  }, []);

  const clearErrors = useCallback(() => {
    dispatch(errorsSlice.actions.clearErrors());
    setShowErrors(false);
  }, [dispatch]);

  const onNewConversation = useCallback(() => {
    wsManager.disconnect();
    dispatch(appSlice.actions.setConvId(''));
    dispatch(appSlice.actions.setStatus('idle'));
    dispatch(appSlice.actions.setWsStatus('disconnected'));
    dispatch(appSlice.actions.setLastSeq(0));
    dispatch(appSlice.actions.setQueueDepth(0));
    dispatch(timelineSlice.actions.clear());
    setConvIdInLocation(null);
  }, [dispatch]);

  const mergedRenderers: ChatWidgetRenderers = useMemo(
    () => ({
      message: MessageCard,
      tool_call: ToolCallCard,
      tool_result: ToolResultCard,
      log: LogCard,
      thinking_mode: ThinkingModeCard,
      default: GenericCard,
      ...(renderers ?? {}),
    }),
    [renderers],
  );

  const headerTitle = profileData?.slug ? `Web Chat (${profileData.slug})` : 'Web Chat';

  const HeaderOverride = (components as ChatWidgetComponents | undefined)?.Header;
  const StatusbarComponent = (components as ChatWidgetComponents | undefined)?.Statusbar ?? DefaultStatusbar;
  const ComposerComponent = (components as ChatWidgetComponents | undefined)?.Composer ?? DefaultComposer;

  const rootPartProps = getPartProps('root', partProps);
  const rootClassName = mergeClassName(className, rootPartProps.className, rootProps?.className);
  const rootStyle = mergeStyle(themeVars, rootPartProps.style, rootProps?.style);

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
          title={headerTitle}
          profile={app.profile || 'default'}
          profiles={profileOptions as ProfileInfo[]}
          wsStatus={app.wsStatus}
          status={app.status || statusText}
          queueDepth={app.queueDepth}
          lastSeq={app.lastSeq}
          errorCount={errorCount}
          showErrors={showErrors}
          onProfileChange={onProfileChange}
          onToggleErrors={toggleErrors}
          partProps={partProps}
        />
      ) : (
        <DefaultHeader
          title={headerTitle}
          profile={app.profile || 'default'}
          profiles={profileOptions as ProfileInfo[]}
          wsStatus={app.wsStatus}
          status={app.status || statusText}
          queueDepth={app.queueDepth}
          lastSeq={app.lastSeq}
          errorCount={errorCount}
          showErrors={showErrors}
          onProfileChange={onProfileChange}
          onToggleErrors={toggleErrors}
          Statusbar={StatusbarComponent as any}
          partProps={partProps}
        />
      )}

      <main data-part="main">
        <ChatTimeline
          entities={entities}
          errors={errors}
          showErrors={showErrors}
          errorCount={errorCount}
          onClearErrors={clearErrors}
          onToggleErrors={toggleErrors}
          renderers={mergedRenderers}
          bottomRef={bottomRef}
          partProps={partProps}
          state={app.status}
        />
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
