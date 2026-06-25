"use client";

import { ArrowLeft, ArrowRight, BookOpen, Braces, Brain, CheckCircle2, CircleDollarSign, Download, Globe, NotebookPen, SquareTerminal, XCircle } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { TerminalLoader } from "@/components/ui/terminal-loader";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { TranscriptEvent, TranscriptResponse, TranscriptUsage } from "@/types/trade";

import { CollapsibleOutput } from "./collapsible-output";
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
      <div className="flex flex-wrap items-center gap-2.5">
        <ClaudeLogo className="h-5 w-5" />
        <h1 className="font-display text-xl font-bold tracking-tight sm:text-2xl">
          {copy.title} for {prettyDate(date)}
        </h1>
        <Badge variant="secondary">{copy.stage}</Badge>
      </div>
      <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{copy.blurb}</p>

      {kind === "portfolio" && <SessionPager date={date} />}

      {state.kind === "loading" && <TranscriptLoading />}
      {state.kind === "error" && <StatusBlock tone="error" title="Couldn't load the transcript" body={state.message} />}
      {state.kind === "ready" && (!state.data.available || (state.data.events ?? []).length === 0) && <EmptyTranscript date={date} kind={kind} />}
      {state.kind === "ready" && state.data.available && (state.data.events ?? []).length > 0 && <TranscriptBody data={state.data} date={date} />}
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
    <div className="mt-5 flex items-center justify-between gap-3 border-y border-border/60 py-1.5 text-[13px]">
      <Link href={`/transcripts/${prev}`} className="inline-flex min-h-9 items-center gap-1.5 rounded-lg px-2 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground sm:min-h-0">
        <ArrowLeft className="h-4 w-4" aria-hidden />
        {prettyDate(prev)}
      </Link>
      {showNext ? (
        <Link href={`/transcripts/${next}`} className="inline-flex min-h-9 items-center gap-1.5 rounded-lg px-2 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground sm:min-h-0">
          {prettyDate(next)}
          <ArrowRight className="h-4 w-4" aria-hidden />
        </Link>
      ) : (
        <span className="inline-flex min-h-9 items-center px-2 text-muted-foreground/50 sm:min-h-0">Latest session</span>
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

// summaryFor extracts a session's write_summary (synopsis + the action items
// it left for next time) from that session's events, if it wrote one.
function summaryFor(events: TranscriptEvent[]): { synopsis: string; actions: string } | null {
  for (const e of events) {
    if (e.type === "tool_use" && e.tool_name === "write_summary") {
      const input = (e.tool_input ?? {}) as { synopsis?: unknown; action_items?: unknown };
      const synopsis = summaryText(input.synopsis);
      const actions = summaryText(input.action_items);
      if (synopsis || actions) return { synopsis, actions };
    }
  }
  return null;
}

/*
A day's transcript is ONE merged event stream across the open / midday /
pre-close sessions, delimited by session_marker events. splitSessions
partitions it into one group per session (each marker plus the events up to
the next marker) so the page can show one session at a time behind a toggle.
Any preamble before the first marker attaches to the first group; a transcript
with no markers becomes a single anonymous group (the toggle then hides).
*/
type SessionGroup = {
  label: string;
  fullLabel: string;
  anchor: string;
  events: TranscriptEvent[];
  usage?: TranscriptUsage;
  durationMs: number;
};

function splitSessions(events: TranscriptEvent[]): SessionGroup[] {
  const groups: SessionGroup[] = [];
  const preamble: TranscriptEvent[] = [];
  let current: SessionGroup | null = null;
  for (const e of events) {
    if (e.type === "session_marker") {
      current = {
        label: sessionShort(e.text ?? ""),
        fullLabel: (e.text ?? "Session").trim() || "Session",
        anchor: sessionAnchor(e.text ?? ""),
        events: [],
        usage: e.usage as TranscriptUsage | undefined,
        durationMs: e.duration_ms ?? 0,
      };
      groups.push(current);
      continue;
    }
    if (current) current.events.push(e);
    else preamble.push(e);
  }
  if (groups.length === 0) {
    return [{ label: "Session", fullLabel: "Session", anchor: "session", events: preamble, durationMs: 0 }];
  }
  if (preamble.length > 0) groups[0].events = [...preamble, ...groups[0].events];
  return groups;
}

// SessionTabs is the within-day toggle: pick which session (open / midday /
// pre-close) to show. Underline-active to match the de-carded nav, no boxy
// container. The caller hides it when there is only one session.
function SessionTabs({ sessions, active, onChange }: { sessions: SessionGroup[]; active: number; onChange: (i: number) => void }) {
  return (
    <div role="tablist" aria-label="Session" className="inline-flex flex-wrap items-center gap-x-5 gap-y-1">
      {sessions.map((s, i) => (
        <button
          key={s.anchor}
          type="button"
          role="tab"
          aria-selected={i === active}
          onClick={() => onChange(i)}
          className={cn("relative min-h-9 pb-1.5 text-sm font-medium transition-colors sm:min-h-0", i === active ? "text-foreground" : "text-muted-foreground hover:text-foreground")}
        >
          {s.label}
          {i === active && <span aria-hidden className="absolute inset-x-0 -bottom-px h-0.5 rounded-full bg-primary" />}
        </button>
      ))}
    </div>
  );
}

// SessionSummaryBlock surfaces the SELECTED session's write_summary above its
// ledger — the conclusion leading the entry.
function SessionSummaryBlock({ events }: { events: TranscriptEvent[] }) {
  const summary = summaryFor(events);
  if (!summary) return null;
  return (
    <section aria-label="Session summary" className="mb-6 rounded-r-lg border-l-2 border-claude/40 bg-claude-light px-4 py-4 sm:px-5">
      <div className="mb-2.5 flex items-center gap-2">
        <NotebookPen className="h-4 w-4 text-claude" aria-hidden />
        <h2 className="font-mono text-[11px] font-semibold uppercase tracking-[0.16em] text-claude">Session summary</h2>
      </div>
      {summary.synopsis && <p className="wrap-anywhere whitespace-pre-wrap text-[15px] leading-relaxed text-foreground/90">{summary.synopsis}</p>}
      {summary.actions && (
        <div className="mt-2.5">
          <p className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">Action items for next session</p>
          <p className="mt-1 wrap-anywhere whitespace-pre-wrap text-[13px] leading-relaxed text-muted-foreground">{summary.actions}</p>
        </div>
      )}
    </section>
  );
}

// DownloadButton exports the WHOLE day's transcript (all sessions) as a
// portable Markdown file, built client-side from the already-loaded data.
function DownloadButton({ data, date }: { data: TranscriptResponse; date: string }) {
  const onClick = () => {
    const blob = new Blob([buildMarkdown(data, date)], { type: "text/markdown;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `vibetradez-sessions-${date}.md`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex min-h-9 shrink-0 items-center gap-1.5 rounded-lg border border-border/60 px-3 text-[13px] font-medium text-muted-foreground transition-colors hover:border-primary/40 hover:text-foreground sm:min-h-0"
    >
      <Download className="h-4 w-4" aria-hidden />
      Download
    </button>
  );
}

// buildMarkdown renders the full day (every session) as a Markdown document:
// day meta, then per session its summary and every event (narration,
// thinking, tool calls with input + result) in order.
function buildMarkdown(data: TranscriptResponse, date: string): string {
  const out: string[] = [`# VibeTradez — session transcript for ${prettyDate(date)}`, ""];
  if (data.model) out.push(`- **Model:** ${data.model}`);
  if (data.created_at) out.push(`- **Captured:** ${new Date(data.created_at).toLocaleString()}`);
  out.push("");
  for (const s of splitSessions(data.events ?? [])) {
    out.push(`## ${s.fullLabel}`, "");
    const summary = summaryFor(s.events);
    if (summary?.synopsis) out.push(`**Synopsis:** ${summary.synopsis}`, "");
    if (summary?.actions) out.push(`**Action items:** ${summary.actions}`, "");
    for (const e of s.events) {
      if (e.type === "text") {
        const t = parseStanceText(e.text ?? "") ?? e.text ?? "";
        out.push(
          t
            .split("\n")
            .map((l) => `> ${l}`)
            .join("\n"),
          "",
        );
      } else if (e.type === "thinking") {
        out.push(`_Thinking:_ ${(e.text ?? "").trim()}`, "");
      } else if (e.type === "tool_use") {
        out.push(`### \`${e.tool_name ?? "tool"}\``, "```json", safeStringify(e.tool_input ?? {}, 2), "```", "");
      } else if (e.type === "tool_result") {
        out.push("```", prettyResult(e.tool_result ?? ""), "```", "");
      }
    }
  }
  out.push("---", "Exported from vibetradez.com — the model's own narration and tool activity, captured verbatim. Not financial advice.");
  return out.join("\n");
}

function TranscriptBody({ data, date }: { data: TranscriptResponse; date: string }) {
  const events = data.events ?? [];
  const sessions = useMemo(() => splitSessions(events), [events]);
  const multi = sessions.length > 1;
  // Default to the latest session (the freshest state); the toggle steps back
  // through the day. null = "not yet chosen", so the default tracks new data
  // without a first-render flash.
  const [active, setActive] = useState<number | null>(null);
  const idx = Math.min(Math.max(active ?? sessions.length - 1, 0), sessions.length - 1);
  const session = sessions[idx];

  return (
    <div className="mt-6">
      {/* Within-day toggle + download: one session shows at a time. The
          toggle only appears on a multi-session day; the Download (whole day)
          is always offered, right-aligned. */}
      <div className="mb-6 flex flex-wrap items-center gap-x-5 gap-y-3 border-b border-border/60 pb-3">
        {multi && <SessionTabs sessions={sessions} active={idx} onChange={setActive} />}
        <div className="ml-auto">
          <DownloadButton data={data} date={date} />
        </div>
      </div>

      <SessionMeta data={data} session={session} />
      <SessionSummaryBlock events={session.events} />
      {session.usage && session.usage.rounds > 0 && <UsageBreakdown usage={session.usage} model={data.model} label="Cost for this session" />}
      <Ledger events={session.events} />

      <p className="mt-8 text-[11px] leading-relaxed text-muted-foreground">
        This is the model's own narration and tool activity, captured verbatim from the run. This is a single public
        account, so balances are shown openly rather than hidden. Not financial advice.
      </p>
    </div>
  );
}

// SessionMeta is the compact meta row for the active session: day-level model
// and capture time, then the session's wall-clock, entry count, and rounds.
function SessionMeta({ data, session }: { data: TranscriptResponse; session: SessionGroup }) {
  if (!data.model) return null;
  const entries = session.events.filter((e) => e.type !== "tool_result").length;
  const rounds = session.usage?.rounds ?? 0;
  return (
    <div className="mb-6 grid grid-cols-2 gap-x-6 gap-y-2.5 border-b border-border/60 pb-3.5 sm:grid-cols-5">
      <MetaCell label="Model" value={data.model} />
      {data.created_at && <MetaCell label="Captured" value={new Date(data.created_at).toLocaleString()} />}
      {session.durationMs > 0 && <MetaCell label="Wall clock" value={formatDuration(session.durationMs)} />}
      <MetaCell label="Entries" value={String(entries)} />
      {rounds > 0 && <MetaCell label="Rounds" value={String(rounds)} />}
    </div>
  );
}

/*
Ledger renders ONE session's events as the desk-blotter <ol>: each tool_use
folded with its following tool_result into a single entry (matched by
tool_use_id when both carry one, else by adjacency so seeded / id-less
transcripts still pair), with a thin labeled rule at each round boundary. A
single session carries no session_marker events, so none of the merged-day
divider handling is needed here.
*/
function Ledger({ events }: { events: TranscriptEvent[] }) {
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
  // Rounds are 0-indexed in the event stream; display 1-indexed so the labels
  // agree with the header's "N rounds" count.
  let lastRound: number | undefined;
  return (
    <ol className="space-y-5">
      {events
        .map((ev, i) => ({ ev, i }))
        .filter(({ ev }) => ev.type !== "tool_result")
        .map(({ ev, i }) => {
          const newRound = ev.round !== lastRound && lastRound !== undefined;
          lastRound = ev.round;
          return (
            <li key={`${ev.round}-${ev.type}-${ev.tool_use_id ?? i}`}>
              {newRound && (
                <div className="mb-5 flex items-center gap-3" aria-hidden>
                  <span className="h-px flex-1 bg-border" />
                  <span className="font-mono text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">Round {ev.round + 1}</span>
                  <span className="h-px flex-1 bg-border" />
                </div>
              )}
              <EventBlock event={ev} resultPayload={resultFor.get(i)} />
            </li>
          );
        })}
    </ol>
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
          <CollapsibleOutput>
            <p className="whitespace-pre-wrap text-[13px] italic leading-relaxed text-muted-foreground">{event.text}</p>
          </CollapsibleOutput>
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
      <CollapsibleOutput className="mt-2 sm:pl-[22px]">
        <ToolBody name={event.tool_name} input={event.tool_input} rawResult={resultRaw} />
      </CollapsibleOutput>
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

function TranscriptLoading() {
  return (
    <TerminalLoader
      className="mt-6"
      minHeightClass="min-h-[44vh]"
      command="Loading the session"
      lines={["Opening the session ledger", "Replaying the day's tool calls", "Reconciling fills against the broker"]}
    />
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
function UsageBreakdown({ usage, model, label }: { usage: TranscriptUsage; model: string; label?: string }) {
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
        {label ?? (showCost ? "Session cost breakdown" : "Token usage")}
      </div>
      <div className="overflow-x-auto border-t border-border/60">
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
