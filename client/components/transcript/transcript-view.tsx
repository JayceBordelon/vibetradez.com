"use client";

import { ArrowLeft, ArrowRight, BookOpen, Braces, Brain, CheckCircle2, CircleDollarSign, Globe, NotebookPen, SquareTerminal, XCircle } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { TranscriptEvent, TranscriptResponse, TranscriptUsage } from "@/types/trade";

import { resultStatus, safeStringify } from "./format";
import { ToolBody } from "./tool-views";

type Kind = "selection" | "execution" | "portfolio";

const KIND_COPY: Record<Kind, { title: string; blurb: string; stage: string }> = {
  selection: {
    title: "Selection reasoning",
    blurb: "How Claudia chose today's tickers and contract intent before the bell.",
    stage: "9:25 ET picker",
  },
  execution: {
    title: "Execution reasoning",
    blurb: "How Claudia picked contracts off the live chain and placed orders at the open.",
    stage: "9:30 ET at-open agent",
  },
  portfolio: {
    title: "Session ledger",
    blurb: "Everything Claudia looked at and every move she made with the account that day, across all of the day's sessions, rendered entry by entry like a desk blotter.",
    stage: "open · midday · pre-close",
  },
};

const MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

// prettyDate turns a YYYY-MM-DD string into "June 4, 2026". Parsed from the
// parts (not new Date(str)) so it never shifts a day across timezones.
function prettyDate(d: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(d.trim());
  if (!m) return d;
  const month = MONTHS[Number(m[2]) - 1];
  if (!month) return d;
  return `${month} ${Number(m[3])}, ${m[1]}`;
}

// sessionAnchor turns a session-marker label ("Open session · 9:45 AM ET")
// into a stable anchor id the jump bar links to.
function sessionAnchor(text: string): string {
  const t = (text ?? "").toLowerCase();
  if (t.includes("open")) return "session-open";
  if (t.includes("midday")) return "session-midday";
  if (t.includes("close")) return "session-close";
  return `session-${t.replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "").slice(0, 24) || "x"}`;
}

// sessionShort is the compact jump-bar chip label: the part before the "·",
// with a trailing "session" trimmed ("Open session · 9:45 AM ET" → "Open").
function sessionShort(text: string): string {
  const head = (text ?? "").split("·")[0]?.trim() ?? "";
  return head.replace(/\s*session$/i, "") || "Session";
}

type LoadState = { kind: "loading" } | { kind: "ready"; data: TranscriptResponse } | { kind: "error"; message: string };

export function TranscriptView({ date, kind }: { date: string; kind: Kind }) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const copy = KIND_COPY[kind];

  useEffect(() => {
    let cancelled = false;
    setState({ kind: "loading" });
    api
      .getTranscript(date, kind)
      .then((data) => {
        if (!cancelled) setState({ kind: "ready", data });
      })
      .catch((e: unknown) => {
        if (!cancelled) setState({ kind: "error", message: e instanceof Error ? e.message : "Failed to load transcript" });
      });
    return () => {
      cancelled = true;
    };
  }, [date, kind]);

  return (
    <div className="mx-auto min-w-0 max-w-[900px] px-4 py-6 sm:px-7">
      <Link
        href="/dashboard"
        className="mb-6 inline-flex min-h-9 items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to dashboard
      </Link>

      {/* Header */}
      <div className="flex flex-wrap items-center gap-2">
        <ClaudeLogo className="h-5 w-5" />
        <h1 className="term-display text-xl font-extrabold uppercase tracking-tight sm:text-2xl">
          {copy.title} for {prettyDate(date)}
        </h1>
        <Badge variant="secondary">{copy.stage}</Badge>
      </div>
      <p className="mt-2 text-sm text-muted-foreground">{copy.blurb}</p>

      {kind === "portfolio" && <SessionPager date={date} />}

      {state.kind === "loading" && <TranscriptSkeleton />}
      {state.kind === "error" && <StatusBlock tone="error" title="Couldn't load the transcript" body={state.message} />}
      {state.kind === "ready" && (!state.data.available || (state.data.events ?? []).length === 0) && <EmptyTranscript date={date} kind={kind} />}
      {state.kind === "ready" && state.data.available && (state.data.events ?? []).length > 0 && <TranscriptBody data={state.data} />}
    </div>
  );
}

