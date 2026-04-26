"use client";

import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { ExecutionBadge, matchesTrade } from "@/components/execution-badge";
import { Badge } from "@/components/ui/badge";
import { ClaudeLogo, OpenAILogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { Metric } from "@/components/ui/metric";
import { api } from "@/lib/api";
import { calcBreakeven, calcMaxLoss, calcMoneyness, sentimentColor, sentimentLabel } from "@/lib/calculations";
import { fmt, fmtMoney, fmtPctDec, fmtPnlInt } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade, Execution } from "@/types/trade";

type LoadState =
  | { kind: "loading" }
  | { kind: "found"; dt: DashboardTrade; resolvedDate: string; execution: Execution | null }
  | { kind: "not-found"; tried: string }
  | { kind: "error"; message: string };

export function TradeDetailPage({ symbol, date }: { symbol: string; date?: string }) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    setState({ kind: "loading" });
    api
      .getTrades(date)
      .then((data) => {
        if (cancelled) return;
        const dt = (data.trades ?? []).find((row) => row.trade.symbol.toUpperCase() === symbol.toUpperCase());
        if (!dt) {
          setState({ kind: "not-found", tried: data.date ?? date ?? "today" });
          return;
        }
        const execution = matchesTrade(data.execution, dt.trade) ? (data.execution ?? null) : null;
        setState({ kind: "found", dt, resolvedDate: data.date, execution });
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setState({ kind: "error", message: e instanceof Error ? e.message : "Failed to load trade" });
      });
    return () => {
      cancelled = true;
    };
  }, [symbol, date]);

  return (
    <div className="mx-auto min-w-0 max-w-[1100px] px-4 py-6 sm:px-7">
      <BackLink />
      {state.kind === "loading" && <LoadingPanel symbol={symbol} />}
      {state.kind === "error" && <Panel tone="error" title="Couldn't load that trade" body={state.message} />}
      {state.kind === "not-found" && (
        <Panel tone="muted" title={`No $${symbol} pick on ${state.tried}`} body="The dashboard only shows picks for trading days the system ran. Try a different date or head back to the dashboard." />
      )}
      {state.kind === "found" && <TradeDetailBody dt={state.dt} resolvedDate={state.resolvedDate} execution={state.execution} />}
    </div>
  );
}

function BackLink() {
  return (
    <Link href="/dashboard" className="mb-4 inline-flex min-h-9 items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground">
      <ArrowLeft className="h-4 w-4" />
      Back to dashboard
    </Link>
  );
}

function LoadingPanel({ symbol }: { symbol: string }) {
  return (
    <Card>
      <CardContent className="space-y-3 p-6">
        <div className="text-sm text-muted-foreground">Loading ${symbol}…</div>
        <div className="h-6 w-1/3 animate-pulse rounded bg-muted/60" />
        <div className="h-4 w-2/3 animate-pulse rounded bg-muted/60" />
      </CardContent>
    </Card>
  );
}

function Panel({ tone, title, body }: { tone: "error" | "muted"; title: string; body: string }) {
  return (
    <Card>
      <CardContent className="space-y-2 p-6">
        <h2 className={cn("text-lg font-semibold", tone === "error" ? "text-red" : "text-foreground")}>{title}</h2>
        <p className="text-sm text-muted-foreground">{body}</p>
      </CardContent>
    </Card>
  );
}

