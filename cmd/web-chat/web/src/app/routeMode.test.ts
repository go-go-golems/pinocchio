import { describe, expect, it } from 'vitest';
import { routeModeFromLocation, routeModeFromSearch } from './routeMode';

describe('routeModeFromSearch', () => {
  it('defaults to production chat mode', () => {
    expect(routeModeFromSearch('')).toEqual({ kind: 'chat' });
    expect(routeModeFromSearch('?sessionId=abc')).toEqual({ kind: 'chat' });
  });

  it('detects debug mode', () => {
    expect(routeModeFromSearch('?debug=1')).toEqual({ kind: 'debug' });
  });

  it('detects provider demo mode', () => {
    expect(routeModeFromSearch('?providerDemo=1')).toEqual({ kind: 'provider-demo' });
  });

  it('detects provider multi-demo mode', () => {
    expect(routeModeFromSearch('?providerMultiDemo=1')).toEqual({ kind: 'provider-multi-demo' });
  });

  it('ignores flags that are not exactly enabled', () => {
    expect(routeModeFromSearch('?debug=true&providerDemo=0&providerMultiDemo=yes')).toEqual({ kind: 'chat' });
  });

  it('uses deterministic priority when multiple dev flags are present', () => {
    expect(routeModeFromSearch('?providerMultiDemo=1&providerDemo=1&debug=1')).toEqual({ kind: 'debug' });
    expect(routeModeFromSearch('?providerMultiDemo=1&providerDemo=1')).toEqual({ kind: 'provider-demo' });
  });
});

describe('routeModeFromLocation', () => {
  it('reads search from a Location-like object', () => {
    expect(routeModeFromLocation({ search: '?providerDemo=1' } as Location)).toEqual({ kind: 'provider-demo' });
  });
});
