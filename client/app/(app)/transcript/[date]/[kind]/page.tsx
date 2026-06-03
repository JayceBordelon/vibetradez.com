import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { TranscriptView } from "@/components/transcript/transcript-view";

const OG_IMAGE = "/og/dashboard.png";

type Kind = "selection" | "execution";

function isKind(v: string): v is Kind {
  return v === "selection" || v === "execution";
}

const KIND_LABEL: Record<Kind, string> = {
  selection: "Selection reasoning",
  execution: "Execution reasoning",
};

interface PageProps {
  params: Promise<{ date: string; kind: string }>;
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { date, kind } = await params;
  if (!isKind(kind)) {
    return { title: "Transcript not found" };
  }
  const label = KIND_LABEL[kind];
  const title = `${label} · ${date}`;
  const description =
    kind === "selection"
      ? `How Claudia picked the tickers on ${date}: the 9:25 ET selection conversation, tool calls, and reasoning.`
      : `How Claudia chose contracts and placed orders on ${date}: the 9:30 ET at-open agent conversation, tool calls, and reasoning.`;
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

export default async function TranscriptPage({ params }: PageProps) {
  const { date, kind } = await params;
  if (!isKind(kind)) {
    notFound();
  }
  return <TranscriptView date={date} kind={kind} />;
}
