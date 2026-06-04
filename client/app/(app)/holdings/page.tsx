import type { Metadata } from "next";

import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { HoldingRow } from "@/components/portfolio/trade-cards";
import { HoldingDetail } from "@/components/portfolio/trade-detail";
import { fmtMoney, fmtPnlInt } from "@/lib/format";
import { fetchHoldings, findHolding } from "@/lib/portfolio-data";
import type { Holding } from "@/types/portfolio";

const OG_IMAGE = "/og/dashboard.png";

// Always render per-request: the book + trade log are live, not build-time data.
export const dynamic = "force-dynamic";

interface PageProps {
  searchParams: Promise<{ trade?: string }>;
}

export async function generateMetadata({ searchParams }: PageProps): Promise<Metadata> {
  const trade = (await searchParams).trade;
  if (trade) {
    const h = await findHolding(decodeURIComponent(trade));
    if (h) {
      const pct = h.cost_basis > 0 ? (h.unrealized_pnl / h.cost_basis) * 100 : 0;
      const title = `${h.label} · ${fmtPnlInt(h.unrealized_pnl)}`;
      const description = `Open ${h.kind} position: ${h.label}, worth ${fmtMoney(h.market_value)} with ${fmtPnlInt(h.unrealized_pnl)} (${pct >= 0 ? "+" : ""}${pct.toFixed(1)}%) unrealized.`;
      return {
        title,
        description,
        openGraph: { title: `${h.label} | VibeTradez`, description, images: [{ url: OG_IMAGE, width: 1200, height: 630 }] },
        twitter: { card: "summary_large_image", title: `${h.label} | VibeTradez`, description, images: [OG_IMAGE] },
      };
    }
  }
  const holdings = await fetchHoldings();
  const n = holdings.length;
  const title = `Holdings (${n})`;
  const description = n > 0 ? `The ${n} position${n === 1 ? "" : "s"} the model is holding right now, stocks and options, with cost basis and unrealized P&L.` : "The model is fully in cash right now. No open positions.";
  return {
    title,
    description,
    openGraph: { title: `${title} | VibeTradez`, description, images: [{ url: OG_IMAGE, width: 1200, height: 630 }] },
    twitter: { card: "summary_large_image", title: `${title} | VibeTradez`, description, images: [OG_IMAGE] },
  };
}

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

export default async function HoldingsPage({ searchParams }: PageProps) {
  const trade = (await searchParams).trade;

  // ?trade=<uuid> opens the detail view in place. A stale or unknown id just
  // falls back to the full list rather than erroring.
  if (trade) {
    const h = await findHolding(decodeURIComponent(trade));
    if (h) return <HoldingDetail h={h} />;
  }

  const holdings = await fetchHoldings();
  const options = holdings.filter((h) => h.kind === "option");
  const equities = holdings.filter((h) => h.kind === "stock");
  const totalMV = holdings.reduce((s, h) => s + h.market_value, 0);
  const totalPnl = holdings.reduce((s, h) => s + h.unrealized_pnl, 0);

  return (
    <div className="mx-auto min-w-0 max-w-[1200px] px-4 py-6 sm:px-7">
      {holdings.length === 0 ? (
        <div className="border-y border-border/50 px-1 py-12 text-center text-sm text-muted-foreground">No open positions. The account is in cash.</div>
      ) : (
        <>
          <StatStrip cols={3}>
            <Stat label="Market value" value={fmtMoney(totalMV)} />
            <Stat label="Unrealized P&L" value={fmtPnlInt(totalPnl)} tone={totalPnl > 0 ? "positive" : totalPnl < 0 ? "negative" : "neutral"} />
            <Stat label="Positions" value={`${holdings.length}`} />
          </StatStrip>
          <Group title="Options" items={options} />
          <Group title="Stocks" items={equities} />
        </>
      )}
    </div>
  );
}
