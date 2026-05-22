"use client";

import { useEffect, useRef } from "react";

/*
useVisiblePoll runs `poll` once on mount and then on every
`intervalMs` tick, BUT pauses while the tab is hidden. Resumes with
an immediate poll when the tab becomes visible again so the user
doesn't see stale data on re-focus.

Without this, a backgrounded tab keeps hammering /api/trades/today
(60s) and /api/quotes/live (15s) for the entire trading day, and
multiple stale tabs multiply load against the Schwab-backed
endpoint. The visibilitychange listener also triggers an immediate
refresh on return so the UI doesn't carry stale data forward.

`poll` is captured in a ref so the effect doesn't re-subscribe on
every render, callers don't need to memoize it.
*/
export function useVisiblePoll(poll: () => void, intervalMs: number, enabled = true): void {
  const pollRef = useRef(poll);
  pollRef.current = poll;

  useEffect(() => {
    if (!enabled) return;
    if (typeof window === "undefined" || typeof document === "undefined") return;

    const tick = () => {
      if (document.hidden) return;
      pollRef.current();
    };
    tick();

    let interval = window.setInterval(tick, intervalMs);

    const onVisibility = () => {
      if (document.hidden) return;
      pollRef.current();
      window.clearInterval(interval);
      interval = window.setInterval(tick, intervalMs);
    };
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      window.clearInterval(interval);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [intervalMs, enabled]);
}
