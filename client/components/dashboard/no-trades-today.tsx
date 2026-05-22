import { ArrowRight, LineChart, Sparkles } from "lucide-react";
import Link from "next/link";

export function NoTradesToday() {
  return (
    <div className="flex flex-col items-center justify-center px-4 py-12 text-center sm:py-24">
      <div className="lg-card relative flex w-full max-w-md flex-col items-center gap-5 px-6 py-10 sm:max-w-lg sm:px-12 sm:py-12">
        <Sparkles className="absolute top-4 left-5 h-3 w-3 animate-pulse text-[var(--claude)] [animation-delay:0.3s] [animation-duration:2.4s] sm:left-6 sm:h-3.5 sm:w-3.5" aria-hidden />
        <Sparkles className="absolute top-7 right-6 h-2.5 w-2.5 animate-pulse text-[var(--claude)] [animation-delay:1.1s] [animation-duration:3s] sm:top-8 sm:right-7" aria-hidden />
        <Sparkles className="absolute bottom-10 left-8 h-2 w-2 animate-pulse text-[var(--claude)] [animation-delay:0.7s] [animation-duration:2.8s] sm:bottom-12 sm:left-10" aria-hidden />

        <div className="relative">
          <div className="absolute -inset-6 rounded-full bg-[var(--claude-light)] blur-2xl" aria-hidden />
          <div className="relative flex h-20 w-20 items-center justify-center rounded-full border border-[var(--claude-border)] bg-[var(--claude-light)] sm:h-24 sm:w-24">
            <LineChart className="h-9 w-9 text-[var(--claude)] sm:h-11 sm:w-11" strokeWidth={1.5} aria-hidden />
          </div>
        </div>

        <div className="flex flex-col items-center gap-1.5">
          <h3 className="text-lg font-semibold tracking-tight sm:text-xl">No trades today</h3>
          <p className="max-w-xs text-sm leading-relaxed text-muted-foreground sm:max-w-sm">Nothing on the board for today, head over to the historical breakdown to see how past picks landed.</p>
        </div>

        <Link
          href="/history"
          className="group mt-1 inline-flex items-center gap-1.5 rounded-full border border-[var(--claude-border)] bg-[var(--claude-light)] px-4 py-2 text-xs font-semibold text-[var(--claude)] transition-colors hover:bg-[var(--claude-light)]/70 sm:text-sm"
        >
          View past performance
          <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden />
        </Link>
      </div>
    </div>
  );
}
