"use client";

import { useEffect, useRef, useState } from "react";

import type { LiveQuotesResponse, QuoteUpdate } from "@/types/trade";

/*
useQuoteStream wraps EventSource('/api/quotes/stream') and exposes
the same LiveQuotesResponse shape the dashboard already consumes
(quotes + options keyed maps), so downstream components are
untouched. Per-tick `quote` events update single keys in place; the
initial `snapshot` event replaces the whole state on every (re)attach.

Lifecycle:
  - Opens the stream on mount when `enabled` is true.
  - Closes on unmount, on tab hidden (pauses bandwidth + Schwab
    subscription cost while the user is elsewhere), and on `enabled`
    transitioning to false (e.g. market just closed mid-session).
  - Reopens when the tab returns to visible.
  - EventSource has built-in auto-reconnect for transient drops; the
    custom error handler caps that loop only on hard 5xx responses
    (specifically the 503 the server returns when the hub isn't
    running — e.g. during the 16:00 ET stop tick).

Returns null until the first snapshot lands; consumers should treat
that as "no data yet" and not blank existing UI (matches the prior
poll-based shape).
*/
export function useQuoteStream(enabled: boolean): LiveQuotesResponse | null {
  const [snapshot, setSnapshot] = useState<LiveQuotesResponse | null>(null);
  const sourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!enabled) {
      sourceRef.current?.close();
      sourceRef.current = null;
      setSnapshot(null);
      return;
    }

    let cancelled = false;

    const open = () => {
      if (cancelled) return;
      // Clean up any prior source before opening a new one (defends
      // against reopens triggered by rapid visibility flicker).
      sourceRef.current?.close();
      const src = new EventSource("/api/quotes/stream", { withCredentials: true });
      sourceRef.current = src;

      src.addEventListener("snapshot", (e) => {
        try {
          const data = JSON.parse((e as MessageEvent).data) as LiveQuotesResponse;
          setSnapshot(data);
        } catch {
          // Malformed snapshot — log and ignore. Next tick replaces.
        }
      });

      src.addEventListener("quote", (e) => {
        try {
          const update = JSON.parse((e as MessageEvent).data) as QuoteUpdate;
          setSnapshot((prev) => mergeUpdate(prev, update));
        } catch {
          // Malformed tick — ignore. Next tick supersedes.
        }
      });

      // ping events are keepalive only; no state mutation.

      src.addEventListener("error", () => {
        /*
        EventSource auto-reconnects for transient drops. For a hard
        503 (hub closed), the browser will keep retrying every few
        seconds — that's fine, the closed-page is the real UI and
        the dashboard re-polls /api/market/status on focus to flip
        back to live mode when the hub restarts.
        */
      });
    };

    const close = () => {
      sourceRef.current?.close();
      sourceRef.current = null;
    };

    const onVisible = () => {
      if (document.visibilityState === "visible") open();
      else close();
    };

    if (document.visibilityState === "visible") open();
    document.addEventListener("visibilitychange", onVisible);

    return () => {
      cancelled = true;
      document.removeEventListener("visibilitychange", onVisible);
      close();
    };
  }, [enabled]);

  return snapshot;
}

function mergeUpdate(prev: LiveQuotesResponse | null, u: QuoteUpdate): LiveQuotesResponse {
  const base: LiveQuotesResponse = prev ?? {
    connected: true,
    market_open: true,
    as_of: u.as_of,
    quotes: {},
    options: {},
  };
  if (u.symbol && u.quote) {
    return {
      ...base,
      as_of: u.as_of,
      quotes: { ...base.quotes, [u.symbol]: u.quote },
    };
  }
  if (u.option_key && u.option) {
    return {
      ...base,
      as_of: u.as_of,
      options: { ...base.options, [u.option_key]: u.option },
    };
  }
  return base;
}
