import { cn } from "@/lib/utils";

/*
The VibeTradez wordmark: a small soft sage mark holding an upward
sparkline, paired with the name set in the display face. One calm accent,
no glow, no terminal chrome — the brand atom for the nav, footer, and
landing. `size` scales the whole lockup; `markOnly` drops the text.
*/
export function BrandMark({ className }: { className?: string }) {
  return (
    <span
      aria-hidden
      className={cn("inline-flex items-center justify-center rounded-[0.55rem] bg-gradient-brand text-white shadow-sm", className)}
    >
      <svg viewBox="0 0 24 24" fill="none" className="h-[58%] w-[58%]" strokeLinecap="round" strokeLinejoin="round">
        <path d="M4 15.5 L9.5 10 L13 13 L20 5.5" stroke="currentColor" strokeWidth={2.4} />
        <path d="M20 5.5 L20 10.5 M20 5.5 L15 5.5" stroke="currentColor" strokeWidth={2.4} />
      </svg>
    </span>
  );
}

export function Wordmark({ className, size = "md" }: { className?: string; size?: "sm" | "md" | "lg" }) {
  const mark = size === "lg" ? "h-8 w-8" : size === "sm" ? "h-6 w-6" : "h-7 w-7";
  const text = size === "lg" ? "text-[22px]" : size === "sm" ? "text-[15px]" : "text-[17px]";
  return (
    <span className={cn("inline-flex items-center gap-2", className)}>
      <BrandMark className={mark} />
      <span className={cn("font-display font-bold tracking-tight text-foreground", text)}>
        Vibe<span className="text-primary">Tradez</span>
      </span>
    </span>
  );
}
