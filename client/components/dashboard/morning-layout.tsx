"use client";

import { AlertTriangle, ArrowRight, Zap } from "lucide-react";
import { motion, type Variants } from "motion/react";
import Link from "next/link";
import type * as React from "react";

import { ExecutionBadge, findExecutionForTrade, liveMarkForTrade } from "@/components/execution-badge";
import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { calcMoneyness, isContractResolved } from "@/lib/calculations";
import { fmtMoney, fmtMoneyInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade, Execution, LiveOptionEntry, LiveQuotesResponse, Trade } from "@/types/trade";

interface MorningLayoutProps {
  trades: DashboardTrade[];
  liveQuotes?: LiveQuotesResponse | null;
  date: string;
  executions?: Execution[] | null;
}

function tradeHref(symbol: string, date: string): string {
  return `/trade/${encodeURIComponent(symbol)}?date=${encodeURIComponent(date)}`;
}

function getLiveOption(liveQuotes: LiveQuotesResponse | null | undefined, trade: Trade): LiveOptionEntry | null {
  if (!liveQuotes?.options) return null;
  if (trade.strike_price === null || trade.expiration === null) return null;
  const key = `${trade.symbol}|${trade.contract_type}|${trade.strike_price.toFixed(2)}|${trade.expiration}`;
  return liveQuotes.options[key] ?? null;
}

interface LiveDelta {
  mark: number | null;
  delta: number | null;
  deltaPct: number | null;
  anomalous: boolean;
}

function liveDelta(trade: Trade, liveQuotes: LiveQuotesResponse | null | undefined): LiveDelta {
  const lo = getLiveOption(liveQuotes, trade);
  if (!lo?.mark || lo.mark <= 0 || trade.estimated_price === null) {
    return {
      mark: lo?.mark ?? null,
      delta: null,
      deltaPct: null,
      anomalous: false,
    };
  }
  const delta = lo.mark - trade.estimated_price;
  const deltaPct = trade.estimated_price > 0 ? (delta / trade.estimated_price) * 100 : null;
  /*
  Anomaly heuristic kept as a defense-in-depth backstop: a >500% gain
  on a saved entry almost always means Schwab's live mark and the
  saved fill are pricing different worlds. Under the at-open
  resolution model this should be impossible (the executor saves a
  live ask, not a stale picker mark), but the badge still flags any
  drift large enough to look like a UI mistake.
  */
  const anomalous = deltaPct !== null && deltaPct > 500;
  return { mark: lo.mark, delta, deltaPct, anomalous };
}

const containerVariants: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.05, delayChildren: 0.04 } },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 10, filter: "blur(4px)" },
  show: {
    opacity: 1,
    y: 0,
    filter: "blur(0px)",
    transition: { duration: 0.4, ease: [0.22, 1, 0.36, 1] },
  },
};

export function MorningLayout({ trades, liveQuotes, date, executions }: MorningLayoutProps) {
  const sorted = [...trades].sort((a, b) => a.trade.rank - b.trade.rank);
  const hero = sorted[0] ?? null;
  // Picker returns exactly 3. Anything beyond rank 3 is legacy data
  // from the prior 10-picks regime and is ignored on the live morning
  // layout (the /history page still surfaces it).
  const basket = sorted.slice(1, 3);

  return (
    <motion.div className="space-y-10" variants={containerVariants} initial="hidden" animate="show">
      {hero && (
        <div className="space-y-5">
          <SectionLabel icon={<Zap className="h-3 w-3" />} label="Auto-fire basket" hint={`Top ${Math.min(3, sorted.length)} · all 3 fire live in my brokerage at 9:30 ET`} />
          <motion.div variants={itemVariants}>
            <HeroPick dt={hero} liveQuotes={liveQuotes} date={date} executions={executions} />
          </motion.div>
          {basket.length > 0 && (
            <div className="grid gap-4 md:grid-cols-2">
              {basket.map((dt) => (
                <motion.div key={dt.trade.symbol} variants={itemVariants}>
                  <RailPick dt={dt} liveQuotes={liveQuotes} date={date} executions={executions} />
                </motion.div>
              ))}
            </div>
          )}
        </div>
      )}
    </motion.div>
  );
}

function SectionLabel({ icon, label, hint }: { icon?: React.ReactNode; label: string; hint?: string }) {
  return (
    <div>
      <div className="flex items-baseline justify-between gap-3 pb-2">
        <Badge variant="outline" className="gap-1.5 rounded-full px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-[0.16em] text-foreground/75">
          {icon}
          {label}
        </Badge>
        {hint && <span className="hidden text-[11px] text-muted-foreground sm:inline">{hint}</span>}
      </div>
      <Separator />
    </div>
  );
}

