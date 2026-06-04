import type { Metadata } from "next";

import { TranscriptView } from "@/components/transcript/transcript-view";

const OG_IMAGE = "/og/dashboard.png";

const MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

// "2026-06-04" -> "June 4, 2026"; returns the input unchanged if it isn't a date.
function prettyDate(d: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(d.trim());
  if (!m) return d;
  const month = MONTHS[Number(m[2]) - 1];
  return month ? `${month} ${Number(m[3])}, ${m[1]}` : d;
}

interface PageProps {
  params: Promise<{ date: string }>;
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { date } = await params;
  const human = prettyDate(date);
  const title = `Session reasoning for ${human}`;
  const description = `Everything Claude looked at and every move it made with the account on ${human}, tool call by tool call.`;
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

/*
The day's session transcript at /transcripts/<date>. There is one transcript
kind for the portfolio manager, so this short URL just renders it; the older
/transcript/<date>/<kind> route still works for any archived v1 kinds.
*/
export default async function TranscriptsByDatePage({ params }: PageProps) {
  const { date } = await params;
  return <TranscriptView date={date} kind="portfolio" />;
}
