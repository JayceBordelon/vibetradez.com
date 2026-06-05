"use client";

import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";

/*
A lightweight sticky progress cue for the narrative scroll. Renders a
vertical rail of dots, one per chapter, pinned to the right edge on
large screens. The active dot tracks whichever chapter currently owns
the middle of the viewport (via a single IntersectionObserver), so the
reader always has a quiet sense of how far down the story they are.

Decorative-but-navigable: each dot is an anchor link to its chapter and
carries an accessible label. Hidden entirely on small screens and under
prefers-reduced-motion the transitions simply collapse to instant.
*/

interface ChapterMarker {
  id: string;
  label: string;
}

export function ChapterProgress({ chapters }: { chapters: ChapterMarker[] }) {
  const [active, setActive] = useState<string>(chapters[0]?.id ?? "");

  useEffect(() => {
    const sections = chapters.map((c) => document.getElementById(c.id)).filter((el): el is HTMLElement => el !== null);
    if (sections.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) setActive(entry.target.id);
        }
      },
      // Trip when a section crosses the vertical middle of the viewport.
      { rootMargin: "-45% 0px -45% 0px", threshold: 0 },
    );

    for (const el of sections) observer.observe(el);
    return () => observer.disconnect();
  }, [chapters]);

  return (
    <nav aria-label="Story progress" className="pointer-events-none fixed right-5 top-1/2 z-40 hidden -translate-y-1/2 lg:block">
      <ol className="flex flex-col items-end gap-3">
        {chapters.map((c, i) => {
          const isActive = c.id === active;
          return (
            <li key={c.id} className="pointer-events-auto flex items-center justify-end gap-2.5">
              <span className={cn("text-[10px] font-semibold uppercase tracking-[0.16em] text-muted-foreground transition-opacity duration-300", isActive ? "opacity-100" : "opacity-0")}>{c.label}</span>
              <a
                href={`#${c.id}`}
                aria-label={`Go to chapter ${i + 1}, ${c.label}`}
                aria-current={isActive ? "true" : undefined}
                className={cn(
                  "block h-2.5 w-2.5 rounded-full border transition-all duration-300",
                  isActive ? "scale-110 border-transparent bg-gradient-brand" : "border-foreground/25 bg-transparent hover:border-foreground/50",
                )}
              />
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
