import { describe, expect, it } from 'vitest';
import type { ProfileInfo } from '../store/profileApi';
import { resolveSelectedProfile } from './profileSelection';

describe('resolveSelectedProfile', () => {
  const profiles: ProfileInfo[] = [{ slug: 'default' }, { slug: 'inventory' }, { slug: 'planner' }];

  it('uses default profile when no app/server profile is available', () => {
    expect(
      resolveSelectedProfile({
        appProfile: '',
        serverProfile: '',
        profiles,
      }),
    ).toBe('default');
  });

  it('keeps currently selected app profile when available', () => {
    expect(
      resolveSelectedProfile({
        appProfile: 'inventory',
        serverProfile: 'default',
        profiles,
      }),
    ).toBe('inventory');
  });

  it('falls back to server profile when app profile is stale', () => {
    expect(
      resolveSelectedProfile({
        appProfile: 'removed-profile',
        serverProfile: 'default',
        profiles,
      }),
    ).toBe('default');
  });

  it('falls back to first available profile when default is absent', () => {
    expect(
      resolveSelectedProfile({
        appProfile: '',
        serverProfile: '',
        profiles: [{ slug: 'planner' }, { slug: 'inventory' }],
      }),
    ).toBe('planner');
  });
});
