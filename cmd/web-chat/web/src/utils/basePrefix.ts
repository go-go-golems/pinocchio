import { getRuntimeConfig } from '../config/runtimeConfig';

function normalizePrefix(prefix: string): string {
  const trimmed = prefix.trim();
  if (!trimmed || trimmed === '/') {
    return '';
  }
  const withLeadingSlash = trimmed.startsWith('/') ? trimmed : `/${trimmed}`;
  return withLeadingSlash.replace(/\/+$/, '');
}

function configuredBasePrefix(): string {
  return normalizePrefix(getRuntimeConfig().basePrefix ?? '');
}

export function basePrefixFromLocation(): string {
  const configured = configuredBasePrefix();
  if (configured) {
    return configured;
  }
  if (typeof window === 'undefined') return '';
  const segs = window.location.pathname.split('/').filter(Boolean);
  return segs.length > 0 ? `/${segs[0]}` : '';
}

export function routerBasenameFromRuntimeConfig(): string | undefined {
  if (typeof window === 'undefined') {
    return undefined;
  }
  const configured = configuredBasePrefix();
  if (!configured) {
    return undefined;
  }
  const pathname = window.location.pathname;
  if (pathname === configured || pathname.startsWith(`${configured}/`)) {
    return configured;
  }
  return undefined;
}
