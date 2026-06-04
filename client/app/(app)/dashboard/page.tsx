import type { Metadata } from "next";

import { PortfolioShell } from "@/components/portfolio/portfolio-shell";

const OG_IMAGE = "/og/dashboard.png";

const DESCRIPTION = "The live book: holdings, cash, the equity curve against SPY, and every move the model makes with its reasoning. One account, managed by Claude.";

export const metadata: Metadata = {
  title: "Live Dashboard",
  description: DESCRIPTION,
  openGraph: {
    title: "VibeTradez | Live Portfolio Dashboard",
    description: DESCRIPTION,
    images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | Live Portfolio Dashboard",
    description: DESCRIPTION,
    images: [OG_IMAGE],
  },
};

export default function DashboardPage() {
  return <PortfolioShell />;
}
