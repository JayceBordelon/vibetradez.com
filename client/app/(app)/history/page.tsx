import type { Metadata } from "next";
import { HistoryShell } from "@/components/history/history-shell";
import { serverFetch } from "@/lib/api";
import type { WeekResponse } from "@/types/trade";

const OG_IMAGE = "/opengraph-image";

function currentWeekRange(): { start: string; end: string } {
  const now = new Date();
  const day = now.getDay();
  const diffToMon = day === 0 ? -6 : 1 - day;
  const monday = new Date(now);
  monday.setDate(now.getDate() + diffToMon);
  const friday = new Date(monday);
  friday.setDate(monday.getDate() + 4);
  const fmt = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  return { start: fmt(monday), end: fmt(friday) };
}

export async function generateMetadata(): Promise<Metadata> {
  try {
    const { start, end } = currentWeekRange();
    const data = await serverFetch<WeekResponse>(`/api/trades/week?start=${start}&end=${end}`);

    let description = "Historical options trading performance with equity curves, exposure analysis, and risk metrics.";

    if (data.days?.length) {
      let totalPnl = 0;
      let totalTrades = 0;
      let winners = 0;
      let losers = 0;

      for (const day of data.days) {
        for (const { summary } of day.trades) {
          totalTrades++;
          if (summary) {
            const pnl = (summary.closing_price - summary.entry_price) * 100;
            totalPnl += pnl;
            if (pnl > 0.5) winners++;
            else if (pnl < -0.5) losers++;
          }
        }
      }

      if (winners + losers > 0) {
        const winRate = Math.round((winners / (winners + losers)) * 100);
        const sign = totalPnl > 0 ? "+" : "";
        description = `This week: ${totalTrades} trades, ${winRate}% win rate, ${sign}$${Math.round(totalPnl)} P&L. Track performance over time.`;
      }
    }

    return {
      title: "Historical Performance",
      description,
      openGraph: {
        title: "VibeTradez | Historical Performance",
        description,
        images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
      },
      twitter: {
        card: "summary_large_image",
        title: "VibeTradez | Historical Performance",
        description,
        images: [OG_IMAGE],
      },
    };
  } catch {
    return {
      title: "Historical Performance",
      description: "Historical options trading performance with equity curves, exposure analysis, and risk metrics.",
      openGraph: {
        title: "VibeTradez | Historical Performance",
        images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
      },
    };
  }
}

export default function HistoryPage() {
  return <HistoryShell />;
}
