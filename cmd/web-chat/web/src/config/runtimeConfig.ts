export interface WebChatRuntimeConfig {
  basePrefix?: string;
  debugApiEnabled?: boolean;
}

declare global {
  interface Window {
    __PINOCCHIO_WEBCHAT_CONFIG__?: WebChatRuntimeConfig;
  }
}

function asConfig(value: unknown): WebChatRuntimeConfig {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return {};
  }
  return value as WebChatRuntimeConfig;
}

export function getRuntimeConfig(): WebChatRuntimeConfig {
  if (typeof window === 'undefined') {
    return {};
  }
  return asConfig(window.__PINOCCHIO_WEBCHAT_CONFIG__);
}
