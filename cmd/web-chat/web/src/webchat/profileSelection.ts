import type { ProfileInfo } from '../store/profileApi';

function normalizeSlug(value: string | undefined | null): string {
  return String(value ?? '').trim();
}

export function resolveSelectedProfile(params: {
  appProfile?: string;
  serverProfile?: string;
  profiles: ProfileInfo[];
}): string {
  const profiles = Array.isArray(params.profiles) ? params.profiles : [];
  const available = new Set(
    profiles
      .map((profile) => normalizeSlug(profile.slug))
      .filter((slug) => slug.length > 0),
  );

  const appProfile = normalizeSlug(params.appProfile);
  if (appProfile && available.has(appProfile)) {
    return appProfile;
  }

  const serverProfile = normalizeSlug(params.serverProfile);
  if (serverProfile && available.has(serverProfile)) {
    return serverProfile;
  }

  if (available.has('default')) {
    return 'default';
  }

  for (const profile of profiles) {
    const slug = normalizeSlug(profile.slug);
    if (slug) {
      return slug;
    }
  }

  if (serverProfile) {
    return serverProfile;
  }
  if (appProfile) {
    return appProfile;
  }
  return 'default';
}
