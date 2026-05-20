"use client";

import { ArrowRight, Clock, LineChart } from "lucide-react";
import Link from "next/link";

import { Button } from "@/components/ui/button";
import type { MarketStatus } from "@/types/trade";

/*
MarketClosedPage is the full-page render when the dashboard loads
outside the 9:30-16:00 ET (or 13:00 ET on half-days) window of a
trading day. No live-data fetches happen on this page — the only
data dependency is the next-open timestamp from /api/market/status,
which is a calendar lookup with no Schwab calls.

The single CTA points at /history, which is DB-only and the natural
home for "what did the model do recently?" while the live dashboard
is dark.
*/
export function MarketClosedPage({ status }: { status: MarketStatus | null }) {
  const headline = headlineFor(status);
  const reopenLine = reopenLineFor(status);

  return (
    <div className="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center px-4 py-12 text-center sm:px-6">
      <div className="mb-5 inline-flex h-12 w-12 items-center justify-center rounded-full bg-muted/60 ring-1 ring-border/60">
        <Clock className="h-5 w-5 text-muted-foreground" aria-hidden />
      </div>

      <h1 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">{headline}</h1>
      {reopenLine && <p className="mt-2 text-sm text-muted-foreground">{reopenLine}</p>}

      <p className="mt-6 max-w-sm text-[13px] text-muted-foreground/85">
        The live dashboard only streams during regular trading hours. Past picks, their EOD outcomes, and aggregate stats are always available in the history archive.
      </p>

      <Button asChild size="lg" className="mt-8 gap-2">
        <Link href="/history" aria-label="View pick history">
          <LineChart className="h-4 w-4" aria-hidden />
          View pick history
          <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" aria-hidden />
        </Link>
      </Button>
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
      // Defensive: this component shouldn't render in this state, but
      // if it does (race between status fetch and UI), surface a label
      // that won't read as a bug.
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
  return `Live dashboard reopens ${fmt.format(next)}`;
}
