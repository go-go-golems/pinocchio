import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
  basePrefixFromLocation,
  clearRuntimeBasePrefix,
  routerBasenameFromRuntimeConfig,
  setRuntimeBasePrefix,
} from './basePrefix';

describe('basePrefix helpers', () => {
  const originalWindow = (globalThis as { window?: Window }).window;

  beforeEach(() => {
    clearRuntimeBasePrefix();
  });

  afterEach(() => {
    clearRuntimeBasePrefix();
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

  it('runtime override has higher precedence than runtime config', () => {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: {
        location: { pathname: '/' },
        __PINOCCHIO_WEBCHAT_CONFIG__: { basePrefix: '/chat' },
      },
    });
    setRuntimeBasePrefix('/other');
    expect(basePrefixFromLocation()).toBe('/other');
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