function TradeDetailBody({ dt, resolvedDate, execution }: { dt: DashboardTrade; resolvedDate: string; execution: Execution | null }) {
  const { trade, summary } = dt;
  const moneyness = calcMoneyness(trade);
  const breakeven = calcBreakeven(trade);
  const maxLoss = calcMaxLoss(trade);

  const pnl = summary ? (summary.closing_price - summary.entry_price) * 100 : 0;
  const pctChange = summary && summary.entry_price > 0 ? ((summary.closing_price - summary.entry_price) / summary.entry_price) * 100 : 0;
  const stockPctChange = summary && summary.stock_open > 0 ? ((summary.stock_close - summary.stock_open) / summary.stock_open) * 100 : 0;

  return (
    <div className="space-y-5">
      {execution && <ExecutionBadge execution={execution} variant="full" />}
      {/* Header: ticker + badges + price */}
      <Card>
        <CardContent className="space-y-4 p-5 sm:p-6">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="font-mono text-2xl font-bold tabular-nums text-foreground sm:text-3xl">${trade.symbol}</h1>
            <Badge variant="outline" className={cn(trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
              {trade.contract_type}
            </Badge>
            <Badge variant={moneyness.variant}>{moneyness.label}</Badge>
            <Badge variant="secondary">Rank #{trade.rank}</Badge>
            <Badge variant="secondary" className="text-xs">
              {trade.risk_level}
            </Badge>
            {summary && <Badge variant={pnl > 0 ? "default" : pnl < 0 ? "destructive" : "secondary"}>EOD {fmtPnlInt(pnl)}</Badge>}
            <span className="ml-auto text-xs text-muted-foreground">{resolvedDate}</span>
          </div>

          <div className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm sm:grid-cols-3 md:grid-cols-4">
            <Metric label="Strike" value={fmtMoney(trade.strike_price)} />
            <Metric label="Expiration" value={`${trade.expiration} (${trade.dte}d)`} />
            <Metric label="Entry" value={fmtMoney(trade.estimated_price)} />
            <Metric
              label="Target"
              value={<span className={cn("text-sm font-semibold tabular-nums", trade.contract_type === "CALL" ? "text-green" : "text-red")}>{fmtMoney(trade.target_price)}</span>}
            />
            <Metric label="Stop loss" value={fmtMoney(trade.stop_loss)} />
            <Metric label="Breakeven" value={fmtMoney(breakeven)} />
            <Metric label="Max loss" value={<span className="text-sm font-semibold tabular-nums text-red">{fmtPnlInt(-maxLoss)}</span>} />
            <Metric
              label="Sentiment"
              value={
                <span className={cn("text-sm font-semibold tabular-nums", sentimentColor(trade.sentiment_score))}>
                  {sentimentLabel(trade.sentiment_score)} ({fmt(trade.sentiment_score, 2)})
                </span>
              }
            />
            <Metric label="Mentions" value={String(trade.mention_count)} />
            <Metric label="Stock at entry" value={fmtMoney(trade.current_price)} />
          </div>

          {trade.catalyst && (
            <div className="rounded-md bg-amber-bg px-3 py-2 text-sm">
              <span className="font-semibold text-amber">Catalyst:</span> {trade.catalyst}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Dual-model rationale + cross verdict */}
      {(trade.gpt_rationale || trade.claude_rationale) && (
        <Card>
          <CardContent className="space-y-5 p-5 sm:p-6">
            <h2 className="text-base font-semibold">Independent rationales</h2>
            <div className="grid gap-5 lg:grid-cols-2">
              {trade.gpt_rationale && (
                <ModelBlock
                  Logo={OpenAILogo}
                  modelLabel="ChatGPT"
                  modelLabelClass="text-gpt"
                  score={trade.gpt_score}
                  rationale={trade.gpt_rationale}
                  verdict={trade.claude_verdict}
                  verdictLabel="Claude's verdict"
                  verdictAccent="border-claude/40 bg-claude/5"
                  verdictLabelClass="text-claude"
                  VerdictLogo={ClaudeLogo}
                />
              )}
              {trade.claude_rationale && (
                <ModelBlock
                  Logo={ClaudeLogo}
                  modelLabel="Claude"
                  modelLabelClass="text-claude"
                  score={trade.claude_score}
                  rationale={trade.claude_rationale}
                  verdict={trade.gpt_verdict}
                  verdictLabel="ChatGPT's verdict"
                  verdictAccent="border-gpt/40 bg-gpt/5"
                  verdictLabelClass="text-gpt"
                  VerdictLogo={OpenAILogo}
                />
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* EOD result if settled */}
      {summary && (
        <Card>
          <CardContent className="space-y-4 p-5 sm:p-6">
            <div className="flex flex-wrap items-baseline justify-between gap-3">
              <h2 className="text-base font-semibold">End-of-day result</h2>
              <span className={cn("text-2xl font-bold tabular-nums", pnl > 0 ? "text-green" : pnl < 0 ? "text-red" : "text-muted-foreground")}>
                {fmtPnlInt(pnl)}
                <span className="ml-2 text-sm font-medium text-muted-foreground">{fmtPctDec(pctChange)}</span>
              </span>
            </div>

            <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
              <Metric label="Contract entry" value={fmtMoney(summary.entry_price)} />
              <Metric
                label="Contract close"
                value={<span className={cn("text-sm font-semibold tabular-nums", pnl > 0 ? "text-green" : pnl < 0 ? "text-red" : "")}>{fmtMoney(summary.closing_price)}</span>}
              />
              <Metric label="Stock open" value={fmtMoney(summary.stock_open)} />
              <Metric
                label="Stock close"
                value={
                  <span className={cn("text-sm font-semibold tabular-nums", stockPctChange > 0 ? "text-green" : stockPctChange < 0 ? "text-red" : "")}>
                    {fmtMoney(summary.stock_close)}
                    <span className="ml-1 text-xs font-medium text-muted-foreground">({fmtPctDec(stockPctChange)})</span>
                  </span>
                }
              />
            </div>

            {summary.notes && <p className="rounded-md border bg-muted/30 p-3 text-sm text-muted-foreground">{summary.notes}</p>}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function ModelBlock({
  Logo,
  modelLabel,
  modelLabelClass,
  score,
  rationale,
  verdict,
  verdictLabel,
  verdictAccent,
  verdictLabelClass,
  VerdictLogo,
}: {
  Logo: React.ComponentType<{ className?: string }>;
  modelLabel: string;
  modelLabelClass: string;
  score: number;
  rationale: string;
  verdict: string;
  verdictLabel: string;
  verdictAccent: string;
  verdictLabelClass: string;
  VerdictLogo: React.ComponentType<{ className?: string }>;
}) {
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Logo className="h-5 w-5" />
        <span className={cn("text-sm font-semibold", modelLabelClass)}>{modelLabel}</span>
        {score > 0 && (
          <Badge variant="secondary" className="tabular-nums">
            {score}/10
          </Badge>
        )}
      </div>
      <p className="text-sm leading-relaxed text-muted-foreground">{rationale}</p>
      {verdict && (
        <div className={cn("rounded-md border-l-2 px-3 py-2 text-sm leading-relaxed", verdictAccent)}>
          <div className={cn("mb-1 flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-wider", verdictLabelClass)}>
            <VerdictLogo className="h-3 w-3" />
            {verdictLabel}
          </div>
          <p className="italic text-muted-foreground">{verdict}</p>
        </div>
      )}
    </div>
  );
}
