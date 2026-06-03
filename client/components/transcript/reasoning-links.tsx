import { ArrowRight, ScrollText } from "lucide-react";
import Link from "next/link";

import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/*
ReasoningLinks is the single, shared entry point to the daily model
transcripts (/transcript/<date>/<kind>). Rendered everywhere a user
might want "how did Claudia decide this?" — the dashboard, the
trade-detail page, and each history day row — so the affordance looks
and behaves identically across the app.

Two variants:
  - "callout" — a prominent Claude-tinted card with a heading + both
    links as buttons. Used where there's room to invite the click
    (dashboard).
  - "inline"  — a compact labelled row of two buttons. Used in dense
    contexts (trade detail, history day rows).

Both link to the same date; the transcript page renders a clean empty
state for days without a captured run, so it's always safe to show.
*/
export function ReasoningLinks({
  date,
  variant = "inline",
  className,
}: {
  date: string;
  variant?: "callout" | "inline";
  className?: string;
}) {
  const selection = `/transcript/${date}/selection`;
  const execution = `/transcript/${date}/execution`;

  if (variant === "callout") {
    return (
      <div
        className={cn(
          "rounded-xl border border-claude-border/50 bg-claude-light px-5 py-4 sm:px-6 sm:py-5",
          className
        )}
      >
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-start gap-3">
            <ClaudeLogo className="mt-0.5 h-5 w-5 shrink-0" />
            <div>
              <h3 className="text-sm font-semibold text-foreground">See how Claudia decided</h3>
              <p className="mt-0.5 text-sm text-muted-foreground">
                Read the model's full reasoning, tool calls, and decisions for this day — the pre-bell ticker
                selection and the at-open order placement.
              </p>
            </div>
          </div>
          <div className="flex shrink-0 flex-wrap gap-2 sm:flex-col sm:items-stretch">
            <Button asChild variant="outline" size="sm">
              <Link href={selection}>
                Selection reasoning
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            </Button>
            <Button asChild variant="outline" size="sm">
              <Link href={execution}>
                Execution reasoning
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={cn("flex flex-wrap items-center gap-2 text-sm", className)}>
      <span className="flex items-center gap-1.5 text-muted-foreground">
        <ScrollText className="h-4 w-4" />
        Model reasoning:
      </span>
      <Button asChild variant="outline" size="sm">
        <Link href={selection}>Selection</Link>
      </Button>
      <Button asChild variant="outline" size="sm">
        <Link href={execution}>Execution</Link>
      </Button>
    </div>
  );
}
