import { ChatProvider } from '@go-go-golems/chat-provider';
import { useCallback, useEffect, useMemo } from 'react';
import { appSlice } from '../../../store/appSlice';
import { useAppDispatch, useAppSelector } from '../../../store/hooks';
import { type ProfileInfo, useGetProfileQuery, useGetProfilesQuery, useSetProfileMutation } from '../../../store/profileApi';
import { basePrefixFromLocation } from '../../../utils/basePrefix';
import { logWarn } from '../../../utils/logger';
import { resolveSelectedProfile } from '../profileSelection';
import '../styles/index.css';
import { pinocchioWebChatTimelineAdapters } from '../extensions/pinocchio-timeline-adapters';
import { setSessionIdInLocation } from '../provider-support/providerSession';
import { WebChatApp } from '../WebChatApp';
import type { WebChatProviderShellProps } from './types';

export function WebChatProviderShell(props: WebChatProviderShellProps) {
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

  const onProfileChange = useCallback(
    async (nextProfile: string) => {
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
    },
    [dispatch, profileData?.registry, profileOptions, refetchProfile, selectedProfile, setProfile],
  );

  const basePrefix = basePrefixFromLocation();
  const config = useMemo(
    () => ({
      basePrefix,
      sessionIdParam: 'sessionId',
      sessionStorageKey: 'pinocchio.web-chat.sessionId',
      onSessionIdChange: setSessionIdInLocation,
      extensions: [pinocchioWebChatTimelineAdapters],
      createSessionBody: () => ({ profile: selectedProfile }),
      sendMessageBody: ({ prompt }: { prompt: string }) => ({ prompt, profile: selectedProfile }),
    }),
    [basePrefix, selectedProfile],
  );

  const headerTitle = profileData?.slug ? `Web Chat (${profileData.slug})` : 'Web Chat';

  return (
    <ChatProvider config={config}>
      <WebChatApp
        {...props}
        selectedProfile={selectedProfile}
        profileOptions={profileOptions as ProfileInfo[]}
        profileTitle={headerTitle}
        onProfileChange={onProfileChange}
      />
    </ChatProvider>
  );
}
