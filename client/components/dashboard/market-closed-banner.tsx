"use client";

import { ArrowRight, Clock } from "lucide-react";
import Link from "next/link";

import type { MarketStatus } from "@/types/trade";

/*
MarketClosedBanner is the slim notice shown atop the dashboard when it
loads outside trading hours. Rather than blanking the page, the
dashboard now renders the most recent trading day's picks + EOD
outcomes (resolved server-side via GetLatestTradeDate) and this banner
just clarifies that the data is the last session, not live, plus when
the live stream resumes.
*/
export function MarketClosedBanner({ status, date }: { status: MarketStatus | null; date?: string }) {
  const reopenLine = reopenLineFor(status);

  return (
    <div className="mb-6 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-xl border border-border/60 bg-muted/40 px-4 py-3 text-sm">
      <Clock className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden />
      <span className="font-medium text-foreground">{headlineFor(status)}</span>
      <span className="text-muted-foreground">Showing the last trading day{date ? ` (${fmtDate(date)})` : ""}, not live.</span>
      {reopenLine && <span className="hidden text-muted-foreground/80 sm:inline">· {reopenLine}</span>}
      <Link href="/history" className="ml-auto inline-flex items-center gap-1 font-medium text-muted-foreground transition-colors hover:text-foreground">
        Full history
        <ArrowRight className="h-3.5 w-3.5" aria-hidden />
      </Link>
    </div>
  );
}

function headlineFor(status: MarketStatus | null): string {
  if (!status) return "Markets closed";
  switch (status.session) {
    case "premarket":
      return "Pre-market";
    case "afterhours":
      return "Markets closed for the day";
    case "closed":
      return "Markets closed";
    case "open":
      return "Markets open";
  }
}

function reopenLineFor(status: MarketStatus | null): string | null {
  if (!status?.next_open) return null;
  const next = new Date(status.next_open);
  if (Number.isNaN(next.getTime())) return null;

  const fmt = new Intl.DateTimeFormat("en-US", {
    weekday: "long",
    hour: "numeric",
    minute: "2-digit",
    timeZone: "America/New_York",
    timeZoneName: "short",
  });
  return `live again ${fmt.format(next)}`;
}

/*
date is a bare YYYY-MM-DD. Parse it as UTC midnight and format in UTC so
it never shifts a day backward in a western timezone.
*/
function fmtDate(date: string): string {
  const d = new Date(`${date}T00:00:00Z`);
  if (Number.isNaN(d.getTime())) return date;
  return new Intl.DateTimeFormat("en-US", { weekday: "short", month: "short", day: "numeric", timeZone: "UTC" }).format(d);
}
