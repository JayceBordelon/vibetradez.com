import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

import { Reveal } from "./reveal";

/*
Chapter is the narrative-scroll primitive for the landing page. Every
story beat is a Chapter: a numbered eyebrow ("01 / The setup"), a
confident headline, supporting prose on one side and an open visual /
stat on the other. Chapters alternate which side the text sits on (the
`flip` prop), so the eye zig-zags down the page and adjacent beats never
look the same.

There are no hard band edges and no boxed surfaces. The page is one
continuous surface: the ambient mesh flows behind everything and chapters
are separated by generous whitespace and type, not by colored rectangles.
The supporting visuals sit openly on the page (large type, hairline rules
at most), never inside cards that wall them off.

`weight` tunes the column split so the descent does not read as the same
50/50 block repeating: "even" is a balanced two-up, "text" gives the
prose more room with a slimmer figure, "figure" lets the visual breathe
wider. The narrative column always leads on mobile, so reading order
stays correct in a single column.
*/

interface ChapterProps {
  /** Two-digit beat number, e.g. "01". */
  index: string;
  /** Short chapter label, e.g. "The setup". */
  kicker: string;
  /** The large narrative headline. Pass JSX so callers can gradient a span. */
  title: ReactNode;
  /** Supporting prose under the headline. */
  children: ReactNode;
  /** The supporting visual / diagram / stat for the opposite column. */
  aside: ReactNode;
  /** When true, the visual sits on the LEFT and text on the RIGHT (desktop). */
  flip?: boolean;
  /** Column split, so adjacent beats never read as the same 50/50 block. */
  weight?: "even" | "text" | "figure";
  /** Accent color for the index marker and rule. */
  accent?: "brand" | "claude" | "green";
  /** Anchor id so the sticky progress rail can target the chapter. */
  id: string;
}

const COLS: Record<NonNullable<ChapterProps["weight"]>, string> = {
  even: "md:grid-cols-2",
  text: "md:grid-cols-[1.25fr_0.75fr]",
  figure: "md:grid-cols-[0.8fr_1.2fr]",
};

export function Chapter({ index, kicker, title, children, aside, flip = false, weight = "even", accent = "brand", id }: ChapterProps) {
  const markerClass = accent === "claude" ? "text-claude" : accent === "green" ? "text-green" : "text-gradient-brand";
  const ruleClass = accent === "claude" ? "bg-claude/30" : accent === "green" ? "bg-green/40" : "bg-gradient-brand opacity-40";

  return (
    <section id={id} className="relative scroll-mt-24 px-5 py-20 sm:px-6 sm:py-28 lg:py-32">
      <div className="relative z-10 mx-auto max-w-5xl">
        <div className={cn("grid items-center gap-12 md:gap-16 lg:gap-20", COLS[weight], flip && "md:[&>*:first-child]:order-2")}>
          {/* Narrative column. Always first in source so mobile reads text-then-figure. */}
          <Reveal effect={flip ? "right" : "left"} duration={700}>
            <div className="flex items-center gap-3">
              <span className={cn("font-mono text-sm font-bold tracking-tight tabular-nums", markerClass)}>{index}</span>
              <span className={cn("h-px w-8", ruleClass)} aria-hidden />
              <span className="text-[11px] font-semibold uppercase tracking-[0.2em] text-muted-foreground">{kicker}</span>
            </div>
            <h2 className="mt-5 text-balance text-3xl font-extrabold leading-[1.08] tracking-tight text-foreground sm:text-[40px] lg:text-[44px]">{title}</h2>
            <div className="mt-5 space-y-4 text-[15px] leading-relaxed text-muted-foreground sm:text-base">{children}</div>
          </Reveal>

          {/* Supporting visual column, sits openly on the page (no card). */}
          <Reveal effect={flip ? "left" : "right"} delay={120} duration={700} className="min-w-0">
            {aside}
          </Reveal>
        </div>
      </div>
    </section>
  );
}
