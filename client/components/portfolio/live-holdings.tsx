"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { HoldingRow } from "@/components/portfolio/trade-cards";
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

function Group({ title, items }: { title: string; items: Holding[] }) {
  if (items.length === 0) return null;
  return (
    <section className="mt-8 first:mt-4">
      <h2 className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
        {title} ({items.length})
      </h2>
      <div className="mt-1 divide-y divide-border/50 border-t border-border/50">
        {items.map((h) => (
          <HoldingRow key={h.id} h={h} />
        ))}
      </div>
    </section>
  );
}

export function LiveHoldings({ initial }: { initial: Holding[] }) {
  const [holdings, setHoldings] = useState<Holding[]>(initial);
  const quotes = useLiveQuotes(initial.length > 0);

  const load = useCallback(() => {
    api
      .getHoldings()
      .then((r) => setHoldings(r.holdings ?? []))
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
    return <div className="border-y border-border/50 px-1 py-12 text-center text-sm text-muted-foreground">No open positions. The account is in cash.</div>;
  }

  return (
    <>
      <StatStrip cols={3}>
        <Stat label="Market value" value={<AnimatedNumber value={totalMV} kind="money" />} />
        <Stat label="Unrealized P&L" value={<AnimatedNumber value={totalPnl} kind="pnlInt" />} tone={totalPnl > 0 ? "positive" : totalPnl < 0 ? "negative" : "neutral"} />
        <Stat label="Positions" value={`${live.length}`} />
      </StatStrip>
      <Group title="Options" items={options} />
      <Group title="Stocks" items={equities} />
    </>
  );
}
