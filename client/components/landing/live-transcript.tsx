"use client";

import { BookOpen, Brain, CheckCircle2, ChevronRight, CircleDollarSign, NotebookPen } from "lucide-react";
import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";

import { ClaudeLogo } from "@/components/ui/brand-icons";
import { cn } from "@/lib/utils";

export type TranscriptLine =
  | { type: "text"; text: string }
  | { type: "thinking"; text: string }
  | {
      type: "tool";
      tool: string;
      payload: unknown;
      result?: unknown;
      open?: boolean;
    };

/*
A faux-live transcript: it streams its messages in one at a time, with a
Claude-Code-style working status line ("· Canoodling… (12m 24s · ↑ 54.1k
tokens)") that follows the latest item as the session "runs", then resolves
into an explicit truncated marker. It starts itself the first time it
scrolls into view, no manual trigger. Decorative, all client side.
*/

const EXECUTION = new Set(["buy_equity", "sell_equity", "buy_option", "sell_option", "cancel_order", "hold"]);
const DOCUMENTATION = new Set(["write_summary"]);

function toolMeta(name: string) {
  if (EXECUTION.has(name)) return { Icon: CircleDollarSign, eyebrow: "Execution tool called" };
  if (DOCUMENTATION.has(name)) return { Icon: NotebookPen, eyebrow: "Documentation tool called" };
  return { Icon: BookOpen, eyebrow: "Reading tool called" };
}

function toolAccent(name: string): string {
  if (name.startsWith("buy_")) return "text-green";
  if (name.startsWith("sell_")) return "text-red";
  if (name === "cancel_order") return "text-amber";
  if (name === "hold") return "text-muted-foreground";
  if (DOCUMENTATION.has(name)) return "text-claude";
  return "text-foreground/90";
}

// Whimsical gerunds in the spirit of the Claude Code status line. Dry, no slang.
const GERUNDS = ["Canoodling", "Cogitating", "Finagling", "Ruminating", "Conjuring", "Percolating", "Deliberating", "Marinating", "Pondering", "Scheming"];

const VISIBLE = 5;
// Deliberately unhurried so the "processing" beat reads for a while.
const STEP_MS = 1600;

function fmtTime(s: number): string {
  return `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, "0")}s`;
}
function fmtTokens(t: number): string {
  return `${(t / 1000).toFixed(1)}k tokens`;
}

