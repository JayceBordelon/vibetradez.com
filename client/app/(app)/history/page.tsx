import type { Metadata } from "next";
import { HistoryShell } from "@/components/history/history-shell";
import { serverFetch } from "@/lib/api";
import type { WeekResponse } from "@/types/trade";

const OG_IMAGE = "/og/history.png";

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
      const { computeTradePnl } = await import("@/lib/calculations");
      let totalPnl = 0;
      let totalTrades = 0;
      let winners = 0;
      let losers = 0;

      for (const day of data.days) {
        // day.trades can be JSON null when the Go server serializes a
        // nil slice (see CLAUDE.md's history-crash note). The page
        // component already guards with `?? []`; do the same here so
        // generateMetadata doesn't silently fall through to the
        // generic fallback description on a malformed day.
        for (const dt of day.trades ?? []) {
          totalTrades++;
          const result = computeTradePnl(dt, day.executions ?? null);
          if (!result.hasData) continue;
          const pnl = result.pnl;
          totalPnl += pnl;
          if (pnl > 0.5) winners++;
          else if (pnl < -0.5) losers++;
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
