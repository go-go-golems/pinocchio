import { afterEach, describe, expect, it } from 'vitest';
import {
  basePrefixFromLocation,
  routerBasenameFromRuntimeConfig,
} from './basePrefix';

describe('basePrefix helpers', () => {
  const originalWindow = (globalThis as { window?: Window }).window;

  afterEach(() => {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: originalWindow,
    });
  });

  it('uses runtime config basePrefix when provided', () => {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        location: { pathname: '/' },
        __PINOCCHIO_WEBCHAT_CONFIG__: { basePrefix: '/chat' },
      },
    });

    expect(basePrefixFromLocation()).toBe('/chat');
  });

  it('falls back to first location segment when runtime config is missing', () => {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        location: { pathname: '/chat/inspect' },
      },
    });
    expect(basePrefixFromLocation()).toBe('/chat');
  });

  it('only applies router basename when location is under configured prefix', () => {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        location: { pathname: '/chat/timeline' },
        __PINOCCHIO_WEBCHAT_CONFIG__: { basePrefix: '/chat' },
      },
    });
    expect(routerBasenameFromRuntimeConfig()).toBe('/chat');

    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        location: { pathname: '/' },
        __PINOCCHIO_WEBCHAT_CONFIG__: { basePrefix: '/chat' },
      },
    });
    expect(routerBasenameFromRuntimeConfig()).toBeUndefined();
  });
});
