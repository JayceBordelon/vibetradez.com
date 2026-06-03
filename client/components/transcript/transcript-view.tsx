"use client";

import { ArrowLeft, ChevronRight, Terminal, Wrench } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { TranscriptEvent, TranscriptResponse } from "@/types/trade";

type Kind = "selection" | "execution";

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
};

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
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{copy.title}</h1>
        <Badge variant="secondary">{copy.stage}</Badge>
        <span className="ml-auto font-mono text-xs text-muted-foreground">{date}</span>
      </div>
      <p className="mt-2 text-sm text-muted-foreground">{copy.blurb}</p>

      {state.kind === "loading" && <TranscriptSkeleton />}
      {state.kind === "error" && (
        <StatusBlock tone="error" title="Couldn't load the transcript" body={state.message} />
      )}
      {state.kind === "ready" && !state.data.available && (
        <StatusBlock
          tone="muted"
          title="No transcript for this day"
          body={`The ${copy.stage} conversation wasn't captured for ${date}. This happens for days the system didn't run (weekends, holidays, or before this feature shipped).`}
        />
      )}
      {state.kind === "ready" && state.data.available && <TranscriptBody data={state.data} />}
    </div>
  );
}

function TranscriptBody({ data }: { data: TranscriptResponse }) {
  return (
    <div className="mt-6">
      {data.model && (
        <div className="mb-5 flex flex-wrap items-center gap-x-4 gap-y-1 border-y border-border/40 py-3 text-xs text-muted-foreground">
          <span>
            Model <span className="font-mono text-foreground/80">{data.model}</span>
          </span>
          {data.created_at && <span>Captured {new Date(data.created_at).toLocaleString()}</span>}
          <span>{data.events.length} events</span>
        </div>
      )}
      <ol className="space-y-3">
        {data.events.map((ev, i) => (
          <li key={`${ev.round}-${ev.type}-${ev.tool_use_id ?? i}`}>
            <EventBlock event={ev} />
          </li>
        ))}
      </ol>
      <p className="mt-8 text-[11px] leading-relaxed text-muted-foreground">
        This is the model's own narration and tool activity, captured verbatim from the run. Account balances are
        redacted. Not financial advice.
      </p>
    </div>
  );
}

function EventBlock({ event }: { event: TranscriptEvent }) {
  switch (event.type) {
    case "text":
      return (
        <div className="rounded-lg border border-claude-border/40 bg-claude-light px-5 py-4">
          <div className="mb-2 flex items-center gap-2">
            <ClaudeLogo className="h-4 w-4" />
            <span className="text-xs font-semibold uppercase tracking-wider text-claude">Claudia</span>
          </div>
          <p className="whitespace-pre-wrap text-[15px] leading-relaxed text-foreground/90">{event.text}</p>
        </div>
      );
    case "thinking":
      return (
        <div className="rounded-lg border border-border/50 bg-muted/30 px-5 py-4">
          <div className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">Thinking</div>
          <p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">{event.text}</p>
        </div>
      );
    case "tool_use":
      return (
        <ToolDetails
          icon={<Wrench className="h-3.5 w-3.5" />}
          eyebrow="Tool call"
          name={event.tool_name ?? "tool"}
          payload={formatJSON(event.tool_input)}
          openByDefault
        />
      );
    case "tool_result":
      return (
        <ToolDetails
          icon={<Terminal className="h-3.5 w-3.5" />}
          eyebrow="Tool result"
          name={event.tool_name ?? "tool"}
          payload={prettyResult(event.tool_result ?? "")}
        />
      );
    default:
      return null;
  }
}

/*
ToolDetails is a collapsible <details> card for tool input/output.
Tool calls open by default (the args are short + interesting); tool
results stay collapsed (often large JSON chains) and expand on click.
Uses native <details> so it works without JS hydration.
*/
function ToolDetails({
  icon,
  eyebrow,
  name,
  payload,
  openByDefault = false,
}: {
  icon: React.ReactNode;
  eyebrow: string;
  name: string;
  payload: string;
  openByDefault?: boolean;
}) {
  return (
    <details open={openByDefault} className="group rounded-lg border border-border/60 bg-card/40">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-4 py-2.5 text-sm">
        <ChevronRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform group-open:rotate-90" />
        <span className="flex items-center gap-1.5 text-muted-foreground">{icon}</span>
        <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{eyebrow}</span>
        <span className="font-mono text-foreground/90">{name}</span>
      </summary>
      <pre className="overflow-x-auto border-t border-border/50 px-4 py-3 font-mono text-xs leading-relaxed text-foreground/80">
        {payload}
      </pre>
    </details>
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
