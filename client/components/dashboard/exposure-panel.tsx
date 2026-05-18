import { executionQuantity, findExecutionForTrade } from "@/components/execution-badge";
import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { computeTradePnl } from "@/lib/calculations";
import { fmt, fmtMoney, fmtMoneyInt, fmtPctDec } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade, Execution } from "@/types/trade";

interface ExposurePanelProps {
  trades: DashboardTrade[];
  hasSummaries: boolean;
  executions?: Execution[] | null;
}

export function ExposurePanel({ trades, hasSummaries, executions }: ExposurePanelProps) {
  const count = trades.length;

  /*
  Capital deployed multiplies the entry premium by the executed
  contract count (quantity > 1 happens when the greedy fill duplicates
  a rank). When no execution exists the pick is hypothetical at qty 1,
  matching the "what if you bought one contract" model the summary
  represents.
  */
  const totalExposure = trades.reduce((sum, dt) => {
    const exec = findExecutionForTrade(executions, dt.trade);
    const qty = executionQuantity(exec);
    const result = computeTradePnl(dt, executions);
    if (result.hasData && result.entryPrice > 0) {
      return sum + result.entryPrice * 100 * qty;
    }
    return sum + (dt.trade.estimated_price ?? 0) * 100 * qty;
  }, 0);

  const totalContracts = trades.reduce((sum, dt) => sum + executionQuantity(findExecutionForTrade(executions, dt.trade)), 0);
  const avgPremium = totalContracts > 0 ? totalExposure / totalContracts / 100 : 0;
  const avgDte = count > 0 ? trades.reduce((sum, dt) => sum + dt.trade.dte, 0) / count : 0;

  let totalReturned = 0;
  let netPnl = 0;
  let roc: number | null = null;

  if (hasSummaries) {
    totalReturned = trades.reduce((sum, dt) => {
      const exec = findExecutionForTrade(executions, dt.trade);
      const qty = executionQuantity(exec);
      const result = computeTradePnl(dt, executions);
      if (!result.hasData) return sum;
      return sum + result.closingPrice * 100 * qty;
    }, 0);
    netPnl = totalReturned - totalExposure;
    roc = totalExposure > 0 ? (netPnl / totalExposure) * 100 : 0;
  }

  const rocTone: "positive" | "negative" | "neutral" = roc === null ? "neutral" : roc > 0 ? "positive" : roc < 0 ? "negative" : "neutral";

  /**
  Both bars share a common scale so they're visually comparable.
  */
  const barMax = Math.max(totalExposure, totalReturned);
  const deployedPct = barMax > 0 ? (totalExposure / barMax) * 100 : 0;
  const returnedPct = barMax > 0 ? (totalReturned / barMax) * 100 : 0;
  const returnedBarColor = totalReturned >= totalExposure ? "bg-green" : "bg-red";

  return (
    <div className="space-y-6">
      <StatStrip cols={hasSummaries && roc !== null ? 4 : 3}>
        <Stat label="Capital at Risk" value={fmtMoneyInt(totalExposure)} />
        <Stat label="Avg Premium" value={fmtMoney(avgPremium)} />
        <Stat label="Avg DTE" value={fmt(avgDte, 1)} />
        {hasSummaries && roc !== null && <Stat label="ROC" value={fmtPctDec(roc)} tone={rocTone} />}
      </StatStrip>

      {hasSummaries && totalExposure > 0 && (
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>Deployed vs Returned</span>
            <span className="tabular-nums">
              {fmtMoneyInt(totalExposure)} → {fmtMoneyInt(totalReturned)}
            </span>
          </div>
          <div className="space-y-1.5">
            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-amber transition-all" style={{ width: `${deployedPct}%` }} />
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
              <div className={cn("h-full rounded-full transition-all", returnedBarColor)} style={{ width: `${returnedPct}%` }} />
            </div>
          </div>
          <div className="flex justify-between text-[11px] text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <span className="inline-block h-2 w-2 rounded-full bg-amber" />
              Deployed
            </span>
            <span className="flex items-center gap-1.5">
              <span className={cn("inline-block h-2 w-2 rounded-full", returnedBarColor)} />
              Returned
            </span>
          </div>
        </div>
      )}

      {!hasSummaries && totalExposure > 0 && <MorningBreakdown trades={trades} totalExposure={totalExposure} executions={executions} />}
    </div>
  );
}

function MorningBreakdown({ trades, totalExposure, executions }: { trades: DashboardTrade[]; totalExposure: number; executions?: Execution[] | null }) {
  const buckets = { LOW: 0, MEDIUM: 0, HIGH: 0 } as Record<"LOW" | "MEDIUM" | "HIGH", number>;
  for (const dt of trades) {
    const qty = executionQuantity(findExecutionForTrade(executions, dt.trade));
    const level = (dt.trade.risk_level ?? "MEDIUM") as keyof typeof buckets;
    const cost = (dt.trade.estimated_price ?? 0) * 100 * qty;
    if (level in buckets) {
      buckets[level] += cost;
    } else {
      buckets.MEDIUM += cost;
    }
  }
  const lowPct = totalExposure > 0 ? (buckets.LOW / totalExposure) * 100 : 0;
  const medPct = totalExposure > 0 ? (buckets.MEDIUM / totalExposure) * 100 : 0;
  const highPct = totalExposure > 0 ? (buckets.HIGH / totalExposure) * 100 : 0;

  if (buckets.LOW === 0 && buckets.MEDIUM === 0 && buckets.HIGH === 0) return null;

  return (
    <div className="space-y-2">
      <div className="text-xs text-muted-foreground">Capital by risk level</div>
      <div className="flex h-2 w-full overflow-hidden rounded-full bg-muted">
        {lowPct > 0 && <div className="h-full bg-blue-500/80 transition-all" style={{ width: `${lowPct}%` }} />}
        {medPct > 0 && <div className="h-full bg-amber transition-all" style={{ width: `${medPct}%` }} />}
        {highPct > 0 && <div className="h-full bg-red transition-all" style={{ width: `${highPct}%` }} />}
      </div>
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-2 rounded-full bg-blue-500/80" />
          LOW {fmtMoneyInt(buckets.LOW)} ({lowPct.toFixed(0)}%)
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-2 rounded-full bg-amber" />
          MED {fmtMoneyInt(buckets.MEDIUM)} ({medPct.toFixed(0)}%)
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-2 rounded-full bg-red" />
          HIGH {fmtMoneyInt(buckets.HIGH)} ({highPct.toFixed(0)}%)
        </span>
      </div>
    </div>
  );
}