/*
SessionPager is the transcript's wayfinding strip: step to the previous or
next trading day (weekends skipped, forward capped at today in market time).
Sessions are browsable instead of URL-editable. The session summary now leads
the page, so there is no jump-to-bottom link.
*/
function stepTradingDay(date: string, dir: 1 | -1): string {
  const d = new Date(`${date}T12:00:00Z`);
  do {
    d.setUTCDate(d.getUTCDate() + dir);
  } while (d.getUTCDay() === 0 || d.getUTCDay() === 6);
  return d.toISOString().slice(0, 10);
}

function SessionPager({ date }: { date: string }) {
  const prev = stepTradingDay(date, -1);
  const next = stepTradingDay(date, 1);
  const showNext = next <= etNow().date;
  return (
    <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 border-y border-dashed border-border py-2.5 font-mono text-[12px]">
      <Link href={`/transcripts/${prev}`} className="inline-flex min-h-9 items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground sm:min-h-0">
        <ArrowLeft className="h-3.5 w-3.5" aria-hidden />
        {prettyDate(prev)}
      </Link>
      <span className="text-muted-foreground/40" aria-hidden>
        //
      </span>
      {showNext ? (
        <Link href={`/transcripts/${next}`} className="inline-flex min-h-9 items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground sm:min-h-0">
          {prettyDate(next)}
          <ArrowRight className="h-3.5 w-3.5" aria-hidden />
        </Link>
      ) : (
        <span className="inline-flex min-h-9 items-center text-muted-foreground/40 sm:min-h-0">latest session</span>
      )}
    </div>
  );
}

/*
The empty state is time-aware, because "no transcript" means different
things: a future date (the session hasn't happened), today before the
session window (it hasn't run yet), today right after the window (it may
still be running), a weekend (markets closed), or a genuinely blank past
trading day. All boundaries are computed in market time (America/New_York)
so overnight hours never blame a day that hasn't started. Only the live
portfolio kind knows a schedule; archived v1 kinds get the generic past
message.
*/
const ET_NOW_FMT = new Intl.DateTimeFormat("en-CA", {
  timeZone: "America/New_York",
  hour12: false,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});

function etNow(): { date: string; minutes: number } {
  const parts: Record<string, string> = {};
  for (const p of ET_NOW_FMT.formatToParts(new Date())) parts[p.type] = p.value;
  const hour = Number(parts.hour) % 24; // en-CA can render midnight as "24"
  return { date: `${parts.year}-${parts.month}-${parts.day}`, minutes: hour * 60 + Number(parts.minute) };
}

// Day of week for a YYYY-MM-DD without timezone drift (anchored to UTC noon).
function weekdayOf(date: string): number {
  return new Date(`${date}T12:00:00Z`).getUTCDay();
}

// Claudia runs three sessions a trading day; the OPEN is the earliest
// (~9:45 ET), so "today" messaging keys off it: before 9:45 nothing has run,
// and the first transcript lands shortly after the open session wraps.
const SESSION_OPEN_MIN = 9 * 60 + 45; // first session (open) ~9:45 AM ET
const SESSION_OPEN_SETTLED_MIN = 10 * 60 + 30; // its transcript is saved by ~10:30 AM ET

function EmptyTranscript({ date, kind }: { date: string; kind: Kind }) {
  const human = prettyDate(date);
  const dow = weekdayOf(date);

  if (dow === 0 || dow === 6) {
    return <StatusBlock tone="muted" title="Markets closed" body={`${human} ${dow === 6 ? "is a Saturday" : "is a Sunday"}. Sessions only run on trading days, so there is nothing to replay here.`} />;
  }

  if (kind === "portfolio") {
    const now = etNow();
    if (date > now.date) {
      return <StatusBlock tone="muted" title="Hasn't happened yet" body={`${human} is still in the future. Claudia runs three sessions on a trading day — the open (~9:45 AM Eastern), midday (~12:30 PM), and pre-close (~3:30 PM) — and the day's transcript lands here once the first one wraps.`} />;
    }
    if (date === now.date && now.minutes < SESSION_OPEN_MIN) {
      return <StatusBlock tone="muted" title="Hasn't run yet" body={`Today's first session (the open) runs around 9:45 AM Eastern. Claudia trades three times a day, and each session is appended here as it wraps.`} />;
    }
    if (date === now.date && now.minutes < SESSION_OPEN_SETTLED_MIN) {
      return <StatusBlock tone="muted" title="Session in progress" body={`Today's open session is running right now (or just wrapped). The transcript appears here as soon as it is saved — check back in a few minutes.`} />;
    }
  }

  return <StatusBlock tone="muted" title="No transcript recorded" body={`No session activity was recorded for ${human}. That usually means a market holiday, or a session that failed before anything could be recorded.`} />;
}

