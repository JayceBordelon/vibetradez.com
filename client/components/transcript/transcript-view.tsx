"use client";

import { ArrowLeft, BookOpen, Brain, CheckCircle2, ChevronRight, CircleDollarSign, NotebookPen, XCircle } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { TranscriptEvent, TranscriptResponse, TranscriptUsage } from "@/types/trade";

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
    title: "Session reasoning",
    blurb: "Everything Claudia looked at and every move she made with the account today, tool call by tool call.",
    stage: "daily portfolio session",
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
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          {copy.title} for {prettyDate(date)}
        </h1>
        <Badge variant="secondary">{copy.stage}</Badge>
      </div>
      <p className="mt-2 text-sm text-muted-foreground">{copy.blurb}</p>

      {state.kind === "loading" && <TranscriptSkeleton />}
      {state.kind === "error" && (
        <StatusBlock tone="error" title="Couldn't load the transcript" body={state.message} />
      )}
      {state.kind === "ready" && (!state.data.available || (state.data.events ?? []).length === 0) && (
        <StatusBlock
          tone="muted"
          title="No transcript recorded"
          body={`No session activity was recorded for ${prettyDate(date)}. This happens on days the ${copy.stage} didn't run (weekends, holidays) or when a session ended without making any moves.`}
        />
      )}
      {state.kind === "ready" && state.data.available && (state.data.events ?? []).length > 0 && <TranscriptBody data={state.data} />}
    </div>
  );
}

function TranscriptBody({ data }: { data: TranscriptResponse }) {
  const events = data.events ?? [];
  const usage = data.usage;
  const durationMs = data.duration_ms ?? 0;
  return (
    <div className="mt-6">
      {data.model && (
        <div className="mb-5 flex flex-wrap items-center gap-x-4 gap-y-1 border-y border-border/40 py-3 text-xs text-muted-foreground">
          <span>
            Model <span className="font-mono text-foreground/80">{data.model}</span>
          </span>
          {data.created_at && <span>Captured {new Date(data.created_at).toLocaleString()}</span>}
          {durationMs > 0 && <span>Ran for {formatDuration(durationMs)}</span>}
          <span>{events.length} events</span>
          {usage && usage.rounds > 0 && <span>{usage.rounds} rounds</span>}
        </div>
      )}
      {usage && usage.rounds > 0 && <UsageBreakdown usage={usage} />}
      <ol className="space-y-6">
        {(() => {
          // Fold each tool_result into its matching tool_use so a call and
          // its outcome render as one element, not two stacked headers. The
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
          return events
            .map((ev, i) => ({ ev, i }))
            .filter(({ ev }) => ev.type !== "tool_result")
            .map(({ ev, i }) => (
              <li key={`${ev.round}-${ev.type}-${ev.tool_use_id ?? i}`}>
                <EventBlock event={ev} resultPayload={resultFor.get(i)} />
              </li>
            ));
        })()}
      </ol>
      <p className="mt-8 text-[11px] leading-relaxed text-muted-foreground">
        This is the model's own narration and tool activity, captured verbatim from the run. This is a single public
        account, so balances are shown openly rather than hidden. Not financial advice.
      </p>
    </div>
  );
}

// Tools fall into three groups, each with its own icon in the transcript:
// EXECUTION tools move money (or record the explicit no-op), DOCUMENTATION
// tools only write to the record, and everything else is a read-only
// reading tool.
const EXECUTION_TOOLS = new Set(["buy_equity", "sell_equity", "buy_option", "sell_option", "cancel_order", "hold"]);
const DOCUMENTATION_TOOLS = new Set(["write_summary"]);

type ToolGroup = "execution" | "documentation" | "reading";

function toolGroup(name?: string): ToolGroup {
  if (name && EXECUTION_TOOLS.has(name)) return "execution";
  if (name && DOCUMENTATION_TOOLS.has(name)) return "documentation";
  return "reading";
}

function toolIcon(group: ToolGroup) {
  return group === "execution" ? CircleDollarSign : group === "documentation" ? NotebookPen : BookOpen;
}

function toolEyebrow(group: ToolGroup): string {
  return group === "execution" ? "Execution tool called" : group === "documentation" ? "Documentation tool called" : "Reading tool called";
}

// resultStatus reads a tool result and classifies it: an {"error": ...}
// payload is an error, an {"ok": true} or any other returned data is a
// success. Undefined when there is no recorded result.
function resultStatus(raw?: string): "success" | "error" | undefined {
  if (raw === undefined) return undefined;
  const trimmed = raw.trim();
  if (!trimmed) return "success";
  try {
    const o = JSON.parse(trimmed);
    if (o && typeof o === "object" && !Array.isArray(o)) {
      if ("error" in o) return "error";
    }
  } catch {
    // non-JSON result (e.g. web prose) means the tool returned something.
  }
  return "success";
}

