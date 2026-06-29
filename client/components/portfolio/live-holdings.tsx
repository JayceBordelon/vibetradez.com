"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Pagination } from "@/components/layout/pagination";
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
// Page the book once it gets long. Matches the /closed page size so both book
// surfaces page at the same cadence; the pager hides itself at or below it.
const PAGE_SIZE = 10;

export function LiveHoldings({ initial }: { initial: Holding[] }) {
  const [holdings, setHoldings] = useState<Holding[]>(initial);
  const [page, setPage] = useState(1);
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
    return <div className="px-2 py-20 text-center text-sm text-muted-foreground">No open positions. The account is in cash.</div>;
  }

  // Page the combined book (options first, then any legacy stock). The headline
  // stats stay over the FULL book; only the tables are sliced. A live poll can
  // grow or shrink the book and leave `page` out of range, so clamp on read —
  // the stored page never has to be corrected, and the pager auto-hides at or
  // below PAGE_SIZE.
  const ordered = [...options, ...equities];
  const totalPages = Math.max(1, Math.ceil(ordered.length / PAGE_SIZE));
  const safePage = Math.min(Math.max(page, 1), totalPages);
  const slice = ordered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);
  const pageOptions = slice.filter((h) => h.kind === "option");
  const pageStocks = slice.filter((h) => h.kind === "stock");

  return (
    <div>
      <StatStrip cols={3}>
        <Stat label="Market value" value={<AnimatedNumber value={totalMV} kind="money" crumb />} />
        <Stat label="Unrealized P&L" value={<AnimatedNumber value={totalPnl} kind="pnlInt" crumb />} tone={totalPnl > 0 ? "positive" : totalPnl < 0 ? "negative" : "neutral"} />
        <Stat label="Positions" value={`${live.length}`} />
      </StatStrip>
      <OptionsTable items={pageOptions} total={options.length} />
      <StocksTable items={pageStocks} total={equities.length} />
      <Pagination page={safePage} pageSize={PAGE_SIZE} totalItems={ordered.length} onPageChange={setPage} />
    </div>
  );
}
