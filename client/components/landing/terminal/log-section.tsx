import type { ReactNode } from "react";

import { Reveal } from "@/components/landing/reveal";
import { cn } from "@/lib/utils";

import { PromptLine } from "./terminal-visuals";

/*
LogSection is the terminal page's story primitive, the successor to the
old Chapter. Every beat opens with a dashed rule and a shell prompt (the
command IS the section label), then a two-column grid: an oversized
display-mono headline with supporting prose on one side, command output
on the other. `flip` alternates the columns so the log zig-zags, `weight`
varies the split so adjacent beats never read as the same block.

No cards, no bands: hairlines and whitespace only, like one continuous
scrollback buffer.
*/

interface LogSectionProps {
  id: string;
  /** The shell command that opens the beat, e.g. "cat 01_setup.txt". */
  command: string;
  /** Trailing comment after the command, the section's quiet aside. */
  comment?: string;
  /** Oversized headline. Pass JSX so callers can phosphor a span. */
  title: ReactNode;
  /** Supporting prose under the headline. */
  children: ReactNode;
  /** The command-output visual for the opposite column. */
  aside: ReactNode;
  /** When true, the output sits LEFT and the text RIGHT (desktop). */
  flip?: boolean;
  weight?: "even" | "text" | "figure";
}

const COLS: Record<NonNullable<LogSectionProps["weight"]>, string> = {
  even: "md:grid-cols-2",
  text: "md:grid-cols-[1.2fr_0.8fr]",
  figure: "md:grid-cols-[0.85fr_1.15fr]",
};

export function LogSection({ id, command, comment, title, children, aside, flip = false, weight = "even" }: LogSectionProps) {
  return (
    <section id={id} className="relative scroll-mt-20 px-5 py-16 sm:px-6 sm:py-20 lg:py-24">
      <div className="mx-auto max-w-5xl">
        <Reveal effect="fade" duration={700}>
          <div className="border-t border-dashed border-border pt-4">
            <PromptLine command={command} comment={comment} />
          </div>
        </Reveal>

        <div className={cn("mt-10 grid items-center gap-12 sm:mt-12 md:gap-16", COLS[weight], flip && "md:[&>*:first-child]:order-2")}>
          {/* Narrative column. Always first in source so mobile reads text-then-output. */}
          <Reveal effect={flip ? "right" : "left"} duration={700}>
            <h2 className="term-display text-balance text-[26px] font-extrabold uppercase leading-[1.12] tracking-tight text-foreground sm:text-[34px] lg:text-[38px]">{title}</h2>
            <div className="mt-5 space-y-4 text-[15px] leading-relaxed text-muted-foreground sm:text-base">{children}</div>
          </Reveal>

          {/* Output column, sits openly on the page like scrollback. */}
          <Reveal effect={flip ? "left" : "right"} delay={120} duration={700} className="min-w-0">
            {aside}
          </Reveal>
        </div>
      </div>
    </section>
  );
}
