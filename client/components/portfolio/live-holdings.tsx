"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { OptionsTable, StocksTable } from "@/components/portfolio/book-tables";
import { AnimatedNumber } from "@/components/ui/animated-number";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { useVisiblePoll } from "@/hooks/use-visible-poll";
import { api } from "@/lib/api";
import { repriceHoldings } from "@/lib/live-pricing";
import type { Holding } from "@/types/portfolio";

/*
LiveHoldings is the /holdings list body, hydrated with the server-rendered
book and kept truly live from there: streamed quotes re-price every mark
tick by tick, and a background poll re-fetches the book itself (so a
position the model opens mid-session appears without a refresh). All the
numbers run through AnimatedNumber, so updates count up or down instead
of flickering.
*/

const REFRESH_SECONDS = 60;

export function LiveHoldings({ initial }: { initial: Holding[] }) {
  const [holdings, setHoldings] = useState<Holding[]>(initial);
  const quotes = useLiveQuotes(initial.length > 0);

  // Sequence guard: useVisiblePoll re-fires load() on interval and on tab
  // re-focus, so commit only the latest poll's result — otherwise a slow
  // earlier fetch can resolve last and revert the book to a stale snapshot.
  const loadSeq = useRef(0);
  const load = useCallback(() => {
    const seq = ++loadSeq.current;
    api
      .getHoldings()
      .then((r) => {
        if (seq === loadSeq.current) setHoldings(r.holdings ?? []);
      })
      .catch(() => {});
  }, []);
  // The server already rendered the current book; start polling for book
  // CHANGES (new/closed positions) after the first interval, not at mount.
  useEffect(() => {
    /* initial render is the RSC payload */
  }, []);
  useVisiblePoll(load, REFRESH_SECONDS * 1000, true);

  const live = useMemo(() => repriceHoldings(holdings, quotes), [holdings, quotes]);
  const options = live.filter((h) => h.kind === "option");
  const equities = live.filter((h) => h.kind === "stock");
  const totalMV = live.reduce((s, h) => s + h.market_value, 0);
  const totalPnl = live.reduce((s, h) => s + h.unrealized_pnl, 0);

  if (live.length === 0) {
    return <div className="rounded-2xl border border-border bg-card px-6 py-16 text-center text-sm text-muted-foreground shadow-sm">No open positions. The account is in cash.</div>;
  }

  return (
    <div className="space-y-6">
      <StatStrip cols={3}>
        <Stat label="Market value" value={<AnimatedNumber value={totalMV} kind="money" crumb />} />
        <Stat label="Unrealized P&L" value={<AnimatedNumber value={totalPnl} kind="pnlInt" crumb />} tone={totalPnl > 0 ? "positive" : totalPnl < 0 ? "negative" : "neutral"} />
        <Stat label="Positions" value={`${live.length}`} />
      </StatStrip>
      <div>
        <OptionsTable items={options} />
        <StocksTable items={equities} />
      </div>
    </div>
  );
}
