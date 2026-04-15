"use client";

import { Sparkle } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { PageToolbar } from "@/components/layout/page-toolbar";
import { Section } from "@/components/layout/section";
import { Badge } from "@/components/ui/badge";
import { ClaudeLogo, OpenAILogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { Metric } from "@/components/ui/metric";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { api } from "@/lib/api";
import { fmtPctDec, fmtPnlInt, percentHueColor, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { ModelComparisonResponse, ModelStats } from "@/types/trade";

type Range = "week" | "month" | "year" | "all";

const RANGE_OPTIONS: { value: Range; label: string }[] = [
  { value: "week", label: "Week" },
  { value: "month", label: "Month" },
  { value: "year", label: "Year" },
  { value: "all", label: "All time" },
];

export function ModelComparisonShell() {
  const [range, setRange] = useState<Range>("all");
  const [data, setData] = useState<ModelComparisonResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    api
      .getModelComparison(range)
      .then((d) => {
        if (cancelled) return;
        setData(d);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : "Failed to load");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [range]);

  const chartSeries = useMemo(() => {
    // Defensive: every data.<side> field is optional in the wire
    // shape (the server returns an empty model stats object when
    // there are no trades yet, and a future API change could drop
    // a field). Coalesce missing arrays to [] so this never throws.
    if (!data?.openai || !data?.anthropic || !data?.combined) return [];
    const merged = new Map<string, { date: string; openai: number; anthropic: number; combined: number }>();
    for (const p of data.openai.cumulative_pnl ?? []) {
      merged.set(p.date, {
        date: p.date,
        openai: p.pnl,
        anthropic: 0,
        combined: 0,
      });
    }
    for (const p of data.anthropic.cumulative_pnl ?? []) {
      const row = merged.get(p.date) ?? {
        date: p.date,
        openai: 0,
        anthropic: 0,
        combined: 0,
      };
      row.anthropic = p.pnl;
      merged.set(p.date, row);
    }
    for (const p of data.combined.cumulative_pnl ?? []) {
      const row = merged.get(p.date) ?? {
        date: p.date,
        openai: 0,
        anthropic: 0,
        combined: 0,
      };
      row.combined = p.pnl;
      merged.set(p.date, row);
    }
    return Array.from(merged.values()).sort((a, b) => a.date.localeCompare(b.date));
  }, [data]);

  const hasData = !!data && !!data.openai && !!data.anthropic && (data.openai.trades_evaluated > 0 || data.anthropic.trades_evaluated > 0);

  const winner = useMemo(() => {
    if (!hasData || !data) return null;
    const a = data.openai.total_pnl;
    const b = data.anthropic.total_pnl;
    if (a === b) return "tie" as const;
    return a > b ? "openai" : "anthropic";
  }, [data, hasData]);

  return (
    <div className="mx-auto max-w-[1200px]">
      <PageToolbar
        leftControls={
          <Tabs value={range} onValueChange={(v) => setRange(v as Range)}>
            <TabsList className="h-8 gap-1 p-1">
              {RANGE_OPTIONS.map((opt) => (
                <TabsTrigger key={opt.value} value={opt.value} className="h-6 px-3 text-xs font-semibold">
                  {opt.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        }
      />

      <div className="px-4 sm:px-7">
        {loading && !data && (
          <Section>
            <p className="py-12 text-center text-sm text-muted-foreground">Loading model comparison…</p>
          </Section>
        )}

        {error && (
          <Section>
            <p className="py-12 text-center text-sm text-red">{error}</p>
          </Section>
        )}

        {data && !hasData && (
          <Section>
            <p className="py-12 text-center text-sm text-muted-foreground">No trade history yet. The comparison will populate once the morning cron has produced its first day of picks.</p>
          </Section>
        )}

        {data && hasData && (
          <>
            <Section title="Head-to-head" subtitle={`Top ${data.top_n} picks per day, replayed under each model's ranking`}>
              <Card className="overflow-hidden p-0">
                <CardContent className="p-0">
                  <div className="grid grid-cols-[1fr_auto_1fr] items-stretch">
                    <SideStat Logo={OpenAILogo} model={data.openai.model} pnl={data.openai.total_pnl} trades={data.openai.trades_evaluated} align="left" leading={winner === "openai"} />
                    <div className="flex flex-col items-center justify-center border-x px-3 py-4 sm:px-5">
                      <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Agree</span>
                      <span className="mt-1 text-lg font-semibold tabular-nums sm:text-xl" style={{ color: percentHueColor(data.agreement_rate * 100) }}>
                        {Math.round(data.agreement_rate * 100)}%
                      </span>
                      <span className="mt-0.5 whitespace-nowrap text-[10px] text-muted-foreground">n={data.total_dual_scored}</span>
                    </div>
                    <SideStat Logo={ClaudeLogo} model={data.anthropic.model} pnl={data.anthropic.total_pnl} trades={data.anthropic.trades_evaluated} align="right" leading={winner === "anthropic"} />
                  </div>
                  <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-t bg-muted/30 px-4 py-2 text-xs">
                    <div className="flex items-center gap-2">
                      <Sparkle className="h-3.5 w-3.5 text-amber" />
                      <span className="font-semibold uppercase tracking-wider text-muted-foreground">Combined</span>
                      <span className={cn("font-semibold tabular-nums", pnlColor(data.combined.total_pnl))}>{fmtPnlInt(data.combined.total_pnl)}</span>
                      <span className="text-muted-foreground">· {data.combined.trades_evaluated} trades</span>
                    </div>
                    {winner && winner !== "tie" && (
                      <div className="flex items-center gap-1.5">
                        <span className="text-muted-foreground">Lead:</span>
                        <span className={cn("font-semibold tabular-nums", pnlColor(Math.abs(data.openai.total_pnl - data.anthropic.total_pnl)))}>
                          +{fmtPnlInt(Math.abs(data.openai.total_pnl - data.anthropic.total_pnl))}
                        </span>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>
            </Section>

            <Section title="Cumulative P&L">
              <Card>
                <CardContent className="p-4">
                  <div className="h-[320px]">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={chartSeries}>
                        <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
                        <XAxis dataKey="date" stroke="var(--chart-text)" tick={{ fontSize: 11 }} />
                        <YAxis stroke="var(--chart-text)" tick={{ fontSize: 11 }} tickFormatter={(v) => `$${Math.round(v)}`} />
                        <Tooltip
                          formatter={(v) => fmtPnlInt(Number(v))}
                          contentStyle={{
                            background: "var(--card)",
                            border: "1px solid var(--border)",
                            borderRadius: 8,
                            fontSize: 12,
                          }}
                        />
                        <Legend wrapperStyle={{ fontSize: 12 }} />
                        <Line type="monotone" dataKey="openai" name={data.openai.model} stroke="var(--gpt)" strokeWidth={2} dot={false} />
                        <Line type="monotone" dataKey="anthropic" name={data.anthropic.model} stroke="var(--claude)" strokeWidth={2} dot={false} />
                        <Line type="monotone" dataKey="combined" name="combined" stroke="var(--amber)" strokeWidth={2} strokeDasharray="4 2" dot={false} />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                </CardContent>
              </Card>
            </Section>

            <Section title="Side-by-side">
              <div className="grid gap-4 lg:grid-cols-2">
                <ModelCard label="OpenAI" stats={data.openai} Logo={OpenAILogo} />
                <ModelCard label="Anthropic" stats={data.anthropic} Logo={ClaudeLogo} />
              </div>
            </Section>
          </>
        )}
      </div>
    </div>
  );
}

function SideStat({
  Logo,
  model,
  pnl,
  trades,
  align,
  leading,
}: {
  Logo: React.ComponentType<{ className?: string }>;
  model: string;
  pnl: number;
  trades: number;
  align: "left" | "right";
  leading: boolean;
}) {
  return (
    <div className={cn("relative flex items-center gap-3 p-4 sm:p-5", align === "right" && "flex-row-reverse text-right")}>
      {leading && <span className="absolute top-2 right-2 text-[9px] font-bold uppercase tracking-wider text-amber sm:top-3 sm:right-3">Lead</span>}
      <Logo className="h-8 w-8 shrink-0 sm:h-10 sm:w-10" />
      <div className={cn("flex min-w-0 flex-col", align === "right" && "items-end")}>
        <span className="max-w-[120px] truncate font-mono text-[11px] font-medium text-muted-foreground sm:max-w-[200px]">{model}</span>
        <span className={cn("mt-0.5 text-xl font-semibold tabular-nums sm:text-3xl", pnlColor(pnl))}>{fmtPnlInt(pnl)}</span>
        <span className="mt-0.5 text-[11px] text-muted-foreground">{trades} trades</span>
      </div>
    </div>
  );
}

function ModelCard({ label, stats, Logo }: { label: string; stats: ModelStats; Logo: React.ComponentType<{ className?: string }> }) {
  return (
    <Card>
      <CardContent className="space-y-4 p-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Logo className="h-7 w-7" />
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</div>
              <div className="mt-0.5 font-mono text-base font-semibold">{stats.model}</div>
            </div>
          </div>
          <Badge variant={stats.total_pnl > 0 ? "default" : stats.total_pnl < 0 ? "destructive" : "secondary"}>{fmtPnlInt(stats.total_pnl)}</Badge>
        </div>

        <div className="grid grid-cols-2 gap-x-4 gap-y-2">
          <Metric label="Win rate" value={`${Math.round(stats.win_rate * 100)}%`} />
          <Metric label="Avg return" value={<span className={cn("text-sm font-semibold tabular-nums", pnlColor(stats.avg_pct_return))}>{fmtPctDec(stats.avg_pct_return)}</span>} />
          <Metric label="Trades evaluated" value={String(stats.trades_evaluated)} />
          <Metric label="Avg score" value={stats.avg_score.toFixed(1)} />
        </div>

        {stats.best_pick && (
          <div className="rounded-md border bg-muted/30 p-3">
            <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">Best pick</div>
            <div className="mt-1 flex items-center justify-between text-sm">
              <span className="font-mono font-semibold">
                ${stats.best_pick.symbol} {stats.best_pick.contract_type}
              </span>
              <span className={cn("font-semibold tabular-nums", pnlColor(stats.best_pick.pnl))}>
                {fmtPnlInt(stats.best_pick.pnl)} · {fmtPctDec(stats.best_pick.pct_return)}
              </span>
            </div>
            <div className="mt-1 text-[11px] text-muted-foreground">
              {stats.best_pick.date} · scored {stats.best_pick.score}/10
            </div>
          </div>
        )}

        {stats.worst_pick && (
          <div className="rounded-md border bg-muted/30 p-3">
            <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">Worst pick</div>
            <div className="mt-1 flex items-center justify-between text-sm">
              <span className="font-mono font-semibold">
                ${stats.worst_pick.symbol} {stats.worst_pick.contract_type}
              </span>
              <span className={cn("font-semibold tabular-nums", pnlColor(stats.worst_pick.pnl))}>
                {fmtPnlInt(stats.worst_pick.pnl)} · {fmtPctDec(stats.worst_pick.pct_return)}
              </span>
            </div>
            <div className="mt-1 text-[11px] text-muted-foreground">
              {stats.worst_pick.date} · scored {stats.worst_pick.score}/10
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