// summaryText normalizes a write_summary field (synopsis / action_items),
// which is normally a string but tolerates an array of strings from older
// seeded transcripts.
function summaryText(v: unknown): string {
  if (typeof v === "string") return v.trim();
  if (Array.isArray(v)) return v.filter((x) => typeof x === "string").join("\n");
  return "";
}

/*
SessionSummaries lifts each session's write_summary (synopsis + the action
items it left for next time) to the TOP of the page, so the conclusion leads
instead of hiding at the bottom of a long ledger. On a multi-session day each
summary is labeled with the session it closed (the most recent session marker
before it). The write_summary entry still renders inline in the ledger below
as the verbatim record.
*/
function SessionSummaries({ events }: { events: TranscriptEvent[] }) {
  const items: { label: string | null; synopsis: string; actions: string }[] = [];
  let label: string | null = null;
  for (const e of events) {
    if (e.type === "session_marker") {
      label = sessionShort(e.text ?? "");
      continue;
    }
    if (e.type === "tool_use" && e.tool_name === "write_summary") {
      const input = (e.tool_input ?? {}) as { synopsis?: unknown; action_items?: unknown };
      const synopsis = summaryText(input.synopsis);
      const actions = summaryText(input.action_items);
      if (synopsis || actions) items.push({ label, synopsis, actions });
    }
  }
  if (items.length === 0) return null;
  const multi = items.length > 1;
  return (
    <section aria-label="Session summary" className="mb-6 rounded-lg border border-claude-border bg-claude-light px-4 py-4 sm:px-5">
      <div className="mb-3 flex items-center gap-2">
        <NotebookPen className="h-4 w-4 text-claude" aria-hidden />
        <h2 className="font-mono text-[11px] font-semibold uppercase tracking-[0.16em] text-claude">{multi ? "Session summaries" : "Session summary"}</h2>
      </div>
      <div className="space-y-4">
        {items.map((it, i) => (
          <div key={`summary-${i}`} className={i > 0 ? "border-t border-claude-border/40 pt-4" : undefined}>
            {multi && it.label && <p className="mb-1.5 font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{it.label}</p>}
            {it.synopsis && <p className="wrap-anywhere whitespace-pre-wrap text-[15px] leading-relaxed text-foreground/90">{it.synopsis}</p>}
            {it.actions && (
              <div className="mt-2.5">
                <p className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">Action items for next session</p>
                <p className="mt-1 wrap-anywhere whitespace-pre-wrap text-[13px] leading-relaxed text-muted-foreground">{it.actions}</p>
              </div>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}

function TranscriptBody({ data }: { data: TranscriptResponse }) {
  const events = data.events ?? [];
  const usage = data.usage;
  const durationMs = data.duration_ms ?? 0;
  // Session markers (open / midday / pre-close) drive the jump bar and anchor
  // each session within the merged day.
  const sessionMarkers = events.filter((e) => e.type === "session_marker");
  return (
    <div className="mt-6">
      <SessionSummaries events={events} />
      {sessionMarkers.length > 1 && (
        <div className="mb-5 flex flex-wrap items-center gap-2 rounded-lg border border-border/60 bg-card-elevated/40 px-3 py-2.5">
          <span className="font-mono text-[10px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">Sessions</span>
          {sessionMarkers.map((m) => (
            <a
              key={sessionAnchor(m.text ?? "")}
              href={`#${sessionAnchor(m.text ?? "")}`}
              className="inline-flex min-h-7 items-center rounded-md border border-border/60 px-2 font-mono text-[11px] font-medium text-muted-foreground transition-colors hover:border-claude/40 hover:text-claude"
            >
              {sessionShort(m.text ?? "")}
            </a>
          ))}
        </div>
      )}
      {data.model && (
        <div className="mb-5 grid grid-cols-2 gap-x-6 gap-y-2.5 rounded-lg border border-border/60 bg-card-elevated/60 px-4 py-3 sm:grid-cols-5">
          <MetaCell label="Model" value={data.model} />
          {data.created_at && <MetaCell label="Captured" value={new Date(data.created_at).toLocaleString()} />}
          {durationMs > 0 && <MetaCell label="Wall clock" value={formatDuration(durationMs)} />}
          <MetaCell label="Entries" value={String(events.length)} />
          {usage && usage.rounds > 0 && <MetaCell label="Rounds" value={String(usage.rounds)} />}
        </div>
      )}
      {usage && usage.rounds > 0 && <UsageBreakdown usage={usage} model={data.model} />}
      <ol className="space-y-5">
        {(() => {
          // Fold each tool_result into its matching tool_use so a call and
          // its outcome render as one ledger entry, not two stacked ones. The
          // result is the tool_result that follows a tool_use (matched by
          // tool_use_id when both carry one, else by adjacency so seeded /
          // id-less transcripts still pair).
          const resultFor = new Map<number, string>();
          for (let i = 0; i < events.length; i++) {
            if (events[i].type !== "tool_use") continue;
            for (let j = i + 1; j < events.length; j++) {
              if (events[j].type === "tool_use") break;
              if (events[j].type === "tool_result") {
                const u = events[i].tool_use_id;
                const r = events[j].tool_use_id;
                if (!u || !r || u === r) resultFor.set(i, events[j].tool_result ?? "");
                break;
              }
            }
          }
          // A thin labeled rule at each round boundary gives the multi-
          // thousand-pixel scroll its landmarks; the header advertises the
          // round count but the body otherwise never shows where one ends.
          let lastRound: number | undefined;
          return events
            .map((ev, i) => ({ ev, i }))
            .filter(({ ev }) => ev.type !== "tool_result")
            .map(({ ev, i }) => {
              // A session marker is itself the divider, so it suppresses the
              // round rule that would otherwise stack right on top of it.
              const newRound = ev.round !== lastRound && lastRound !== undefined && ev.type !== "session_marker";
              lastRound = ev.round;
              // Each session marker carries its slot anchor so the jump bar can
              // scroll to it; nothing else needs an id now that the summary
              // leads the page.
              const anchorId = ev.type === "session_marker" ? sessionAnchor(ev.text ?? "") : undefined;
              return (
                <li key={`${ev.round}-${ev.type}-${ev.tool_use_id ?? i}`} id={anchorId} className={anchorId ? "scroll-mt-24" : undefined}>
                  {newRound && (
                    // Rounds are 0-indexed in the event stream; display
                    // 1-indexed so the labels agree with the header's
                    // "N rounds" count (the unlabeled preamble is Round 1).
                    <div className="mb-5 flex items-center gap-3" aria-hidden>
                      <span className="h-px flex-1 bg-border" />
                      <span className="font-mono text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">Round {ev.round + 1}</span>
                      <span className="h-px flex-1 bg-border" />
                    </div>
                  )}
                  <EventBlock event={ev} resultPayload={resultFor.get(i)} />
                </li>
              );
            });
        })()}
      </ol>
      <p className="mt-8 text-[11px] leading-relaxed text-muted-foreground">
        This is the model's own narration and tool activity, captured verbatim from the run. This is a single public
        account, so balances are shown openly rather than hidden. Not financial advice.
      </p>
    </div>
  );
}

function MetaCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className="text-[10px] uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className="truncate font-mono text-[11px] text-foreground/85" title={value}>
        {value}
      </div>
    </div>
  );
}

// Tools fall into groups, each with its own icon and eyebrow in the ledger:
// EXECUTION tools move money (or record the explicit no-op), DOCUMENTATION
// tools only write to the record, WEB and CODE tools run on Anthropic's
// side, and everything else is a read-only reading tool.
const EXECUTION_TOOLS = new Set(["buy_equity", "sell_equity", "buy_option", "sell_option", "cancel_order", "hold"]);
const DOCUMENTATION_TOOLS = new Set(["write_summary"]);
const WEB_TOOLS = new Set(["web_search", "web_fetch"]);
const CODE_TOOLS = new Set(["code_execution", "bash_code_execution", "text_editor_code_execution"]);

type ToolGroup = "execution" | "documentation" | "web" | "code" | "reading";

function toolGroup(name?: string): ToolGroup {
  if (name && EXECUTION_TOOLS.has(name)) return "execution";
  if (name && DOCUMENTATION_TOOLS.has(name)) return "documentation";
  if (name && WEB_TOOLS.has(name)) return "web";
  if (name && CODE_TOOLS.has(name)) return "code";
  return "reading";
}

function toolIcon(group: ToolGroup) {
  switch (group) {
    case "execution":
      return CircleDollarSign;
    case "documentation":
      return NotebookPen;
    case "web":
      return Globe;
    case "code":
      return SquareTerminal;
    default:
      return BookOpen;
  }
}

function toolTag(group: ToolGroup): string {
  switch (group) {
    case "execution":
      return "execution";
    case "documentation":
      return "documentation";
    case "web":
      return "web · anthropic-run";
    case "code":
      return "sandbox · anthropic-run";
    default:
      return "reading";
  }
}

// toolAccentClass colors a tool by intent: buys green, sells red, cancels
// amber, documentation clay. Reading tools stay neutral.
function toolAccentClass(name?: string): string {
  if (!name) return "text-foreground/90";
  if (name.startsWith("buy_")) return "text-green";
  if (name.startsWith("sell_")) return "text-red";
  if (name === "cancel_order") return "text-amber";
  if (name === "hold") return "text-muted-foreground";
  if (DOCUMENTATION_TOOLS.has(name)) return "text-claude";
  return "text-foreground/90";
}

/*
parseStanceText unwraps the session's terminal contract: the agent's last
message is required to be a bare JSON object holding only its closing
stance, which read as a wall of {"stance": "..."} in the ledger. When a
narration block IS that object (with or without a stray code fence), the
prose inside is the content; anything else renders verbatim.
*/
function parseStanceText(text: string): string | null {
  let t = text.trim();
  if (t.startsWith("```")) {
    t = t
      .replace(/^```[a-z]*\s*/i, "")
      .replace(/```\s*$/, "")
      .trim();
  }
  if (!t.startsWith("{")) return null;
  try {
    const obj: unknown = JSON.parse(t);
    if (obj && typeof obj === "object" && "stance" in obj && typeof (obj as { stance: unknown }).stance === "string") {
      const stance = (obj as { stance: string }).stance.trim();
      if (stance) return stance;
    }
  } catch {
    return null;
  }
  return null;
}

function EventBlock({ event, resultPayload }: { event: TranscriptEvent; resultPayload?: string }) {
  switch (event.type) {
    case "text": {
      const stance = parseStanceText(event.text ?? "");
      return (
        <div className="border-l-2 border-claude/40 pl-4">
          <div className="mb-1.5 flex items-center gap-2">
            <ClaudeLogo className="h-4 w-4" />
            <span className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-claude">{stance ? "Closing stance" : "Narration"}</span>
          </div>
          <p className="wrap-anywhere whitespace-pre-wrap font-serif text-[15px] leading-relaxed text-foreground/90">{stance ?? event.text}</p>
        </div>
      );
    }
    case "thinking":
      // Neutral border + muted label, NOT the claude accent: narration and
      // thinking must stay distinguishable at scroll speed.
      return (
        <div className="border-l-2 border-muted-foreground/25 pl-4">
          <div className="mb-1.5 flex items-center gap-2">
            <Brain className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">Extended thinking</span>
          </div>
          <p className="whitespace-pre-wrap text-[13px] italic leading-relaxed text-muted-foreground">{event.text}</p>
        </div>
      );
    case "tool_use":
      return <ToolSlip event={event} resultRaw={resultPayload} />;
    case "session_marker":
      // The seam between two intraday sessions (open / midday / pre-close)
      // merged into one day's ledger: a labeled divider, anchored on its <li>.
      return (
        <div className="flex items-center gap-3 py-1">
          <span className="h-px flex-1 bg-claude/30" aria-hidden />
          <span className="font-mono text-[11px] font-semibold uppercase tracking-[0.16em] text-claude">{event.text}</span>
          <span className="h-px flex-1 bg-claude/30" aria-hidden />
        </div>
      );
    default:
      return null;
  }
}

/*
toolSummary flattens a call's input into a compact "k: v · k: v" line so the
entry header still says what the call touched (ticker, qty, price) without
reading the body. Primitive values and short primitive arrays only.
*/
function toolSummary(input: unknown): string {
  if (!input || typeof input !== "object" || Array.isArray(input)) return "";
  const parts: string[] = [];
  for (const [k, v] of Object.entries(input as Record<string, unknown>)) {
    if (parts.length >= 4) break;
    if (v == null) continue;
    if (Array.isArray(v)) {
      const prims = v.filter((x) => typeof x === "string" || typeof x === "number");
      if (prims.length > 0) parts.push(`${k}: ${prims.slice(0, 6).join(", ")}${prims.length > 6 ? ", …" : ""}`);
    } else if (typeof v === "string" || typeof v === "number" || typeof v === "boolean") {
      const s = String(v);
      parts.push(`${k}: ${s.length > 48 ? `${s.slice(0, 48)}…` : s}`);
    }
  }
  return parts.join(" · ");
}

/*
ToolSlip is one ledger entry: a compact header line (icon, tool name, input
hint, group tag, outcome) over the purpose-built body for that tool, with a
per-entry raw toggle that reveals the verbatim input JSON and result string.
Raw data is always reachable but never the primary rendering.
*/
function ToolSlip({ event, resultRaw }: { event: TranscriptEvent; resultRaw?: string }) {
  const [showRaw, setShowRaw] = useState(false);
  const name = event.tool_name ?? "tool";
  const group = toolGroup(event.tool_name);
  const Icon = toolIcon(group);
  const status = resultStatus(resultRaw);
  const accent = toolAccentClass(event.tool_name);
  const hint = toolSummary(event.tool_input);

  return (
    <div>
      <div className="flex items-center gap-2">
        <span className={cn("shrink-0", accent)} aria-hidden>
          <Icon className="h-3.5 w-3.5" />
        </span>
        <span className={cn("shrink-0 font-mono text-[13px] font-semibold", accent)}>{name}</span>
        {hint && <span className="hidden min-w-0 truncate text-[11px] text-muted-foreground sm:block">{hint}</span>}
        <span className="ml-auto hidden shrink-0 font-mono text-[9px] uppercase tracking-[0.14em] text-muted-foreground/80 sm:block">{toolTag(group)}</span>
        {/* Success is the overwhelming default, so it gets a quiet check
            instead of a repeated green pill; only failures get the loud
            badge, and a missing result gets a muted note pill. */}
        {status === "success" && (
          <span className="inline-flex shrink-0 items-center sm:ml-0 ml-auto" title="success">
            <CheckCircle2 className="h-3.5 w-3.5 text-green/70" />
            <span className="sr-only">success</span>
          </span>
        )}
        {status === "error" && (
          <span className="ml-auto inline-flex shrink-0 items-center gap-1 rounded-full border border-red-border bg-red-bg px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-red sm:ml-0">
            <XCircle className="h-3 w-3" aria-hidden />
            error
          </span>
        )}
        {status === undefined && (
          <span className="ml-auto inline-flex shrink-0 items-center rounded-full border border-border bg-muted px-2 py-0.5 font-mono text-[9px] uppercase tracking-wider text-muted-foreground sm:ml-0">
            no result
          </span>
        )}
        <button
          type="button"
          aria-expanded={showRaw}
          aria-label={`Toggle raw data for ${name}`}
          onClick={() => setShowRaw((o) => !o)}
          className={cn(
            "inline-flex min-h-6 shrink-0 cursor-pointer items-center gap-1 rounded-md border px-1.5 font-mono text-[9px] uppercase tracking-wider transition-colors motion-reduce:transition-none",
            showRaw ? "border-border bg-muted text-foreground" : "border-border/60 text-muted-foreground hover:bg-muted hover:text-foreground"
          )}
        >
          <Braces className="h-3 w-3" aria-hidden />
          raw
        </button>
      </div>
      <div className="mt-2 sm:pl-[22px]">
        <ToolBody name={event.tool_name} input={event.tool_input} rawResult={resultRaw} />
      </div>
      <div
        className={cn("grid transition-all duration-200 ease-out motion-reduce:transition-none", showRaw ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0")}
        aria-hidden={!showRaw}
      >
        <div className="overflow-hidden">
          <div className="mt-2 space-y-2.5 rounded-md border border-border/60 bg-muted/30 px-3 py-2 sm:ml-[22px]">
            <div>
              <div className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">Tool input</div>
              <pre className="mt-0.5 wrap-anywhere whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-foreground/70">{formatJSON(event.tool_input)}</pre>
            </div>
            {resultRaw !== undefined && (
              <div>
                <div className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">Raw result</div>
                <pre className="mt-0.5 wrap-anywhere whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-foreground/70">{prettyResult(resultRaw)}</pre>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function StatusBlock({ tone, title, body }: { tone: "error" | "muted"; title: string; body: string }) {
  return (
    <div className="mt-8 space-y-2">
      <h2 className={cn("text-xl font-semibold", tone === "error" ? "text-red" : "text-foreground")}>{title}</h2>
      <p className="text-sm text-muted-foreground">{body}</p>
    </div>
  );
}

function TranscriptSkeleton() {
  return (
    <div className="mt-6 space-y-3" aria-busy="true" aria-live="polite">
      <Skeleton className="h-24 w-full rounded-lg" />
      <Skeleton className="h-10 w-full rounded-lg" />
      <Skeleton className="h-10 w-3/4 rounded-lg" />
      <Skeleton className="h-24 w-full rounded-lg" />
    </div>
  );
}

/*
Published Claude list prices per million tokens, by model id, used ONLY to
estimate a session's cost from its captured token usage. The Messages API
does not return a dollar cost per request, so this is computed, not actual.
Keep in sync with https://platform.claude.com/docs/en/about-claude/pricing.
The agent caches with the 5-minute TTL, so cacheWrite uses the 5m rate.
*/
const MODEL_RATES: Record<string, { input: number; output: number; cacheRead: number; cacheWrite: number }> = {
  "claude-fable-5": { input: 10, output: 50, cacheRead: 1, cacheWrite: 12.5 },
  "claude-opus-4-8": { input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25 },
  "claude-opus-4-7": { input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25 },
  "claude-opus-4-6": { input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25 },
  "claude-sonnet-4-6": { input: 3, output: 15, cacheRead: 0.3, cacheWrite: 3.75 },
  "claude-haiku-4-5": { input: 1, output: 5, cacheRead: 0.1, cacheWrite: 1.25 },
};
// Web search bills $10 / 1,000 searches; web fetch and (with the web tools)
// code execution are free, so they add nothing to the estimate.
const WEB_SEARCH_USD = 0.01;

// formatUsd shows more precision for sub-dollar sessions so a $0.0042 run
// doesn't collapse to $0.00.
function formatUsd(n: number): string {
  if (n === 0) return "$0.00";
  if (n < 0.01) return `$${n.toFixed(4)}`;
  if (n < 1) return `$${n.toFixed(3)}`;
  return `$${n.toFixed(2)}`;
}

/*
UsageBreakdown renders the session's token spend as a line-item cost table:
each category with its quantity, unit rate, and estimated dollar cost, plus a
total. Token rows price per million tokens; the web rows price per request
(web fetch is free). When the model's list prices are unknown the money
columns are dropped and only the quantities are shown.
*/
function UsageBreakdown({ usage, model }: { usage: TranscriptUsage; model: string }) {
  const r = MODEL_RATES[model];
  const showCost = r !== undefined;

  type Row = { label: string; qty: string; rate: string; cost: number };
  const rows: Row[] = [
    { label: "Output", qty: formatTokens(usage.output_tokens), rate: r ? `$${r.output}/M` : "n/a", cost: r ? (usage.output_tokens * r.output) / 1_000_000 : 0 },
    { label: "Input (fresh)", qty: formatTokens(usage.input_tokens), rate: r ? `$${r.input}/M` : "n/a", cost: r ? (usage.input_tokens * r.input) / 1_000_000 : 0 },
    { label: "Cache read", qty: formatTokens(usage.cache_read_tokens), rate: r ? `$${r.cacheRead}/M` : "n/a", cost: r ? (usage.cache_read_tokens * r.cacheRead) / 1_000_000 : 0 },
    { label: "Cache write (5m)", qty: formatTokens(usage.cache_creation_tokens), rate: r ? `$${r.cacheWrite}/M` : "n/a", cost: r ? (usage.cache_creation_tokens * r.cacheWrite) / 1_000_000 : 0 },
  ];
  if (usage.web_search_requests > 0) {
    rows.push({ label: "Web searches", qty: String(usage.web_search_requests), rate: `$${WEB_SEARCH_USD.toFixed(2)}/ea`, cost: usage.web_search_requests * WEB_SEARCH_USD });
  }
  if (usage.web_fetch_requests > 0) {
    rows.push({ label: "Web fetches", qty: String(usage.web_fetch_requests), rate: "free", cost: 0 });
  }
  const total = rows.reduce((s, row) => s + row.cost, 0);

  return (
    <div className="mb-6">
      <div className="mb-2 font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
        {showCost ? "Session cost breakdown" : "Token usage"}
      </div>
      <div className="overflow-x-auto rounded-md border border-border/60">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-border/60 bg-muted/30 text-[10px] uppercase tracking-wider text-muted-foreground">
              <th className="px-3 py-2 text-left font-semibold">Line item</th>
              <th className="px-3 py-2 text-right font-semibold">Quantity</th>
              {showCost && <th className="px-3 py-2 text-right font-semibold">Rate</th>}
              {showCost && <th className="px-3 py-2 text-right font-semibold">Est. cost</th>}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.label} className="border-b border-border/40 last:border-0">
                <td className="px-3 py-2 text-foreground/80">{row.label}</td>
                <td className="px-3 py-2 text-right font-mono tabular-nums text-foreground/70">{row.qty}</td>
                {showCost && <td className="px-3 py-2 text-right font-mono tabular-nums text-muted-foreground">{row.rate}</td>}
                {showCost && <td className="px-3 py-2 text-right font-mono tabular-nums text-foreground/80">{formatUsd(row.cost)}</td>}
              </tr>
            ))}
          </tbody>
          {showCost && (
            <tfoot>
              <tr className="border-t border-border/60 bg-claude/5">
                <td className="px-3 py-2 font-semibold text-foreground/90" colSpan={3}>
                  Total (estimated)
                </td>
                <td className="px-3 py-2 text-right font-mono font-semibold tabular-nums text-claude">{formatUsd(total)}</td>
              </tr>
            </tfoot>
          )}
        </table>
      </div>
      {showCost && (
        <p className="mt-2 text-[10px] leading-relaxed text-muted-foreground/70">
          Estimated from token usage at list prices, not a billed amount. The API does not return a per-session cost.
        </p>
      )}
    </div>
  );
}

// formatDuration renders a millisecond span as "Xm Ys" (or "Ys" under a
// minute, "Xh Ym" over an hour). Used for the session wall-clock.
function formatDuration(ms: number): string {
  const totalSec = Math.round(ms / 1000);
  if (totalSec < 60) return `${totalSec}s`;
  const min = Math.floor(totalSec / 60);
  const sec = totalSec % 60;
  if (min < 60) return sec ? `${min}m ${sec}s` : `${min}m`;
  const hr = Math.floor(min / 60);
  const remMin = min % 60;
  return remMin ? `${hr}h ${remMin}m` : `${hr}h`;
}

// formatTokens renders a token count compactly: 948, 12.3k, 1.4M.
function formatTokens(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(n < 10_000 ? 1 : 0)}k`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}

// formatJSON pretty-prints a value (tool input). Falls back to String()
// for anything that can't be serialized.
function formatJSON(value: unknown): string {
  if (value === undefined || value === null) return "(no input)";
  return safeStringify(value, 2);
}

// prettyResult tries to parse a tool-result string as JSON and re-emit it
// indented; non-JSON results (web search prose, error strings) are shown
// as-is.
function prettyResult(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) return "(empty result)";
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return raw;
  }
}
