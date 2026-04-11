import type { Metadata } from "next";
import { DashboardShell } from "@/components/dashboard/dashboard-shell";
import { serverFetch } from "@/lib/api";
import type { DashboardResponse } from "@/types/trade";

const OG_IMAGE = "/og";

export async function generateMetadata(): Promise<Metadata> {
  try {
    const data = await serverFetch<DashboardResponse>("/api/trades/today");
    const count = data.trades?.length ?? 0;
    const hasSummaries = data.trades?.some((t) => t.summary) ?? false;

    let description = "Live options trade dashboard with dual-model daily picks (OpenAI + Claude) and real-time analytics.";

    if (count > 0 && hasSummaries) {
      let totalPnl = 0;
      let winners = 0;
      let losers = 0;
      for (const { summary } of data.trades) {
        if (summary) {
          const pnl = (summary.closing_price - summary.entry_price) * 100;
          totalPnl += pnl;
          if (pnl > 0.5) winners++;
          else if (pnl < -0.5) losers++;
        }
      }
      const sign = totalPnl > 0 ? "+" : "";
      description = `Today: ${count} union picks, ${winners}W/${losers}L, ${sign}$${Math.round(totalPnl)} P&L. Dual-model options dashboard with real-time charts.`;
    } else if (count > 0) {
      const topSymbols = data.trades
        .slice(0, 3)
        .map((t) => t.trade.symbol)
        .join(", ");
      description = `Today's ${count} union picks: ${topSymbols} and more. OpenAI + Claude dual-model dashboard.`;
    }

    return {
      title: "Live Dashboard",
      description,
      openGraph: {
        title: "VibeTradez | Live Options Dashboard",
        description,
        images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
      },
      twitter: {
        card: "summary_large_image",
        title: "VibeTradez | Live Options Dashboard",
        description,
        images: [OG_IMAGE],
      },
    };
  } catch {
    return {
      title: "Live Dashboard",
      description: "Live options trade dashboard with dual-model daily picks (OpenAI + Claude) and real-time analytics.",
      openGraph: {
        title: "VibeTradez | Live Options Dashboard",
        images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
      },
    };
  }
}

export default function DashboardPage() {
  return <DashboardShell />;
}
