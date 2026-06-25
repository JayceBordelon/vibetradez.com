import type { ReactNode } from "react";

import { Reveal } from "@/components/landing/reveal";
import { cn } from "@/lib/utils";

/*
The landing's story primitive: a clean two-column feature section with a
sage eyebrow label, a big calm headline with supporting prose on one side,
and a product visual on the other. `flip` alternates the columns so the
page zig-zags; `weight` varies the split so adjacent beats don't read as
the same block. Whitespace and soft cards only — no terminal chrome.

Prop names are retained from the previous (terminal) primitive so call
sites stay stable; `comment` is now the eyebrow label and `command` is
unused.
*/

interface LogSectionProps {
  id: string;
  command?: string;
  /** Eyebrow label above the headline. */
  comment?: string;
  /** Big headline. Pass JSX so callers can accent a span. */
  title: ReactNode;
  /** Supporting prose under the headline. */
  children: ReactNode;
  /** The product visual for the opposite column. */
  aside: ReactNode;
  /** When true, the visual sits LEFT and the text RIGHT (desktop). */
  flip?: boolean;
  weight?: "even" | "text" | "figure";
}

const COLS: Record<NonNullable<LogSectionProps["weight"]>, string> = {
  even: "lg:grid-cols-2",
  text: "lg:grid-cols-[1.05fr_0.95fr]",
  figure: "lg:grid-cols-[0.9fr_1.1fr]",
};

export function LogSection({ id, comment, title, children, aside, flip = false, weight = "even" }: LogSectionProps) {
  return (
    <section id={id} className="relative scroll-mt-20 px-5 py-16 sm:px-6 sm:py-20">
      <div className={cn("mx-auto grid max-w-5xl items-center gap-12 lg:gap-16", COLS[weight], flip && "lg:[&>*:first-child]:order-2")}>
        <Reveal effect={flip ? "right" : "left"} duration={700}>
          {comment && <p className="mb-4 text-xs font-semibold uppercase tracking-[0.14em] text-primary">{comment}</p>}
          <h2 className="font-display text-pretty text-3xl font-bold leading-[1.12] tracking-tight text-foreground sm:text-[38px]">{title}</h2>
          <div className="mt-5 space-y-4 text-[15px] leading-relaxed text-muted-foreground sm:text-base">{children}</div>
        </Reveal>

        <Reveal effect={flip ? "left" : "right"} delay={120} duration={700} className="min-w-0">
          {aside}
        </Reveal>
      </div>
    </section>
  );
}