// toolAccentClass colors a tool by intent: buys green, sells red, holds the
// brand/primary purple, documentation clay. Reading tools stay neutral.
function toolAccentClass(name?: string): string {
  if (!name) return "text-foreground/90";
  if (name.startsWith("buy_")) return "text-green";
  if (name.startsWith("sell_")) return "text-red";
  if (name === "cancel_order") return "text-amber";
  if (name === "hold") return "text-muted-foreground";
  if (DOCUMENTATION_TOOLS.has(name)) return "text-claude";
  return "text-foreground/90";
}

function EventBlock({ event, resultPayload }: { event: TranscriptEvent; resultPayload?: string }) {
  switch (event.type) {
    case "text":
      return (
        <div className="border-l-2 border-claude/40 pl-4">
          <div className="mb-1.5 flex items-center gap-2">
            <ClaudeLogo className="h-4 w-4" />
            <span className="text-xs font-semibold uppercase tracking-wider text-claude">Narration</span>
          </div>
          <p className="whitespace-pre-wrap text-[15px] leading-relaxed text-foreground/90">{event.text}</p>
        </div>
      );
    case "thinking":
      return (
        <div className="border-l-2 border-claude/40 pl-4">
          <div className="mb-1.5 flex items-center gap-2">
            <Brain className="h-3.5 w-3.5 text-claude" />
            <span className="text-xs font-semibold uppercase tracking-wider text-claude">Extended thinking</span>
          </div>
          <p className="whitespace-pre-wrap text-sm italic leading-relaxed text-foreground/80">{event.text}</p>
        </div>
      );
    case "tool_use": {
      const group = toolGroup(event.tool_name);
      const ToolIcon = toolIcon(group);
      return (
        <ToolDetails
          icon={<ToolIcon className="h-3.5 w-3.5" />}
          eyebrow={toolEyebrow(group)}
          name={event.tool_name ?? "tool"}
          accentClass={toolAccentClass(event.tool_name)}
          payload={formatJSON(event.tool_input)}
          status={resultStatus(resultPayload)}
          result={resultPayload !== undefined ? prettyResult(resultPayload) : undefined}
        />
      );
    }
    default:
      return null;
  }
}

/*
ToolDetails is a collapsible card for a tool call's input + result. Collapsed
by default (a session can run dozens of calls); the body expands and collapses
with a smooth height + fade animation via the grid-rows 0fr→1fr trick, so it
animates cleanly without measuring heights.
*/
function ToolDetails({
  icon,
  eyebrow,
  name,
  payload,
  accentClass = "text-foreground/90",
  status,
  result,
}: {
  icon: React.ReactNode;
  eyebrow: string;
  name: string;
  payload: string;
  accentClass?: string;
  status?: "success" | "error";
  result?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="flex w-full cursor-pointer items-start gap-2.5 py-1 text-left"
      >
        <ChevronRight className={cn("mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform duration-200", open && "rotate-90")} />
        <span className={cn("mt-px shrink-0", accentClass)}>{icon}</span>
        <span className="min-w-0 flex-1 leading-tight">
          <span className="block text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{eyebrow}</span>
          <span className={cn("mt-0.5 block font-mono text-sm font-medium", accentClass)}>{name}</span>
        </span>
        {status && (
          <span className={cn("inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider", status === "success" ? "bg-green-bg text-green" : "bg-red-bg text-red")}>
            {status === "success" ? <CheckCircle2 className="h-3 w-3" /> : <XCircle className="h-3 w-3" />}
            {status}
          </span>
        )}
      </button>
      <div className={cn("grid transition-all duration-200 ease-out", open ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0")}>
        <div className="overflow-hidden">
          <div className="mt-1.5 ml-[26px] space-y-2.5">
            <div className="border-l-2 border-border/50 pl-4">
              <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Payload</div>
              <pre className="mt-0.5 whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-foreground/70">{payload}</pre>
            </div>
            {result && (
              <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2">
                <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Returned</div>
                <pre className="mt-0.5 whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-foreground/70">{result}</pre>
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
UsageBreakdown shows the session's token spend per category plus the
server-tool request counts. Output and the two cache figures are the ones
that move cost; input is the fresh (uncached) input billed each round.
*/
function UsageBreakdown({ usage }: { usage: TranscriptUsage }) {
  const stats: { label: string; value: string }[] = [
    { label: "Output tokens", value: formatTokens(usage.output_tokens) },
    { label: "Input (fresh)", value: formatTokens(usage.input_tokens) },
    { label: "Cache read", value: formatTokens(usage.cache_read_tokens) },
    { label: "Cache write", value: formatTokens(usage.cache_creation_tokens) },
  ];
  if (usage.web_search_requests > 0) stats.push({ label: "Web searches", value: String(usage.web_search_requests) });
  if (usage.web_fetch_requests > 0) stats.push({ label: "Web fetches", value: String(usage.web_fetch_requests) });

  return (
    <div className="mb-6">
      <div className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Token usage</div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-6">
        {stats.map((s) => (
          <div key={s.label} className="rounded-md border border-border/60 bg-muted/30 px-3 py-2">
            <div className="font-mono text-sm font-medium text-foreground/90">{s.value}</div>
            <div className="mt-0.5 text-[10px] uppercase tracking-wider text-muted-foreground">{s.label}</div>
          </div>
        ))}
      </div>
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
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
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
