import type { Metadata } from "next";
import { TradeDetailPage } from "@/components/trade/trade-detail-page";

const OG_IMAGE = "/opengraph-image";

interface PageProps {
  params: Promise<{ symbol: string }>;
  searchParams: Promise<{ date?: string }>;
}

export async function generateMetadata({ params, searchParams }: PageProps): Promise<Metadata> {
  const { symbol } = await params;
  const { date } = await searchParams;
  const symbolUpper = symbol.toUpperCase();
  const title = date ? `$${symbolUpper} on ${date}` : `$${symbolUpper}`;
  const description = `Single-contract view for $${symbolUpper}. Independent picks, dual-model conviction, cross-examination verdicts, and EOD result if settled.`;
  return {
    title,
    description,
    openGraph: {
      title: `${title} | VibeTradez`,
      description,
      images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
    },
    twitter: {
      card: "summary_large_image",
      title: `${title} | VibeTradez`,
      description,
      images: [OG_IMAGE],
    },
  };
}

export default async function TradePage({ params, searchParams }: PageProps) {
  const { symbol } = await params;
  const { date } = await searchParams;
  return <TradeDetailPage symbol={symbol.toUpperCase()} date={date} />;
}
