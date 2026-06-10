"use client";

import { useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

/*
Typed headline for the CRT hero. Lines appear character by character with
a blinking block caret riding the live edge, like a session booting up.

Layout never shifts while typing: every character is always rendered and
untyped ones are just invisible, so the block claims its final size on
the server render. Screen readers get the full text immediately via an
sr-only copy while the animated copy stays aria-hidden. Under
prefers-reduced-motion the whole headline renders instantly.
*/

interface TypeLinesProps {
  lines: string[];
  /** ms per character */
  speed?: number;
  /** ms before the first character lands */
  startDelay?: number;
  className?: string;
}

export function TypeLines({ lines, speed = 55, startDelay = 350, className }: TypeLinesProps) {
  const total = lines.reduce((n, l) => n + l.length, 0);
  const [typed, setTyped] = useState(0);
  const reduced = useRef(false);

  useEffect(() => {
    reduced.current = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (reduced.current) {
      setTyped(total);
      return;
    }
    let count = 0;
    let timer: ReturnType<typeof setTimeout>;
    const tick = () => {
      count += 1;
      setTyped(count);
      if (count < total) timer = setTimeout(tick, speed);
    };
    timer = setTimeout(tick, startDelay);
    return () => clearTimeout(timer);
  }, [total, speed, startDelay]);

  // How many characters of line i are visible given `typed` overall. The
  // caret rides the first line that is still incomplete and parks on the
  // last line once everything has landed.
  let remaining = typed;
  let caretPlaced = false;
  const done = typed >= total;

  return (
    <span className={cn("block", className)}>
      <span className="sr-only">{lines.join(" ")}</span>
      {lines.map((line, i) => {
        const visible = Math.max(0, Math.min(line.length, remaining));
        remaining -= visible;
        const last = i === lines.length - 1;
        const caretHere = !caretPlaced && (done ? last : visible < line.length);
        if (caretHere) caretPlaced = true;
        return (
          <span key={line} aria-hidden className="block whitespace-nowrap">
            {line.split("").map((ch, j) => (
              <span key={`${i}-${j}`} className={j < visible ? undefined : "opacity-0"}>
                {ch}
              </span>
            ))}
            {caretHere && <span className="term-caret" />}
          </span>
        );
      })}
    </span>
  );
}
