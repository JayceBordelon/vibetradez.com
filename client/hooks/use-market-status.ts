"use client";

import { useCallback, useEffect, useState } from "react";

import { api } from "@/lib/api";
import type { MarketStatus } from "@/types/trade";

/*
useMarketStatus polls GET /api/market/status on mount and on tab
focus, returning the current state so the dashboard can decide
between rendering the live streaming UI or the market-closed page.

The endpoint is cheap (calendar lookup, no Schwab calls) and the
status only flips on cron-tick boundaries, so polling on focus
instead of on a timer is enough — a user away from the tab during
9:30 ET will see the flip the moment they return.

Returns null until the first fetch lands; consumers should render a
loading placeholder during this brief window.
*/
export function useMarketStatus(): MarketStatus | null {
  const [status, setStatus] = useState<MarketStatus | null>(null);

  const refresh = useCallback(() => {
    api
      .getMarketStatus()
      .then(setStatus)
      .catch(() => {
        // Swallow transient failures. The next focus / mount retries.
        // Prior state stays rendered.
      });
  }, []);

  useEffect(() => {
    refresh();
    const onVisible = () => {
      if (document.visibilityState === "visible") refresh();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, [refresh]);

  return status;
}
