import { redirect } from "next/navigation";

/*
/transcripts is the Sessions tab's landing spot: it forwards to today's
session (market time), where the date pager takes over for browsing
backward. Computed per-request so the redirect never serves a stale day.
*/
export const dynamic = "force-dynamic";

export default function TranscriptsIndexPage() {
  const today = new Intl.DateTimeFormat("en-CA", { timeZone: "America/New_York" }).format(new Date());
  redirect(`/transcripts/${today}`);
}
