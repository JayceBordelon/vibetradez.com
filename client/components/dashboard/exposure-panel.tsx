import { Card, CardContent } from "@/components/ui/card";
import { Metric } from "@/components/ui/metric";
import { fmt, fmtMoney, fmtMoneyInt, fmtPctDec } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade } from "@/types/trade";

interface ExposurePanelProps {
  trades: DashboardTrade[];
  hasSummaries: boolean;
}

export function ExposurePanel({ trades, hasSummaries }: ExposurePanelProps) {
  const count = trades.length;

  const totalExposure = trades.reduce((sum, dt) => {
    const price = dt.summary?.entry_price ?? dt.trade.estimated_price ?? 0;
    return sum + price * 100;
  }, 0);

  const avgPremium = count > 0 ? totalExposure / count / 100 : 0;

  const avgDte = count > 0 ? trades.reduce((sum, dt) => sum + dt.trade.dte, 0) / count : 0;

  let totalReturned = 0;
  let netPnl = 0;
  let roc: number | null = null;

  if (hasSummaries) {
    const withSummaries = trades.filter((dt) => dt.summary);
    totalReturned = withSummaries.reduce((sum, dt) => {
      if (!dt.summary) return sum;
      return sum + dt.summary.closing_price * 100;
    }, 0);
    netPnl = totalReturned - totalExposure;
    roc = totalExposure > 0 ? (netPnl / totalExposure) * 100 : 0;
  }

  const rocColor = roc === null ? "" : roc > 0 ? "text-green" : roc < 0 ? "text-red" : "text-muted-foreground";

  /**
  Both bars share a common scale so they're visually comparable: whichever
  is larger fills 100%, the other fills proportionally less. Without this
  the deployed bar always pinned to 100% and a winning day pushed the
  returned bar past its container, making every day look identical.
  */
  const barMax = Math.max(totalExposure, totalReturned);
  const deployedPct = barMax > 0 ? (totalExposure / barMax) * 100 : 0;
  const returnedPct = barMax > 0 ? (totalReturned / barMax) * 100 : 0;
  const returnedBarColor = totalReturned >= totalExposure ? "bg-green" : "bg-red";

  return (
    <Card className="lg-card">
      <CardContent className="space-y-5 p-5">
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <Metric label="Capital at Risk" value={fmtMoneyInt(totalExposure)} />
          <Metric label="Avg Premium" value={fmtMoney(avgPremium)} />
          <Metric label="Avg DTE" value={fmt(avgDte, 1)} />
          {hasSummaries && roc !== null && <Metric label="ROC" value={<span className={cn("text-sm font-semibold tabular-nums", rocColor)}>{fmtPctDec(roc)}</span>} />}
        </div>

        {hasSummaries && totalExposure > 0 && (
          <div className="space-y-2">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>Deployed vs Returned</span>
              <span className="tabular-nums">
                {fmtMoneyInt(totalExposure)} &rarr; {fmtMoneyInt(totalReturned)}
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

        {/* Morning mode (no EOD summaries yet): risk-level capital
            distribution so the section has substance instead of sitting
            empty until 4pm. */}
        {!hasSummaries && totalExposure > 0 && <MorningBreakdown trades={trades} totalExposure={totalExposure} />}
      </CardContent>
    </Card>
  );
}

function MorningBreakdown({ trades, totalExposure }: { trades: DashboardTrade[]; totalExposure: number }) {
  /**
  Risk-level capital share, computed from premium paid per pick. Long
  options can only lose the premium so total premium == capital at risk
  for the bucket.
  */
  const buckets = { LOW: 0, MEDIUM: 0, HIGH: 0 } as Record<"LOW" | "MEDIUM" | "HIGH", number>;
  for (const dt of trades) {
    const level = (dt.trade.risk_level ?? "MEDIUM") as keyof typeof buckets;
    if (level in buckets) {
      buckets[level] += (dt.trade.estimated_price ?? 0) * 100;
    } else {
      buckets.MEDIUM += (dt.trade.estimated_price ?? 0) * 100;
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
