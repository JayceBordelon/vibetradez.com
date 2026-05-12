"use client";

import { AlertTriangle, ArrowRight } from "lucide-react";
import { motion, type Variants } from "motion/react";
import Link from "next/link";

import { ExecutionBadge, findExecutionForTrade, liveMarkForTrade } from "@/components/execution-badge";
import { Badge } from "@/components/ui/badge";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { calcMoneyness } from "@/lib/calculations";
import { fmtMoney, fmtMoneyInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade, Execution, LiveQuotesResponse } from "@/types/trade";

interface MorningCardsProps {
  trades: DashboardTrade[];
  liveQuotes?: LiveQuotesResponse | null;
  date: string;
  executions?: Execution[] | null;
}

function tradeHref(symbol: string, date: string): string {
  return `/trade/${encodeURIComponent(symbol)}?date=${encodeURIComponent(date)}`;
}

const containerVariants: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.045, delayChildren: 0.04 } },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 12, filter: "blur(4px)" },
  show: { opacity: 1, y: 0, filter: "blur(0px)", transition: { duration: 0.35, ease: [0.22, 1, 0.36, 1] } },
};

export function MorningCards({ trades, liveQuotes, date, executions }: MorningCardsProps) {
  return (
    <motion.div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" variants={containerVariants} initial="hidden" animate="show">
      {trades.map((dt) => (
        <motion.div key={dt.trade.symbol} variants={itemVariants}>
          <MorningCard dt={dt} liveQuotes={liveQuotes} date={date} execution={findExecutionForTrade(executions, dt.trade)} />
        </motion.div>
      ))}
    </motion.div>
  );
}

interface MorningCardProps {
  dt: DashboardTrade;
  liveQuotes?: LiveQuotesResponse | null;
  date: string;
  execution?: Execution | null;
}

function MorningCard({ dt, liveQuotes, date, execution }: MorningCardProps) {
  const { trade } = dt;
  const moneyness = calcMoneyness(trade);

  /**
  Live option mark for "current contract price". Backend keys are
  "<SYMBOL>|<CALL|PUT>|<strike formatted to 2dp>|<expiration>" (see
  server.go:846), so reconstruct the same key here. Falls back to
  null when Schwab isn't connected or the contract dropped off the
  chain; the card then renders a Skeleton pulse so the missing slot
  reads as "loading" rather than "absent".
  */
  const optionKey = `${trade.symbol}|${trade.contract_type}|${trade.strike_price.toFixed(2)}|${trade.expiration}`;
  const liveOption = liveQuotes?.options?.[optionKey] ?? null;
  const currentContractPrice = liveOption?.mark ?? null;
  const contractDelta = currentContractPrice !== null ? currentContractPrice - trade.estimated_price : null;
  const contractDeltaPct = contractDelta !== null && trade.estimated_price > 0 ? (contractDelta / trade.estimated_price) * 100 : null;

  /*
  Hide the divergence percent when it crosses into the "Data check"
  band so the cell stops printing nonsense numbers like +44,000% or
  -99%. The amber badge below already labels the row as suspect; an
  inline percent on top of that is just noise that reads as a real
  return. The dollar Mark stays so the reader can still see the live
  premium that Schwab is reporting.
  */
  const showDeltaPct = contractDeltaPct !== null && Math.abs(contractDeltaPct) <= 500;
  const currentValue =
    currentContractPrice !== null ? (
      <span className={cn("text-sm font-semibold tabular-nums", pnlColor(contractDelta ?? 0))}>
        {fmtMoney(currentContractPrice)}
        {showDeltaPct && contractDeltaPct !== null && (
          <span className="ml-1 text-[11px]">
            ({contractDeltaPct > 0 ? "+" : ""}
            {contractDeltaPct.toFixed(1)}%)
          </span>
        )}
      </span>
    ) : (
      <Skeleton className="inline-block h-7 w-20 align-middle" />
    );

  const riskBadgeVariant: "destructive" | "outline" | "secondary" = trade.risk_level === "HIGH" ? "destructive" : trade.risk_level === "MEDIUM" ? "outline" : "secondary";
  const showScore = trade.score > 0;
  /*
  Anomaly heuristic: a >500% gain on a short-DTE OTM contract almost
  always means Schwab's live mark and Claude's saved estimate are
  pricing different worlds (stale picker quote, mid-day data glitch,
  or wrong strike). Real legit wins (a 2-3x return on a directional
  call) stay well under the threshold. Pure decays produce negative
  deltas so they never trigger this. The picker-side validation drops
  these picks at save time as of the schwab-live-quote-fixes branch;
  this badge is the defense-in-depth for picks that slipped through
  or for live quotes that drift wildly post-save.
  */
  const isAnomalousDivergence = contractDeltaPct !== null && contractDeltaPct > 500;

  return (
    <Link href={tradeHref(trade.symbol, date)} className="block">
      <Card className="lg-card group h-full transition-all hover:-translate-y-0.5 hover:border-foreground/30 hover:shadow-md">
        <CardContent className="space-y-4 p-5">
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant="secondary">#{trade.rank}</Badge>
            <span className="text-xl font-bold tracking-tight">${trade.symbol}</span>
            <Badge variant="outline" className={cn(trade.contract_type === "CALL" ? "border-green-border text-green" : "border-red-border text-red")}>
              {trade.contract_type}
            </Badge>
            <Badge variant={moneyness.variant}>{moneyness.label}</Badge>
            <Badge variant={riskBadgeVariant}>{trade.risk_level}</Badge>
            {isAnomalousDivergence && (
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
            {showScore && (
              <div className="ml-auto inline-flex items-center gap-1 rounded-md border bg-muted/40 px-2 py-0.5 text-[11px] font-semibold tabular-nums">
                <ClaudeLogo className="h-3 w-3" />
                <span>{trade.score}/10</span>
              </div>
            )}
          </div>

          <div className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
            <div>
              <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Buy</div>
              <div className="mt-0.5 text-2xl font-semibold leading-none tabular-nums">{fmtMoney(trade.estimated_price)}</div>
              <div className="mt-0.5 text-[11px] text-muted-foreground">{fmtMoneyInt(trade.estimated_price * 100)} / contract</div>
            </div>
            <div>
              <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Current</div>
              <div className="mt-0.5 text-2xl font-semibold leading-none tabular-nums">{currentValue}</div>
              <div className="mt-0.5 text-[11px] text-muted-foreground">
                {currentContractPrice !== null ? <>{fmtMoneyInt(currentContractPrice * 100)} / contract</> : <Skeleton className="inline-block h-3 w-24 align-middle" />}
              </div>
            </div>
          </div>

          {trade.catalyst && (
            <div className="rounded-md bg-amber-bg px-3 py-2 text-sm">
              <span className="font-semibold text-amber">Catalyst:</span> {trade.catalyst}
            </div>
          )}

          {trade.thesis && <p className="text-sm leading-relaxed text-muted-foreground">{trade.thesis}</p>}

          <div className="flex items-center justify-between border-t pt-3 text-xs font-medium text-muted-foreground transition-colors group-hover:text-foreground">
            <span>View full contract</span>
            <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
