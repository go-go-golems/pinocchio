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

function maxScrollTop(container: HTMLElement): number {
  return Math.max(0, container.scrollHeight - container.clientHeight);
}

function distanceFromBottom(container: HTMLElement): number {
  return maxScrollTop(container) - container.scrollTop;
}

function scrollDebugEnabled(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem('pinocchio.debugScroll') === '1';
  } catch {
    return false;
  }
}

function rounded(value: number): number {
  return Math.round(value * 100) / 100;
}

function scrollMetrics(container: HTMLElement) {
  return {
    scrollTop: rounded(container.scrollTop),
    scrollHeight: container.scrollHeight,
    clientHeight: container.clientHeight,
    maxScrollTop: rounded(maxScrollTop(container)),
    distanceFromBottom: rounded(distanceFromBottom(container)),
  };
}

function logScrollDebug(event: string, payload: Record<string, unknown> = {}) {
  if (!scrollDebugEnabled()) return;
  // Keep these logs easy to filter in DevTools.
  console.debug('[pinocchio-scroll]', event, payload);
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
  const programmaticReleaseTimerRef = useRef<number | null>(null);
  const lastScrollTopRef = useRef(0);
  const previousResetKeyRef = useRef(resetKey);
  const previousContentVersionRef = useRef<string | number | undefined>(undefined);

  const scrollToBottom = useCallback((_behavior: ScrollBehavior = 'auto') => {
    const container = containerRef.current;
    if (!container) return;

    const target = maxScrollTop(container);
    if (Math.abs(container.scrollTop - target) <= 1) {
      lastScrollTopRef.current = container.scrollTop;
      logScrollDebug('scrollToBottom.skip-already-bottom', { target: rounded(target), ...scrollMetrics(container) });
      return;
    }

    logScrollDebug('scrollToBottom.start', {
      requestedBehavior: _behavior,
      target: rounded(target),
      ...scrollMetrics(container),
    });

    isProgrammaticScrollRef.current = true;
    if (programmaticReleaseTimerRef.current !== null) {
      window.clearTimeout(programmaticReleaseTimerRef.current);
    }

    // Use an immediate, clamped scroll target. Smooth scrolling emits a long
    // sequence of scroll events that can race with streaming token updates and
    // with the jump-to-latest pill appearing/disappearing, causing visible jitter.
    container.scrollTo({ top: target, behavior: 'auto' });

    window.requestAnimationFrame(() => {
      lastScrollTopRef.current = container.scrollTop;
      logScrollDebug('scrollToBottom.after-raf', { ...scrollMetrics(container) });
      programmaticReleaseTimerRef.current = window.setTimeout(() => {
        isProgrammaticScrollRef.current = false;
        programmaticReleaseTimerRef.current = null;
        const latest = containerRef.current;
        if (latest) {
          lastScrollTopRef.current = latest.scrollTop;
          logScrollDebug('scrollToBottom.release-guard', { ...scrollMetrics(latest) });
        }
      }, 80);
    });
  }, []);

  const jumpToLatest = useCallback(() => {
    logScrollDebug('jumpToLatest');
    setMode('following');
    scrollToBottom('auto');
  }, [scrollToBottom]);

  const onWheel = useCallback<WheelEventHandler<HTMLElement>>((event) => {
    if (!enabled || isProgrammaticScrollRef.current) return;
    if (event.deltaY < 0) {
      logScrollDebug('wheel.detach-up', { deltaY: rounded(event.deltaY) });
      setMode('detached');
    }
  }, [enabled]);

  const onScroll = useCallback<UIEventHandler<HTMLElement>>((event) => {
    if (!enabled) return;
    const container = event.currentTarget;
    if (isProgrammaticScrollRef.current) {
      logScrollDebug('scroll.ignored-programmatic', { ...scrollMetrics(container) });
      return;
    }

    const currentScrollTop = container.scrollTop;
    const previousScrollTop = lastScrollTopRef.current;
    lastScrollTopRef.current = currentScrollTop;
    const distance = distanceFromBottom(container);

    logScrollDebug('scroll.user', {
      mode,
      previousScrollTop: rounded(previousScrollTop),
      currentScrollTop: rounded(currentScrollTop),
      distance: rounded(distance),
      attachThresholdPx,
      detachThresholdPx,
      ...scrollMetrics(container),
    });

    setMode((currentMode) => {
      if (currentMode === 'following') {
        // If layout shrinks (for example when the Jump to latest pill is removed),
        // the browser may clamp scrollTop downward while still being exactly at the
        // bottom. That looks like an upward scroll numerically, but it is not user
        // intent and must not detach.
        if (distance <= attachThresholdPx) {
          return currentMode;
        }
        // Only detach when the user scrolls upward and ends up away from bottom.
        if (currentScrollTop < previousScrollTop - 2) {
          logScrollDebug('mode.following-to-detached', {
            reason: 'upward-scroll-away-from-bottom',
            previousScrollTop: rounded(previousScrollTop),
            currentScrollTop: rounded(currentScrollTop),
            distance: rounded(distance),
          });
          return 'detached';
        }
        return currentMode;
      }
      if (distance <= attachThresholdPx) {
        logScrollDebug('mode.detached-to-following', {
          reason: 'near-bottom',
          distance: rounded(distance),
          attachThresholdPx,
        });
        return 'following';
      }
      return currentMode;
    });
  }, [attachThresholdPx, detachThresholdPx, enabled, mode]);

  useEffect(() => {
    if (previousResetKeyRef.current === resetKey) {
      return;
    }
    logScrollDebug('resetKey.changed', { previous: previousResetKeyRef.current, next: resetKey });
    previousResetKeyRef.current = resetKey;
    setMode('following');
  }, [resetKey]);

  useLayoutEffect(() => {
    if (!enabled || mode !== 'following' || !contentVersion) {
      previousContentVersionRef.current = contentVersion;
      return;
    }
    if (previousContentVersionRef.current === contentVersion) {
      return;
    }
    previousContentVersionRef.current = contentVersion;
    logScrollDebug('contentVersion.changed', { contentVersion, isStreaming, mode });
    scrollToBottom('auto');
  }, [contentVersion, enabled, isStreaming, mode, scrollToBottom]);

  useLayoutEffect(() => {
    if (!enabled || mode !== 'following' || !tailRef.current || typeof MutationObserver === 'undefined') {
      return;
    }
    const timeline = tailRef.current.parentElement;
    if (!(timeline instanceof HTMLElement)) return;
    const observer = new MutationObserver((mutations) => {
      logScrollDebug('mutation', { mutationCount: mutations.length, mode });
      scrollToBottom('auto');
    });
    observer.observe(timeline, { childList: true, subtree: true, characterData: true });
    return () => observer.disconnect();
  }, [enabled, mode, scrollToBottom]);

  useEffect(() => {
    return () => {
      if (programmaticReleaseTimerRef.current !== null) {
        window.clearTimeout(programmaticReleaseTimerRef.current);
      }
    };
  }, []);

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
