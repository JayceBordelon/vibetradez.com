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
import { StatCard } from "@/components/ui/stat-card";
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
            <Section title="Headline metrics" subtitle={`Top ${data.top_n} picks per day, replayed under each model's ranking`}>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <StatCard
                  label="OpenAI net P&L"
                  value={fmtPnlInt(data.openai.total_pnl)}
                  tone={data.openai.total_pnl > 0 ? "positive" : data.openai.total_pnl < 0 ? "negative" : "neutral"}
                  icon={OpenAILogo}
                  sub={`${data.openai.trades_evaluated} trades`}
                />
                <StatCard
                  label="Anthropic net P&L"
                  value={fmtPnlInt(data.anthropic.total_pnl)}
                  tone={data.anthropic.total_pnl > 0 ? "positive" : data.anthropic.total_pnl < 0 ? "negative" : "neutral"}
                  icon={ClaudeLogo}
                  sub={`${data.anthropic.trades_evaluated} trades`}
                />
                <StatCard
                  label="Combined net P&L"
                  value={fmtPnlInt(data.combined.total_pnl)}
                  tone={data.combined.total_pnl > 0 ? "positive" : data.combined.total_pnl < 0 ? "negative" : "neutral"}
                  icon={Sparkle}
                  sub={`${data.combined.trades_evaluated} trades`}
                />
                <StatCard
                  label="Agreement rate"
                  value={`${Math.round(data.agreement_rate * 100)}%`}
                  valueColor={percentHueColor(data.agreement_rate * 100)}
                  sub={`${data.total_dual_scored} dual-scored trades within ±1`}
                />
              </div>

              {winner && winner !== "tie" && (
                <div className="mt-4 flex items-center justify-center gap-2 rounded-md border bg-muted/30 px-4 py-3 text-sm">
                  <Sparkle className="h-4 w-4 text-amber" />
                  <span className="font-semibold">{winner === "openai" ? data.openai.model : data.anthropic.model}</span>
                  <span className="text-muted-foreground">leads by </span>
                  <span className={cn("font-semibold", pnlColor(Math.abs(data.openai.total_pnl - data.anthropic.total_pnl)))}>
                    {fmtPnlInt(Math.abs(data.openai.total_pnl - data.anthropic.total_pnl))}
                  </span>
                </div>
              )}
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