export function LiveTranscript({ lines }: { lines: TranscriptLine[] }) {
  const shown = lines.slice(0, VISIBLE);
  const [step, setStep] = useState(0); // how many items are revealed
  const [gerund, setGerund] = useState(0);
  const [secs, setSecs] = useState(740);
  const [tokens, setTokens] = useState(12300);

  const rootRef = useRef<HTMLDivElement>(null);
  const started = useRef(false);

  // Reset the meters and stream from the first item.
  const start = useCallback(() => {
    setSecs(740);
    setTokens(12300);
    setGerund(0);
    setStep(1);
  }, []);

  // Start once the transcript first scrolls into view. No manual trigger: the
  // session begins streaming on its own when the reader reaches it. Falls back
  // to starting immediately where IntersectionObserver isn't available.
  useEffect(() => {
    const el = rootRef.current;
    if (!el || started.current) return;
    if (typeof IntersectionObserver === "undefined") {
      started.current = true;
      start();
      return;
    }
    const obs = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting && !started.current) {
            started.current = true;
            start();
            obs.disconnect();
          }
        }
      },
      { threshold: 0.35 },
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, [start]);

  // Reveal the next item on a timer.
  useEffect(() => {
    if (step === 0 || step >= shown.length) return;
    const t = setTimeout(() => setStep((s) => s + 1), STEP_MS);
    return () => clearTimeout(t);
  }, [step, shown.length]);

  const streaming = step > 0 && step < shown.length;

  // Tick the status line (gerund, clock, token meter) while it streams.
  useEffect(() => {
    if (!streaming) return;
    const id = setInterval(() => {
      setSecs((s) => s + 1);
      setTokens((t) => t + 1900 + ((t * 13) % 1700));
      setGerund((g) => (g + 1) % GERUNDS.length);
    }, 850);
    return () => clearInterval(id);
  }, [streaming]);

  const hiddenCount = lines.length - shown.length;

  return (
    <div ref={rootRef} className="min-w-0">
      <div className="flex items-center gap-2 border-b border-claude/20 pb-3 font-mono text-[11px] text-muted-foreground">
        <ClaudeLogo className="h-3.5 w-3.5" />
        <span className="font-semibold text-claude">Claudia</span>
        <span className="text-muted-foreground/40">/</span>
        session-2026-06-04
        {step > 0 ? (
          <span className="relative ml-auto flex h-2 w-2" aria-hidden>
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-70" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
          </span>
        ) : (
          <span className="ml-auto h-2 w-2 rounded-full bg-muted-foreground/30" aria-hidden />
        )}
      </div>

      {/* Every line is mounted from the first frame and revealed by fading in
          place (opacity + transform, which don't reflow), so the block holds
          its full height the whole time instead of growing line by line and
          shoving the page below it down as each event "streams" in. */}
      <ol className="mt-5 space-y-5">
        {shown.map((l, i) => {
          const revealed = step > i;
          return (
            <li
              key={`${l.type}-${i}`}
              aria-hidden={!revealed}
              className={cn(
                "transition-all duration-500 ease-out motion-reduce:transition-none",
                revealed ? "translate-y-0 opacity-100" : "pointer-events-none translate-y-1 opacity-0"
              )}
            >
              {l.type === "text" && <Block icon={<ClaudeLogo className="h-4 w-4" />} label="Narration" text={l.text} />}
              {l.type === "thinking" && <Block icon={<Brain className="h-3.5 w-3.5 text-claude" />} label="Extended thinking" text={l.text} italic />}
              {l.type === "tool" && <ToolRow tool={l.tool} payload={l.payload} result={l.result} open={l.open} />}
            </li>
          );
        })}
      </ol>

      {/* A fixed-height status rail: the idle, streaming, and truncated states
          all share one reserved slot, so swapping between them never changes
          the block's height either. */}
      <div className="relative mt-4 min-h-[3.25rem] font-mono text-[12px]">
        {step === 0 && (
          <div className="flex items-center gap-2 text-muted-foreground/60" aria-hidden>
            <span className="inline-flex h-2 w-2 animate-pulse rounded-full bg-muted-foreground/40" />
            Loading session&hellip;
          </div>
        )}
        {streaming && (
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-muted-foreground">
            <span className="inline-flex h-2 w-2 animate-pulse rounded-full bg-claude" aria-hidden />
            <span className="font-semibold text-claude">{GERUNDS[gerund]}&hellip;</span>
            <span className="text-muted-foreground/60">
              ({fmtTime(secs)} &middot; &uarr; {fmtTokens(tokens)})
            </span>
          </div>
        )}
        {!streaming && step >= shown.length && hiddenCount > 0 && (
          <div className="flex items-center gap-2 border-t border-dashed border-border/60 pt-4 text-[11px] text-muted-foreground/60">
            <span aria-hidden>&middot;&middot;&middot;</span>
            transcript truncated, {hiddenCount} more events this session
          </div>
        )}
      </div>
    </div>
  );
}

function Block({ icon, label, text, italic }: { icon: ReactNode; label: string; text: string; italic?: boolean }) {
  return (
    <div className="border-l-2 border-claude/40 pl-4">
      <div className="mb-1.5 flex items-center gap-2">
        {icon}
        <span className="text-xs font-semibold uppercase tracking-wider text-claude">{label}</span>
      </div>
      <p className={cn("text-[14px] leading-relaxed text-foreground/90", italic && "text-sm italic text-foreground/80")}>{text}</p>
    </div>
  );
}

function ToolRow({ tool, payload, result, open }: { tool: string; payload: unknown; result?: unknown; open?: boolean }) {
  const { Icon, eyebrow } = toolMeta(tool);
  const accent = toolAccent(tool);
  return (
    <div>
      <div className="flex w-full items-start gap-2.5 py-1 text-left">
        <ChevronRight className={cn("mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform", open && "rotate-90")} />
        <span className={cn("mt-px shrink-0", accent)}>
          <Icon className="h-3.5 w-3.5" />
        </span>
        <span className="min-w-0 flex-1 leading-tight">
          <span className="block text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{eyebrow}</span>
          <span className={cn("mt-0.5 block font-mono text-sm font-medium", accent)}>{tool}</span>
        </span>
        {open && result !== undefined && (
          <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-green-bg px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider text-green">
            <CheckCircle2 className="h-3 w-3" />
            success
          </span>
        )}
      </div>
      {open && (
        <div className="mt-1.5 ml-[26px] space-y-2.5">
          <div className="border-l-2 border-border/50 pl-4">
            <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Payload</div>
            <pre className="mt-0.5 whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-foreground/70">{JSON.stringify(payload, null, 2)}</pre>
          </div>
          {result !== undefined && (
            <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2">
              <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Returned</div>
              <pre className="mt-0.5 whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-foreground/70">{JSON.stringify(result, null, 2)}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
