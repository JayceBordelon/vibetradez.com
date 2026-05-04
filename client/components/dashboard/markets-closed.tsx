import { ArrowRight, Moon, Sparkles } from "lucide-react";
import Link from "next/link";

interface MarketsClosedProps {
  reason: string;
  nextOpen?: string;
}

export function MarketsClosed({ reason, nextOpen }: MarketsClosedProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center sm:py-24">
      <div className="lg-card relative flex flex-col items-center gap-5 px-8 py-10 sm:px-12 sm:py-12">
        <Sparkles className="absolute top-4 left-6 h-3.5 w-3.5 animate-pulse text-[var(--claude)] [animation-delay:0.3s] [animation-duration:2.4s]" aria-hidden />
        <Sparkles className="absolute top-8 right-7 h-2.5 w-2.5 animate-pulse text-[var(--claude)] [animation-delay:1.1s] [animation-duration:3s]" aria-hidden />
        <Sparkles className="absolute bottom-12 left-10 h-2 w-2 animate-pulse text-[var(--claude)] [animation-delay:0.7s] [animation-duration:2.8s]" aria-hidden />

        <div className="relative">
          <div className="absolute -inset-6 rounded-full bg-[var(--claude-light)] blur-2xl" aria-hidden />
          <div className="relative flex h-24 w-24 items-center justify-center rounded-full border border-[var(--claude-border)] bg-[var(--claude-light)]">
            <Moon className="h-11 w-11 text-[var(--claude)]" strokeWidth={1.5} aria-hidden />
          </div>
          <span
            className="absolute -top-1 -right-2 flex animate-bounce flex-col items-center font-mono text-xs font-bold text-[var(--claude)] [animation-delay:0s] [animation-duration:2s]"
            aria-hidden
          >
            Z
          </span>
          <span
            className="absolute -top-3 right-2 flex animate-bounce flex-col items-center font-mono text-sm font-bold text-[var(--claude)] [animation-delay:0.4s] [animation-duration:2s]"
            aria-hidden
          >
            Z
          </span>
          <span
            className="absolute -top-6 right-6 flex animate-bounce flex-col items-center font-mono text-base font-bold text-[var(--claude)] [animation-delay:0.8s] [animation-duration:2s]"
            aria-hidden
          >
            Z
          </span>
        </div>

        <div className="flex flex-col items-center gap-1.5">
          <h3 className="text-xl font-semibold tracking-tight">Markets closed today</h3>
          <p className="max-w-sm text-sm leading-relaxed text-muted-foreground">{reason}</p>
          {nextOpen ? <p className="mt-1 font-mono text-xs text-muted-foreground/80">Next pick run · {nextOpen}</p> : null}
          <p className="mt-2 text-xs italic text-muted-foreground/70">Claude is taking a nap. Check back when the bell rings.</p>
        </div>

        <Link
          href="/history"
          className="group mt-1 inline-flex items-center gap-1.5 rounded-full border border-[var(--claude-border)] bg-[var(--claude-light)] px-4 py-2 text-xs font-semibold text-[var(--claude)] transition-colors hover:bg-[var(--claude-light)]/70 sm:text-sm"
        >
          See past performance
          <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden />
        </Link>
      </div>
    </div>
  );
}
