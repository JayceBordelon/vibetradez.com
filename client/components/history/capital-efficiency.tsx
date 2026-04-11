import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Metric } from "@/components/ui/metric";
import { fmtMoneyInt, fmtPctDec, fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";

interface CapitalEfficiencyProps {
  totalInvested: number;
  totalReturn: number;
  totalPnl: number;
  roc: number;
}

export function CapitalEfficiency({ totalInvested, totalReturn, totalPnl, roc }: CapitalEfficiencyProps) {
  return (
    <Card>
      <CardHeader>
        <h3 className="text-base font-semibold">Capital Efficiency</h3>
        <p className="text-sm text-muted-foreground">How efficiently capital was deployed across this period</p>
      </CardHeader>
      <CardContent className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Metric label="Total Deployed" value={fmtMoneyInt(totalInvested)} />
        <Metric label="Total Returned" value={fmtMoneyInt(totalReturn)} />
        <Metric label="Net P&L" value={<span className={cn("text-sm font-semibold tabular-nums", pnlColor(totalPnl))}>{fmtPnlInt(totalPnl)}</span>} />
        <Metric label="ROC" value={<span className={cn("text-sm font-semibold tabular-nums", pnlColor(roc))}>{fmtPctDec(roc)}</span>} />
      </CardContent>
    </Card>
  );
}
