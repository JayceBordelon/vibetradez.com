import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { CountUp } from "@/components/landing/count-up";
import { LiveTranscript } from "@/components/landing/live-transcript";
import { Reveal } from "@/components/landing/reveal";
import { cn } from "@/lib/utils";

/*
Supporting visuals for the narrative chapters. These sit OPENLY on the
page: large type, hairline rules at most, no boxes, no card fills, no
panel shadows. Each one fills the opposite column of a Chapter so every
beat has something for the eye to land on that is distinct from its
prose, without ever walling the content off behind a frame.
*/

/* ── A single oversized figure with a label, for the hook / setup beats.
      Open: a huge gradient numeral on the bare surface, a hairline tick,
      then the supporting copy. No box. ── */
export function StatBlock({ value, label, sub, accent = "brand" }: { value: ReactNode; label: string; sub?: string; accent?: "brand" | "claude" }) {
  return (
    <div className="md:pl-2">
      <div className={cn("font-mono text-[52px] font-extrabold leading-none tracking-tight tabular-nums sm:text-[68px]", accent === "claude" ? "text-claude" : "text-gradient-brand")}>{value}</div>
      <div className="mt-6 flex items-start gap-4">
        <span className={cn("mt-2 h-px w-8 shrink-0", accent === "claude" ? "bg-claude/40" : "bg-gradient-brand opacity-40")} aria-hidden />
        <div>
          <div className="text-sm font-bold tracking-tight text-foreground">{label}</div>
          {sub && <div className="mt-1.5 max-w-xs text-[13px] leading-relaxed text-muted-foreground">{sub}</div>}
        </div>
      </div>
    </div>
  );
}


/* ── Stepped vertical timeline, for the daily-loop beat. A soft gradient
      spine with open steps, no surrounding card. ── */
export function StepTimeline({ steps }: { steps: { time: string; title: string; detail: string; Icon: LucideIcon }[] }) {
  return (
    <ol className="relative space-y-8">
      <span className="absolute bottom-5 left-5 top-5 w-px -translate-x-1/2 bg-gradient-brand opacity-25" aria-hidden />
      {steps.map((s, i) => (
        <Reveal as="li" key={s.title} effect="rise" delay={i * 170} duration={600} className="relative flex gap-5">
          <span className="relative z-10 inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-background ring-1 ring-foreground/12">
            <s.Icon className="h-4 w-4 text-foreground/65" aria-hidden />
          </span>
          <div className="min-w-0 flex-1 pt-1">
            <span className="text-[10px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">{s.time}</span>
            <h3 className="mt-1 text-[15px] font-bold tracking-tight text-foreground">{s.title}</h3>
            <p className="mt-1 text-[13px] leading-relaxed text-muted-foreground">{s.detail}</p>
          </div>
        </Reveal>
      ))}
    </ol>
  );
}

/* ── Transcript snippet, for the receipts beat. A compact teaser that
      mirrors the real transcript viewer: a narration block, an extended
      thinking block in the clay accent, and a few collapsible tool-call
      rows with one example expanded to show its Payload and Returned
      sections. Static (no client state) so it can render on the server. ── */

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

// The transcript renders as a faux-live, self-streaming readout (client side),
// so the heavy lifting lives in LiveTranscript.
export function TranscriptCard({ lines }: { lines: TranscriptLine[] }) {
  return <LiveTranscript lines={lines} />;
}

/* ── Hard-cap list, for the guardrails beat. An open definition list:
      label on the left, a large tabular figure on the right, split by
      hairlines. No surrounding box. ── */
/* ── The tool manifest: the fixed, hand-written surface the model acts
      through. Three open groups of mono tool names, no card, with a "tested
      in CI" tag to reinforce that nothing here is improvised. ── */
export function ToolManifest({ groups }: { groups: { label: string; note: string; names: string[] }[] }) {
  return (
    <div className="space-y-6">
      <div className="flex items-baseline justify-between">
        <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">The toolbox</span>
        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-gradient-brand">tested in CI</span>
      </div>
      {groups.map((g) => (
        <div key={g.label}>
          <div className="flex items-baseline gap-3">
            <h4 className="text-[11px] font-semibold uppercase tracking-[0.16em] text-foreground">{g.label}</h4>
            <span className="h-px flex-1 bg-foreground/10" />
            <span className="text-[11px] text-muted-foreground/60">{g.note}</span>
          </div>
          <p className="mt-2.5 font-mono text-[13px] leading-relaxed text-foreground/75">{g.names.join("  ·  ")}</p>
        </div>
      ))}
    </div>
  );
}

export function CapList({ caps }: { caps: { label: string; num: number }[] }) {
  return (
    <div>
      <div className="flex items-baseline justify-between">
        <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">Hard caps</span>
        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-gradient-brand">enforced in code</span>
      </div>
      <dl className="mt-5">
        {caps.map((c) => (
          <div key={c.label} className="flex items-baseline justify-between gap-6 border-t border-border/70 py-4">
            <dt className="text-[14px] leading-snug text-muted-foreground">{c.label}</dt>
            <dd className="shrink-0 font-mono text-2xl font-extrabold tabular-nums text-foreground sm:text-3xl">
              <CountUp to={c.num} suffix="%" durationMs={1400} />
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
