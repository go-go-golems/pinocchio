import type { KeyboardEvent } from 'react';
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { appSlice } from '../store/appSlice';
import { errorsSlice, makeAppError } from '../store/errorsSlice';
import { useAppDispatch, useAppSelector } from '../store/hooks';
import { type ProfileInfo, useGetProfileQuery, useGetProfilesQuery, useSetProfileMutation } from '../store/profileApi';
import { selectTimelineEntities, timelineSlice } from '../store/timelineSlice';
import { basePrefixFromLocation } from '../utils/basePrefix';
import { logWarn } from '../utils/logger';
import { wsManager } from '../ws/wsManager';
import { DefaultComposer } from './components/Composer';
import { DefaultHeader } from './components/Header';
import { DefaultStatusbar } from './components/Statusbar';
import { ChatTimeline } from './components/Timeline';
import { getPartProps, mergeClassName, mergeStyle } from './parts';
import { resolveSelectedProfile } from './profileSelection';
import { resolveTimelineRenderers } from './rendererRegistry';
import type {
  ChatWidgetComponents,
  ChatWidgetProps,
  ChatWidgetRenderers,
  RenderEntity,
} from './types';
import './styles/theme-default.css';
import './styles/webchat.css';

const ATTACH_THRESHOLD_PX = 24;
const DETACH_THRESHOLD_PX = 48;

type ScrollMode = 'following' | 'detached';

function distanceFromBottom(container: HTMLElement): number {
  return container.scrollHeight - container.clientHeight - container.scrollTop;
}

function sessionIdFromLocation(): string {
  try {
    const u = new URL(window.location.href);
    const q = u.searchParams.get('sessionId') || '';
    return q.trim();
  } catch (err) {
    logWarn('sessionIdFromLocation failed', { scope: 'sessionIdFromLocation' }, err);
    return '';
  }
}

