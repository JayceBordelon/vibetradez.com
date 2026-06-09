import type { Metadata } from "next";

import { LiveHoldings } from "@/components/portfolio/live-holdings";
import { HoldingDetail } from "@/components/portfolio/trade-detail";
import { fmtMoney, fmtPnlInt } from "@/lib/format";
import { fetchHoldings, findHolding } from "@/lib/portfolio-data";

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

export default async function HoldingsPage({ searchParams }: PageProps) {
  const trade = (await searchParams).trade;

  // ?trade=<uuid> opens the detail view in place. A stale or unknown id just
  // falls back to the full list rather than erroring.
  if (trade) {
    const h = await findHolding(decodeURIComponent(trade));
    if (h) return <HoldingDetail h={h} />;
  }

  // Server-render the current book for the first paint; LiveHoldings then
  // keeps it truly live (streamed quote re-pricing + background repoll).
  const holdings = await fetchHoldings();

  return (
    <div className="mx-auto min-w-0 max-w-[1200px] px-4 py-6 sm:px-7">
      <LiveHoldings initial={holdings} />
    </div>
  );
}
