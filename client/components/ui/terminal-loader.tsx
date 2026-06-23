"use client";

import { useEffect, useRef, useState } from "react";
import { cn } from "@/lib/utils";

/*
TerminalLoader is the app-wide loading placeholder. Instead of grey skeleton
blobs, a page that is still fetching plays a small terminal "boot" sequence in
the CRT phosphor theme the rest of the app already uses: the `$` command types
itself out, then the status lines light up one at a time, each running under a
braille spinner before it hands off to a green checkmark, while a block
progress bar fills and a scanline sweeps the panel. Loading reads like the
product booting rather than a generic shimmer. The cascade loops, so however
long the fetch takes the panel always looks alive.

All motion is driven client-side after mount, so the SSR/first paint is a
static "t+0.0s, nothing booted yet" frame that hydrates cleanly. Under
prefers-reduced-motion the effects short-circuit to a finished, static state
(command shown, every line checked) and the ambient CRT animations are killed
in CSS. role=status + aria-busy keep it announced to assistive tech; every
animated glyph is aria-hidden.
*/

// Braille spinner — the classic 10-frame ⠋⠙⠹… cycle, advanced on its own tick.
const SPINNER_FRAMES = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

function ProgressBar({ ratio, cells }: { ratio: number; cells: number }) {
  const filled = Math.round(ratio * cells);
  const pct = Math.round(ratio * 100);
  return (
    <div className="mt-3.5 flex items-center gap-2" aria-hidden>
      <span className="phosphor tracking-[0.05em] text-green/80">
        {"█".repeat(filled)}
        <span className="text-green/20">{"░".repeat(Math.max(0, cells - filled))}</span>
      </span>
      <span className="tabular-nums text-[11px] text-muted-foreground/70">{pct}%</span>
    </div>
  );
}

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
  const total = lines.length;
  // typed = chars of the command revealed so far. step = number of lines that
  // have finished in the current pass (so step is also the index of the line
  // currently running, until it reaches `total` = all done). frame = spinner
  // position. elapsed = tenths of a second since mount, for the t+ counter.
  const [typed, setTyped] = useState(0);
  const [step, setStep] = useState(0);
  const [frame, setFrame] = useState(0);
  const [elapsed, setElapsed] = useState(0);
  const [reduced, setReduced] = useState(false);
  const commandDone = typed >= command.length;
  const stepReady = useRef(false);

  // Honor reduced motion: jump to the finished frame and run no timers.
  useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    if (mq.matches) {
      setReduced(true);
      setTyped(command.length);
      setStep(total);
    }
  }, [command.length, total]);

  // Type the command out, one character at a time.
  useEffect(() => {
    if (reduced) return;
    setTyped(0);
    let i = 0;
    const id = setInterval(() => {
      i += 1;
      setTyped(i);
      if (i >= command.length) clearInterval(id);
    }, 42);
    return () => clearInterval(id);
  }, [command, reduced]);

  // Advance the spinner glyph independently of the line cascade.
  useEffect(() => {
    if (reduced) return;
    const id = setInterval(() => setFrame((f) => (f + 1) % SPINNER_FRAMES.length), 90);
    return () => clearInterval(id);
  }, [reduced]);

  // Tick the elapsed counter in tenths of a second.
  useEffect(() => {
    if (reduced) return;
    const id = setInterval(() => setElapsed((e) => e + 1), 100);
    return () => clearInterval(id);
  }, [reduced]);

  // Cascade through the status lines once the command has finished typing.
  // When every line is done we hold on the "all green" frame for one beat,
  // then reboot the cascade so the panel never goes static mid-fetch.
  stepReady.current = commandDone;
  useEffect(() => {
    if (reduced || total === 0) return;
    const id = setInterval(() => {
      if (!stepReady.current) return;
      setStep((s) => (s >= total ? 0 : s + 1));
    }, 720);
    return () => clearInterval(id);
  }, [reduced, total]);

  const cells = compact ? 12 : 18;
  const ratio = total === 0 ? 1 : step / total;
  const elapsedLabel = `t+${(elapsed / 10).toFixed(1)}s`;

  return (
    <div className={cn("flex w-full items-center justify-center", minHeightClass, className)} role="status" aria-busy="true">
      <div
        className={cn(
          "vt-loader vt-loader-glow w-full border border-dashed border-border/70 bg-card-elevated/20 font-mono",
          compact ? "max-w-sm px-4 py-3.5 text-[12px]" : "max-w-md px-5 py-5 text-[13px]"
        )}
      >
        {/* Sweeping scanline, drawn above the content but below text legibility. */}
        <span className="vt-loader-scan" aria-hidden />

        {/* HUD meta row: a session label and a live uptime counter. */}
        <div className="mb-3 flex items-center justify-between border-b border-dashed border-border/50 pb-2 text-[10px] uppercase tracking-[0.18em] text-muted-foreground/60">
          <span aria-hidden>vt · boot sequence</span>
          <span className="tabular-nums text-green/70" aria-hidden>
            {elapsedLabel}
          </span>
        </div>

        {/* The command line: types itself out, trailed by a blinking caret. */}
        <div className="flex items-center gap-2">
          <span className="phosphor font-bold text-green" aria-hidden>
            $
          </span>
          <span className="text-foreground/90">{reduced ? command : command.slice(0, typed)}</span>
          <span className="term-caret" aria-hidden />
        </div>

        {/* Status lines: pending → running (spinner) → done (check). */}
        <ul className={cn("space-y-1.5", compact ? "mt-3" : "mt-3.5")}>
          {lines.map((line, i) => {
            const done = i < step;
            const active = i === step && !reduced;
            return (
              <li
                key={i}
                className={cn(
                  "flex items-center gap-2 transition-colors duration-300",
                  done ? "text-muted-foreground/70" : active ? "text-foreground/90" : "text-muted-foreground/40"
                )}
              >
                <span aria-hidden className={cn("inline-flex w-3 shrink-0 justify-center", active && "phosphor")}>
                  {done ? (
                    <span className="text-green">✓</span>
                  ) : active ? (
                    <span className="text-green">{SPINNER_FRAMES[frame]}</span>
                  ) : (
                    <span className="text-muted-foreground/40">◦</span>
                  )}
                </span>
                <span className="min-w-0 truncate">{line}</span>
                {active && (
                  <span aria-hidden className="text-muted-foreground/40">
                    &hellip;
                  </span>
                )}
              </li>
            );
          })}
        </ul>

        <ProgressBar ratio={ratio} cells={cells} />
        <span className="sr-only">Loading</span>
      </div>
    </div>
  );
}