function setSessionIdInLocation(sessionId: string | null) {
  try {
    const u = new URL(window.location.href);
    if (!sessionId) {
      u.searchParams.delete('sessionId');
    } else {
      u.searchParams.set('sessionId', sessionId);
    }
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

export function ChatWidget({
  unstyled,
  theme = 'default',
  themeVars,
  className,
  rootProps,
  partProps,
  components,
  renderers,
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
  const [scrollMode, setScrollMode] = useState<ScrollMode>('following');
  const mainRef = useRef<HTMLElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const isProgrammaticScrollRef = useRef(false);

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
    if (serverSlug && !bySlug.has(serverSlug)) {
      bySlug.set(serverSlug, { slug: serverSlug });
    }
    if (bySlug.size === 0) {
      bySlug.set('default', { slug: 'default' });
    }
    return Array.from(bySlug.values());
  }, [profileData?.slug, profilesData]);

  const selectedProfile = useMemo(
    () =>
      resolveSelectedProfile({
        appProfile: app.profile,
        serverProfile: profileData?.slug,
        profiles: profileOptions,
      }),
    [app.profile, profileData?.slug, profileOptions],
  );

  useEffect(() => {
    if (selectedProfile !== app.profile) {
      dispatch(appSlice.actions.setProfile(selectedProfile));
    }
  }, [app.profile, dispatch, selectedProfile]);

  useEffect(() => {
    const sessionId = sessionIdFromLocation();
    if (sessionId && sessionId !== app.convId) {
      dispatch(appSlice.actions.setConvId(sessionId));
    }
  }, [app.convId, dispatch]);

  useEffect(() => {
    const sessionId = app.convId || sessionIdFromLocation();
    if (!sessionId) return;
    if (sessionId !== app.convId) {
      dispatch(appSlice.actions.setConvId(sessionId));
      return;
    }

    const basePrefix = basePrefixFromLocation();
    void wsManager.connect({
      sessionId,
      basePrefix,
      dispatch,
      onStatus: (s) => setStatusText(s),
      hydrate: true,
    });
    return () => {
      wsManager.disconnect();
    };
  }, [app.convId, dispatch]);

  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'auto') => {
    const container = mainRef.current;
    if (!container) return;
    isProgrammaticScrollRef.current = true;
    container.scrollTo({ top: container.scrollHeight, behavior });
    window.requestAnimationFrame(() => {
      isProgrammaticScrollRef.current = false;
    });
  }, []);

  const onMainScroll = useCallback(() => {
    const container = mainRef.current;
    if (!container || isProgrammaticScrollRef.current) return;
    const distance = distanceFromBottom(container);
    setScrollMode((current) => {
      if (current === 'following' && distance > DETACH_THRESHOLD_PX) {
        return 'detached';
      }
      if (current === 'detached' && distance <= ATTACH_THRESHOLD_PX) {
        return 'following';
      }
      return current;
    });
  }, []);

  const jumpToLatest = useCallback(() => {
    setScrollMode('following');
    scrollToBottom('auto');
  }, [scrollToBottom]);

  useEffect(() => {
    if (app.convId) {
      setScrollMode('following');
      return;
    }
    setScrollMode('following');
  }, [app.convId]);

  useLayoutEffect(() => {
    if (!entityCount || scrollMode !== 'following') return;
    scrollToBottom(app.status === 'streaming' ? 'auto' : 'smooth');
  }, [app.status, entityCount, scrollMode, scrollToBottom]);

  useLayoutEffect(() => {
    if (
      !entityCount ||
      scrollMode !== 'following' ||
      !mainRef.current ||
      !bottomRef.current ||
      typeof MutationObserver === 'undefined'
    ) {
      return;
    }
    const timeline = bottomRef.current.parentElement;
    if (!(timeline instanceof HTMLElement)) return;
    const observer = new MutationObserver(() => {
      scrollToBottom('auto');
    });
    observer.observe(timeline, { childList: true, subtree: true, characterData: true });
    return () => observer.disconnect();
  }, [entityCount, scrollMode, scrollToBottom]);

  const send = useCallback(() => {
    if (!text.trim()) return;
    const prompt = text;
    const basePrefix = basePrefixFromLocation();

    setText('');
    void (async () => {
      let sessionId = app.convId || sessionIdFromLocation();
      if (!sessionId) {
        try {
          const createRes = await fetch(`${basePrefix}/api/chat/sessions`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ profile: selectedProfile }),
          });
          if (!createRes.ok) {
            const msg = await createRes.text();
            dispatch(errorsSlice.actions.reportError(makeAppError(msg, 'session.create', undefined, { status: createRes.status })));
            return;
          }
          const created = await createRes.json() as { sessionId?: string };
          sessionId = String(created?.sessionId ?? '').trim();
          if (!sessionId) {
            dispatch(errorsSlice.actions.reportError(makeAppError('session create failed', 'session.create')));
            return;
          }
          dispatch(appSlice.actions.setConvId(sessionId));
          setSessionIdInLocation(sessionId);
        } catch (err) {
          dispatch(errorsSlice.actions.reportError(makeAppError('session create failed', 'session.create', err)));
          return;
        }
      }

      try {
        await wsManager.connect({
          sessionId,
          basePrefix,
          dispatch,
          onStatus: (s) => setStatusText(s),
          hydrate: true,
        });
      } catch (err) {
        dispatch(errorsSlice.actions.reportError(makeAppError('ws connect failed', 'send.ws', err, { sessionId })));
        return;
      }

      try {
        const res = await fetch(`${basePrefix}/api/chat/sessions/${encodeURIComponent(sessionId)}/messages`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            prompt,
            profile: selectedProfile,
          }),
        });
        if (!res.ok) {
          const msg = await res.text();
          dispatch(errorsSlice.actions.reportError(makeAppError(msg, 'send', undefined, { status: res.status })));
          return;
        }
        const body = await res.json() as { status?: string };
        const status = String(body?.status ?? '').trim();
        if (status) {
          dispatch(appSlice.actions.setStatus(status));
        }
      } catch (err) {
        dispatch(errorsSlice.actions.reportError(makeAppError('send failed', 'send', err)));
      }
    })();
  }, [app.convId, dispatch, selectedProfile, text]);

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
      if (!profile || profile === selectedProfile) return;
      const selectedOption = profileOptions.find((candidate) => String(candidate?.slug ?? '').trim() === profile);
      const registry = String(selectedOption?.registry ?? profileData?.registry ?? 'default').trim() || 'default';
      try {
        const res = await setProfile({ profile, registry }).unwrap();
        const serverSlug = String(res.slug ?? res.profile ?? '').trim();
        if (serverSlug) {
          dispatch(appSlice.actions.setProfile(serverSlug));
        }
      } catch (err) {
        logWarn('profile switch failed', { scope: 'profiles.switch', extra: { profile } }, err);
        try {
          const refreshed = await refetchProfile().unwrap();
          const refreshedSlug = String(refreshed.slug ?? refreshed.profile ?? '').trim();
          if (refreshedSlug) {
            dispatch(appSlice.actions.setProfile(refreshedSlug));
          }
        } catch {
          // Keep current selection if profile refresh fails.
        }
      }
    },
    [dispatch, profileData?.registry, profileOptions, refetchProfile, selectedProfile, setProfile],
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
    setScrollMode('following');
    dispatch(appSlice.actions.setConvId(''));
    dispatch(appSlice.actions.setStatus('idle'));
    dispatch(appSlice.actions.setWsStatus('disconnected'));
    dispatch(appSlice.actions.setLastSeq(0));
    dispatch(appSlice.actions.setQueueDepth(0));
    dispatch(timelineSlice.actions.clear());
    setSessionIdInLocation(null);
  }, [dispatch]);

  const mergedRenderers: ChatWidgetRenderers = useMemo(
    () => resolveTimelineRenderers(renderers),
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
          profile={selectedProfile}
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
          profile={selectedProfile}
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

      <main data-part="main" ref={mainRef} onScroll={onMainScroll}>
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
