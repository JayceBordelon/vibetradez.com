"use client";

import { AlertTriangle, ArrowRight, ChevronRight, Zap } from "lucide-react";
import { motion, type Variants } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type * as React from "react";

import { ExecutionBadge, findExecutionForTrade, liveMarkForTrade } from "@/components/execution-badge";
import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { calcMoneyness } from "@/lib/calculations";
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
  if (!lo?.mark || lo.mark <= 0) return { mark: null, delta: null, deltaPct: null, anomalous: false };
  const delta = lo.mark - trade.estimated_price;
  const deltaPct = trade.estimated_price > 0 ? (delta / trade.estimated_price) * 100 : null;
  /*
  Anomaly heuristic mirrored from the old morning-cards: a >500%
  gain on a saved estimate almost always means Schwab's live mark and
  Claude's picker quote are pricing different worlds. The picker-side
  validation already drops these at save time, but this stays as
  defense-in-depth on the hero pick where the delta is most prominent.
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
  show: { opacity: 1, y: 0, filter: "blur(0px)", transition: { duration: 0.4, ease: [0.22, 1, 0.36, 1] } },
};

export function MorningLayout({ trades, liveQuotes, date, executions }: MorningLayoutProps) {
  const sorted = [...trades].sort((a, b) => a.trade.rank - b.trade.rank);
  const hero = sorted[0] ?? null;
  const basket = sorted.slice(1, 3);
  const watchlist = sorted.slice(3);

  return (
    <motion.div className="space-y-10" variants={containerVariants} initial="hidden" animate="show">
      {hero && (
        <div className="space-y-5">
          <SectionLabel icon={<Zap className="h-3 w-3" />} label="Auto-fire basket" hint={`Top ${Math.min(3, sorted.length)} · ordered live in my brokerage at 9:31 ET`} />
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

      {watchlist.length > 0 && (
        <div className="space-y-4">
          <SectionLabel label="Watchlist" hint={`Picks #4-${watchlist.length + 3} · ranked for reference, not executed`} />
          <motion.div variants={itemVariants}>
            <WatchlistTable trades={watchlist} liveQuotes={liveQuotes} date={date} executions={executions} />
          </motion.div>
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
              <Badge variant="outline" className={cn("text-[11px] font-semibold", trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
                {trade.contract_type} {fmtMoney(trade.strike_price)}
              </Badge>
              <Badge variant={moneyness.variant}>{moneyness.label}</Badge>
              <Badge variant={riskVariant}>{trade.risk_level}</Badge>
              <span className="text-[11px] text-muted-foreground">{trade.dte}d to expiry</span>
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

          <div className="grid grid-cols-2 gap-x-8 gap-y-1 lg:min-w-[280px]">
            <PriceBlock label="Buy" value={fmtMoney(trade.estimated_price)} sub={`${fmtMoneyInt(trade.estimated_price * 100)} / contract`} />
            <PriceBlock
              label="Current"
              value={mark !== null ? <span className={cn(pnlColor(delta ?? 0))}>{fmtMoney(mark)}</span> : <Skeleton className="inline-block h-9 w-24 align-middle" />}
              sub={mark !== null ? (showDeltaPct && deltaPct !== null ? `${deltaPct > 0 ? "+" : ""}${deltaPct.toFixed(1)}%` : `${fmtMoneyInt(mark * 100)} / contract`) : undefined}
              subClassName={mark !== null && showDeltaPct ? pnlColor(delta ?? 0) : undefined}
            />
          </div>
        </CardContent>
      </Card>
    </Link>
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
            <Badge variant="outline" className={cn("text-[10px] font-semibold", trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
              {trade.contract_type} {fmtMoney(trade.strike_price)}
            </Badge>
            <Badge variant={moneyness.variant} className="text-[10px]">
              {moneyness.label}
            </Badge>
            {trade.score > 0 && (
              <Badge variant="outline" className="ml-auto gap-1 bg-muted/40 text-[10px] font-semibold tabular-nums">
                <ClaudeLogo className="h-3 w-3" />
                {trade.score}/10
              </Badge>
            )}
          </div>

          <div className="mt-3 grid grid-cols-2 gap-x-4">
            <div>
              <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Buy</div>
              <div className="mt-0.5 text-xl font-semibold leading-none tabular-nums">{fmtMoney(trade.estimated_price)}</div>
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

function WatchlistTable({ trades, liveQuotes, date, executions }: { trades: DashboardTrade[]; liveQuotes?: LiveQuotesResponse | null; date: string; executions?: Execution[] | null }) {
  return (
    <div className="min-w-0">
      <div className="hidden overflow-hidden rounded-xl border border-border/60 md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-12 text-center">#</TableHead>
              <TableHead>Trade</TableHead>
              <TableHead className="text-center">Score</TableHead>
              <TableHead className="text-right">Buy</TableHead>
              <TableHead className="text-right">Current</TableHead>
              <TableHead className="text-center">DTE</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {trades.map((dt) => (
              <WatchlistRow key={dt.trade.symbol} dt={dt} liveQuotes={liveQuotes} date={date} executions={executions} />
            ))}
          </TableBody>
        </Table>
      </div>

      <div className="space-y-2 md:hidden">
        {trades.map((dt) => (
          <WatchlistMobileRow key={dt.trade.symbol} dt={dt} liveQuotes={liveQuotes} date={date} />
        ))}
      </div>
    </div>
  );
}

function WatchlistRow({ dt, liveQuotes, date, executions }: { dt: DashboardTrade; liveQuotes?: LiveQuotesResponse | null; date: string; executions?: Execution[] | null }) {
  const router = useRouter();
  const { trade } = dt;
  const moneyness = calcMoneyness(trade);
  const { mark, delta, deltaPct } = liveDelta(trade, liveQuotes);
  const execution = findExecutionForTrade(executions, trade);
  const href = tradeHref(trade.symbol, date);
  const showDeltaPct = deltaPct !== null && Math.abs(deltaPct) <= 500;

  return (
    <TableRow className="cursor-pointer transition-colors hover:bg-muted/50" onClick={() => router.push(href)}>
      <TableCell className="text-center text-sm tabular-nums text-muted-foreground">{trade.rank}</TableCell>
      <TableCell>
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="font-mono text-sm font-semibold">${trade.symbol}</span>
          <Badge variant="outline" className={cn("text-[10px] font-semibold", trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
            {trade.contract_type} {fmtMoney(trade.strike_price)}
          </Badge>
          <Badge variant={moneyness.variant} className="text-[10px]">
            {moneyness.label}
          </Badge>
          {execution && <ExecutionBadge execution={execution} liveMark={liveMarkForTrade(liveQuotes, trade)} />}
        </div>
      </TableCell>
      <TableCell className="text-center text-xs tabular-nums">
        {trade.score > 0 ? (
          <Badge variant="outline" className="gap-1 bg-muted/40 font-semibold">
            <ClaudeLogo className="h-3 w-3" />
            {trade.score}/10
          </Badge>
        ) : (
          <span className="text-muted-foreground">-</span>
        )}
      </TableCell>
      <TableCell className="text-right font-mono text-sm tabular-nums">{fmtMoney(trade.estimated_price)}</TableCell>
      <TableCell className={cn("text-right font-mono text-sm tabular-nums", mark !== null ? pnlColor(delta ?? 0) : "text-muted-foreground")}>
        {mark !== null ? (
          <span>
            {fmtMoney(mark)}
            {showDeltaPct && deltaPct !== null && (
              <span className="ml-1 text-[10px]">
                ({deltaPct > 0 ? "+" : ""}
                {deltaPct.toFixed(1)}%)
              </span>
            )}
          </span>
        ) : (
          <Skeleton className="inline-block h-4 w-16 align-middle" />
        )}
      </TableCell>
      <TableCell className="text-center text-xs tabular-nums text-muted-foreground">{trade.dte}d</TableCell>
      <TableCell className="w-10 text-right">
        <Link
          href={href}
          aria-label={`Open ${trade.symbol} detail`}
          onClick={(e) => e.stopPropagation()}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <ChevronRight className="h-4 w-4" />
        </Link>
      </TableCell>
    </TableRow>
  );
}

function WatchlistMobileRow({ dt, liveQuotes, date }: { dt: DashboardTrade; liveQuotes?: LiveQuotesResponse | null; date: string }) {
  const { trade } = dt;
  const { mark, delta } = liveDelta(trade, liveQuotes);

  return (
    <Link href={tradeHref(trade.symbol, date)} className="block">
      <Card className="gap-0 rounded-lg border border-border/60 bg-card/30 py-0 shadow-none transition-colors hover:bg-card/60">
        <CardContent className="flex items-center justify-between gap-3 p-3">
          <div className="flex min-w-0 items-center gap-2">
            <span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-foreground/10 text-[10px] font-bold tabular-nums">{trade.rank}</span>
            <span className="font-mono text-sm font-semibold">${trade.symbol}</span>
            <Badge variant="outline" className={cn("text-[10px]", trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
              {trade.contract_type}
            </Badge>
          </div>
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-xs tabular-nums text-muted-foreground">{fmtMoney(trade.estimated_price)}</span>
            <span className="text-muted-foreground/40">→</span>
            <span className={cn("font-mono text-sm font-semibold tabular-nums", mark !== null ? pnlColor(delta ?? 0) : "text-muted-foreground")}>{mark !== null ? fmtMoney(mark) : "-"}</span>
            <ChevronRight className="h-4 w-4 text-muted-foreground/60" />
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
