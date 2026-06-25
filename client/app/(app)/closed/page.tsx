import type { Metadata } from "next";

import { Pagination } from "@/components/layout/pagination";
import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { ClosedTable } from "@/components/portfolio/book-tables";
import { ClosedDetail } from "@/components/portfolio/trade-detail";
import { fmtPnlInt } from "@/lib/format";
import { fetchClosedTrades, findClosedTrade } from "@/lib/portfolio-data";

const OG_IMAGE = "/og/dashboard.png";
// 12 rows fills a 900px desktop viewport without the dead band 5 left, and
// keeps the pager from fragmenting the history into ten pages.
const PAGE_SIZE = 12;

// Always render per-request: the book + trade log are live, not build-time data.
export const dynamic = "force-dynamic";

interface PageProps {
  searchParams: Promise<{ page?: string; trade?: string }>;
}

export async function generateMetadata({ searchParams }: PageProps): Promise<Metadata> {
  const trade = (await searchParams).trade;
  if (trade) {
    const t = await findClosedTrade(decodeURIComponent(trade));
    if (t) {
      const verb = t.realized_pnl >= 0 ? "made" : "lost";
      const title = `${t.label} · ${fmtPnlInt(t.realized_pnl)}`;
      const description = `Closed ${t.kind} trade: ${t.label} ${verb} ${fmtPnlInt(t.realized_pnl)} (${t.realized_pct >= 0 ? "+" : ""}${t.realized_pct.toFixed(1)}%) over ${t.hold_days} days, ${t.opened_date} to ${t.closed_date}.`;
      return {
        title,
        description,
        openGraph: { title: `${t.label} | VibeTradez`, description, images: [{ url: OG_IMAGE, width: 1200, height: 630 }] },
        twitter: { card: "summary_large_image", title: `${t.label} | VibeTradez`, description, images: [OG_IMAGE] },
      };
    }
  }
  const trades = await fetchClosedTrades();
  const n = trades.length;
  const realized = trades.reduce((s, t) => s + t.realized_pnl, 0);
  const title = `Closed trades (${n})`;
  const description = n > 0 ? `${n} completed round trip${n === 1 ? "" : "s"} the model has closed, for ${realized >= 0 ? "+" : ""}$${Math.round(realized).toLocaleString()} realized.` : "No closed trades yet.";
  return {
    title,
    description,
    openGraph: { title: `${title} | VibeTradez`, description, images: [{ url: OG_IMAGE, width: 1200, height: 630 }] },
    twitter: { card: "summary_large_image", title: `${title} | VibeTradez`, description, images: [OG_IMAGE] },
  };
}

export default async function ClosedTradesPage({ searchParams }: PageProps) {
  const sp = await searchParams;

  // ?trade=<uuid> opens the detail view in place. A stale or unknown id just
  // falls back to the paginated list rather than erroring.
  if (sp.trade) {
    const t = await findClosedTrade(decodeURIComponent(sp.trade));
    if (t) return <ClosedDetail t={t} />;
  }

  const trades = await fetchClosedTrades();
  const realized = trades.reduce((s, t) => s + t.realized_pnl, 0);
  // A win is strictly profitable: a scratch (exactly $0) paid the spread for
  // nothing and counts as a loss, matching the server's track-record stats
  // (trades.Summarize) and the dashboard's win-rate strip.
  const wins = trades.filter((t) => t.realized_pnl > 0).length;
  const winRate = trades.length > 0 ? (wins / trades.length) * 100 : 0;

  // Page through the round trips. The headline stats stay over the full set;
  // only the list is sliced. Clamp the requested page into range so a bad
  // ?page= can't render an empty list.
  const totalPages = Math.max(1, Math.ceil(trades.length / PAGE_SIZE));
  const requested = Number.parseInt(sp.page ?? "1", 10);
  const page = Math.min(Math.max(Number.isNaN(requested) ? 1 : requested, 1), totalPages);
  const pageTrades = trades.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  return (
    <div className="animate-soft-rise mx-auto min-w-0 max-w-[1200px] px-4 py-8 sm:px-7">
      <header className="mb-6">
        <h1 className="font-display text-2xl font-semibold tracking-tight sm:text-[28px]">Closed trades</h1>
        <p className="mt-1 text-sm text-muted-foreground">Every completed round trip the model has closed, with realized P&L.</p>
      </header>
      {trades.length === 0 ? (
        <div className="rounded-2xl border border-border bg-card px-6 py-16 text-center text-sm text-muted-foreground shadow-sm">No closed trades yet. Round trips show up here once the model sells out of a position.</div>
      ) : (
        <>
          <StatStrip cols={3}>
            <Stat label="Realized P&L" value={fmtPnlInt(realized)} tone={realized > 0 ? "positive" : realized < 0 ? "negative" : "neutral"} />
            <Stat label="Win rate" value={`${winRate.toFixed(0)}%`} />
            <Stat label="Trades" value={`${trades.length}`} />
          </StatStrip>
          <h2 className="mt-7 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">Round trips ({trades.length})</h2>
          <ClosedTable items={pageTrades} />
          <Pagination basePath="/closed" page={page} pageSize={PAGE_SIZE} totalItems={trades.length} />
        </>
      )}
    </div>
  );
}