function HeroPick({ dt, liveQuotes, date, executions }: { dt: DashboardTrade; liveQuotes?: LiveQuotesResponse | null; date: string; executions?: Execution[] | null }) {
  const { trade } = dt;
  const resolved = isContractResolved(trade);
  const moneyness = calcMoneyness(trade);
  const { mark, delta, deltaPct, anomalous } = liveDelta(trade, liveQuotes);
  const execution = findExecutionForTrade(executions, trade);
  const showDeltaPct = deltaPct !== null && Math.abs(deltaPct) <= 500;
  const riskVariant = trade.risk_level === "HIGH" ? "destructive" : trade.risk_level === "MEDIUM" ? "outline" : "secondary";

  return (
    <Link href={tradeHref(trade.symbol, date)} className="group block">
      <Card className="lg-panel relative gap-0 overflow-hidden rounded-2xl border border-border/60 py-0 shadow-sm transition-all hover:border-foreground/30 hover:shadow-md">
        <div className="lg-orb lg-orb-claude pointer-events-none absolute -top-32 -right-24 h-80 w-80 opacity-[0.18]" aria-hidden />

        <CardContent className="relative grid gap-6 px-5 py-5 sm:px-7 sm:py-6 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start lg:gap-10">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-gradient-brand text-xs font-bold text-white shadow-sm">1</span>
              <span className="text-2xl font-extrabold tracking-tight sm:text-3xl">${trade.symbol}</span>
              {resolved ? (
                <>
                  <Badge variant="outline" className={cn("text-[11px] font-semibold", trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
                    {trade.contract_type} {fmtMoney(trade.strike_price ?? 0)}
                  </Badge>
                  <Badge variant={moneyness.variant}>{moneyness.label}</Badge>
                </>
              ) : (
                <Badge variant="outline" className={cn("text-[11px] font-semibold", trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
                  {trade.contract_type} · finding contracts...
                </Badge>
              )}
              <Badge variant={riskVariant}>{trade.risk_level}</Badge>
              {resolved && trade.dte !== null && <span className="text-[11px] text-muted-foreground">{trade.dte}d to expiry</span>}
              {trade.score > 0 && (
                <Badge variant="outline" className="gap-1 bg-muted/40 text-[11px] font-semibold tabular-nums">
                  <ClaudeLogo className="h-3 w-3" />
                  {trade.score}/10
                </Badge>
              )}
              {anomalous && (
                <Badge
                  variant="outline"
                  className="gap-1 border-amber-border text-[10px] font-medium text-amber"
                  title="Live mark diverges sharply from the saved entry. Likely stale picker quote or transient data anomaly."
                >
                  <AlertTriangle className="h-3 w-3" />
                  Data check
                </Badge>
              )}
              {execution && <ExecutionBadge execution={execution} liveMark={liveMarkForTrade(liveQuotes, trade)} />}
            </div>

            {trade.catalyst && (
              <div className="mt-4 rounded-md bg-amber-bg px-3 py-2 text-sm">
                <span className="font-semibold text-amber">Catalyst:</span> {trade.catalyst}
              </div>
            )}

            {trade.thesis && <p className="mt-4 max-w-2xl text-sm leading-relaxed text-muted-foreground sm:text-[15px]">{trade.thesis}</p>}

            <div className="mt-5 inline-flex items-center gap-1.5 text-xs font-semibold text-foreground/70 transition-colors group-hover:text-foreground">
              View Claudia&apos;s full rationale
              <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
            </div>
          </div>

          {resolved ? (
            <div className="grid grid-cols-2 gap-x-8 gap-y-1 lg:min-w-[280px]">
              <PriceBlock label="Buy" value={fmtMoney(trade.estimated_price ?? 0)} sub={`${fmtMoneyInt((trade.estimated_price ?? 0) * 100)} / contract`} />
              <PriceBlock
                label="Current"
                value={mark !== null ? <span className={cn(pnlColor(delta ?? 0))}>{fmtMoney(mark)}</span> : <Skeleton className="inline-block h-9 w-24 align-middle" />}
                sub={mark !== null ? (showDeltaPct && deltaPct !== null ? `${deltaPct > 0 ? "+" : ""}${deltaPct.toFixed(1)}%` : `${fmtMoneyInt(mark * 100)} / contract`) : undefined}
                subClassName={mark !== null && showDeltaPct ? pnlColor(delta ?? 0) : undefined}
              />
            </div>
          ) : (
            <PendingContractBlock intent={contractIntentLabel(trade)} />
          )}
        </CardContent>
      </Card>
    </Link>
  );
}

function contractIntentLabel(trade: Trade): string {
  const otm = trade.target_otm_pct;
  const dte = trade.min_dte;
  const otmStr = otm <= 0.1 ? "ATM" : `~${otm.toFixed(otm < 1 ? 1 : 0)}% OTM`;
  const dteStr = dte <= 0 ? "0DTE" : `${dte}+ DTE`;
  return `${otmStr}, ${dteStr}`;
}

function PendingContractBlock({ intent }: { intent: string }) {
  return (
    <div className="flex min-w-0 flex-col items-start gap-1 rounded-lg border border-dashed border-border/70 bg-muted/20 px-4 py-3 lg:min-w-[280px]">
      <div className="flex items-center gap-2">
        <Skeleton className="h-3 w-3 rounded-full" />
        <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">Finding contracts</span>
      </div>
      <div className="text-sm font-medium text-foreground/85">{intent}</div>
      <div className="text-[11px] text-muted-foreground">Resolves at market open (9:30 ET)</div>
    </div>
  );
}

function PriceBlock({ label, value, sub, subClassName }: { label: string; value: React.ReactNode; sub?: React.ReactNode; subClassName?: string }) {
  return (
    <div>
      <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className="mt-1 text-3xl font-semibold leading-none tabular-nums sm:text-4xl">{value}</div>
      {sub && <div className={cn("mt-2 text-[11px]", subClassName ?? "text-muted-foreground")}>{sub}</div>}
    </div>
  );
}

function RailPick({ dt, liveQuotes, date, executions }: { dt: DashboardTrade; liveQuotes?: LiveQuotesResponse | null; date: string; executions?: Execution[] | null }) {
  const { trade } = dt;
  const resolved = isContractResolved(trade);
  const moneyness = calcMoneyness(trade);
  const { mark, delta, deltaPct } = liveDelta(trade, liveQuotes);
  const execution = findExecutionForTrade(executions, trade);
  const showDeltaPct = deltaPct !== null && Math.abs(deltaPct) <= 500;

  return (
    <Link href={tradeHref(trade.symbol, date)} className="group block h-full">
      <Card className="h-full gap-0 rounded-xl border border-border/60 bg-card/30 py-0 shadow-none transition-all hover:-translate-y-0.5 hover:border-foreground/30 hover:bg-card/60 hover:shadow-sm">
        <CardContent className="flex h-full flex-col p-4">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-foreground/10 text-[10px] font-bold tabular-nums">{trade.rank}</span>
            <span className="text-lg font-bold tracking-tight">${trade.symbol}</span>
            {resolved ? (
              <>
                <Badge variant="outline" className={cn("text-[10px] font-semibold", trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
                  {trade.contract_type} {fmtMoney(trade.strike_price ?? 0)}
                </Badge>
                <Badge variant={moneyness.variant} className="text-[10px]">
                  {moneyness.label}
                </Badge>
              </>
            ) : (
              <Badge variant="outline" className={cn("text-[10px] font-semibold", trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
                {trade.contract_type} · finding contracts...
              </Badge>
            )}
            {trade.score > 0 && (
              <Badge variant="outline" className="ml-auto gap-1 bg-muted/40 text-[10px] font-semibold tabular-nums">
                <ClaudeLogo className="h-3 w-3" />
                {trade.score}/10
              </Badge>
            )}
          </div>

          {resolved ? (
            <div className="mt-3 grid grid-cols-2 gap-x-4">
              <div>
                <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Buy</div>
                <div className="mt-0.5 text-xl font-semibold leading-none tabular-nums">{fmtMoney(trade.estimated_price ?? 0)}</div>
              </div>
              <div>
                <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Current</div>
                <div className={cn("mt-0.5 text-xl font-semibold leading-none tabular-nums", mark !== null ? pnlColor(delta ?? 0) : "")}>
                  {mark !== null ? (
                    <>
                      {fmtMoney(mark)}
                      {showDeltaPct && deltaPct !== null && (
                        <span className="ml-1 text-[10px] font-medium">
                          ({deltaPct > 0 ? "+" : ""}
                          {deltaPct.toFixed(1)}%)
                        </span>
                      )}
                    </>
                  ) : (
                    <Skeleton className="inline-block h-6 w-20 align-middle" />
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="mt-3 text-[11px] text-muted-foreground">
              <span className="font-medium text-foreground/80">{contractIntentLabel(trade)}</span> · resolves at open
            </div>
          )}

          {trade.thesis && <p className="mt-3 line-clamp-2 text-[13px] leading-relaxed text-muted-foreground">{trade.thesis}</p>}

          {execution && (
            <div className="mt-3">
              <ExecutionBadge execution={execution} liveMark={liveMarkForTrade(liveQuotes, trade)} />
            </div>
          )}

          <div className="mt-auto inline-flex items-center gap-1 pt-3 text-[11px] font-medium text-muted-foreground transition-colors group-hover:text-foreground">
            View contract
            <ArrowRight className="h-3 w-3 transition-transform group-hover:translate-x-0.5" />
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
