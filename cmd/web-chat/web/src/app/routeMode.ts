export type WebChatRouteMode =
  | { kind: 'chat' }
  | { kind: 'debug' }
  | { kind: 'provider-demo' }
  | { kind: 'provider-multi-demo' };

function flagIsEnabled(params: URLSearchParams, name: string): boolean {
  return params.get(name) === '1';
}

export function routeModeFromSearch(search: string): WebChatRouteMode {
  const normalized = search.startsWith('?') ? search.slice(1) : search;
  const params = new URLSearchParams(normalized);

  if (flagIsEnabled(params, 'debug')) return { kind: 'debug' };
  if (flagIsEnabled(params, 'providerDemo')) return { kind: 'provider-demo' };
  if (flagIsEnabled(params, 'providerMultiDemo')) return { kind: 'provider-multi-demo' };
  return { kind: 'chat' };
}

export function routeModeFromLocation(location: Pick<Location, 'search'>): WebChatRouteMode {
  return routeModeFromSearch(location.search);
}
