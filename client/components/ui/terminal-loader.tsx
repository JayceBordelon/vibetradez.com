import { cn } from "@/lib/utils";

/*
TerminalLoader is the app-wide loading placeholder. Instead of grey skeleton
blobs, a page that is still fetching shows a small terminal "boot" panel in the
CRT phosphor theme the rest of the app already uses: a `$` prompt, a blinking
block cursor, and a few status lines that pulse like a session coming online.
Loading reads like the product rather than a generic shimmer.

role=status + aria-busy keeps it announced to assistive tech. The pulse
animations fall back to static under prefers-reduced-motion.
*/
export function TerminalLoader({
  command,
  lines,
  className,
  minHeightClass = "min-h-[55vh]",
  compact = false,
}: {
  command: string;
  lines: string[];
  className?: string;
  minHeightClass?: string;
  compact?: boolean;
}) {
  return (
    <div className={cn("flex w-full items-center justify-center", minHeightClass, className)} role="status" aria-busy="true">
      <div
        className={cn(
          "w-full border border-dashed border-border/70 bg-card-elevated/20 font-mono",
          compact ? "max-w-sm px-4 py-3.5 text-[12px]" : "max-w-md px-5 py-5 text-[13px]"
        )}
      >
        <div className="flex items-center gap-2">
          <span className="phosphor font-bold text-green" aria-hidden>
            $
          </span>
          <span className="text-foreground/90">{command}</span>
          <span className="ml-0.5 inline-block h-3.5 w-[7px] animate-pulse bg-green align-middle motion-reduce:animate-none" aria-hidden />
        </div>
        <ul className={cn("space-y-1.5", compact ? "mt-2.5" : "mt-3")}>
          {lines.map((line, i) => (
            <li key={line} className="flex items-center gap-2 text-muted-foreground/80">
              <span
                className="inline-flex h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-green/70 motion-reduce:animate-none"
                style={{ animationDelay: `${i * 180}ms` }}
                aria-hidden
              />
              <span className="min-w-0 truncate">{line}</span>
              <span aria-hidden className="text-muted-foreground/40">
                &hellip;
              </span>
            </li>
          ))}
        </ul>
        <span className="sr-only">Loading</span>
      </div>
    </div>
  );
}
