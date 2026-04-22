import { type UIEventHandler, useCallback, useEffect, useLayoutEffect, useRef, useState, type WheelEventHandler } from 'react';

const DEFAULT_ATTACH_THRESHOLD_PX = 24;
const DEFAULT_DETACH_THRESHOLD_PX = 48;

export type ScrollMode = 'following' | 'detached';

type UseStickyScrollFollowOptions = {
  enabled?: boolean;
  isStreaming?: boolean;
  contentVersion?: string | number;
  resetKey?: string | number;
  attachThresholdPx?: number;
  detachThresholdPx?: number;
};

function distanceFromBottom(container: HTMLElement): number {
  return container.scrollHeight - container.clientHeight - container.scrollTop;
}

export function useStickyScrollFollow({
  enabled = true,
  isStreaming = false,
  contentVersion,
  resetKey,
  attachThresholdPx = DEFAULT_ATTACH_THRESHOLD_PX,
  detachThresholdPx = DEFAULT_DETACH_THRESHOLD_PX,
}: UseStickyScrollFollowOptions) {
  const [mode, setMode] = useState<ScrollMode>('following');
  const containerRef = useRef<HTMLElement>(null);
  const tailRef = useRef<HTMLDivElement>(null);
  const isProgrammaticScrollRef = useRef(false);
  const lastScrollTopRef = useRef(0);

  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'auto') => {
    const container = containerRef.current;
    if (!container) return;
    isProgrammaticScrollRef.current = true;
    container.scrollTo({ top: container.scrollHeight, behavior });
    window.requestAnimationFrame(() => {
      isProgrammaticScrollRef.current = false;
      lastScrollTopRef.current = container.scrollTop;
    });
  }, []);

  const jumpToLatest = useCallback(() => {
    setMode('following');
    scrollToBottom('auto');
  }, [scrollToBottom]);

  const onWheel = useCallback<WheelEventHandler<HTMLElement>>((event) => {
    if (!enabled || isProgrammaticScrollRef.current) return;
    if (event.deltaY < 0) {
      setMode('detached');
    }
  }, [enabled]);

  const onScroll = useCallback<UIEventHandler<HTMLElement>>((event) => {
    if (!enabled || isProgrammaticScrollRef.current) return;
    const container = event.currentTarget;
    const currentScrollTop = container.scrollTop;
    const previousScrollTop = lastScrollTopRef.current;
    lastScrollTopRef.current = currentScrollTop;
    const distance = distanceFromBottom(container);

    setMode((currentMode) => {
      if (currentMode === 'following') {
        if (currentScrollTop < previousScrollTop || distance > detachThresholdPx) {
          return 'detached';
        }
        return currentMode;
      }
      if (distance <= attachThresholdPx) {
        return 'following';
      }
      return currentMode;
    });
  }, [attachThresholdPx, detachThresholdPx, enabled]);

  useEffect(() => {
    setMode('following');
  }, [resetKey]);

  useLayoutEffect(() => {
    if (!enabled || mode !== 'following' || !contentVersion) return;
    scrollToBottom(isStreaming ? 'auto' : 'smooth');
  }, [contentVersion, enabled, isStreaming, mode, scrollToBottom]);

  useLayoutEffect(() => {
    if (!enabled || mode !== 'following' || !tailRef.current || typeof MutationObserver === 'undefined') {
      return;
    }
    const timeline = tailRef.current.parentElement;
    if (!(timeline instanceof HTMLElement)) return;
    const observer = new MutationObserver(() => {
      scrollToBottom('auto');
    });
    observer.observe(timeline, { childList: true, subtree: true, characterData: true });
    return () => observer.disconnect();
  }, [contentVersion, enabled, mode, scrollToBottom]);

  return {
    containerRef,
    tailRef,
    mode,
    jumpToLatest,
    onScroll,
    onWheel,
    scrollToBottom,
  };
}
