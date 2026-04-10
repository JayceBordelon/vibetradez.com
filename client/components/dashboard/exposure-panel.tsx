import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Metric } from "@/components/ui/metric";
import { calcMaxGain } from "@/lib/calculations";
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
		const price =
			dt.summary?.entry_price ?? dt.trade.estimated_price ?? 0;
		return sum + price * 100;
	}, 0);

	const avgPremium = count > 0 ? totalExposure / count / 100 : 0;

	const avgDte =
		count > 0
			? trades.reduce((sum, dt) => sum + dt.trade.dte, 0) / count
			: 0;

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

	const totalMaxGain = hasSummaries
		? 0
		: trades.reduce((sum, dt) => sum + (calcMaxGain(dt.trade) ?? 0), 0);

	const rocColor =
		roc === null
			? ""
			: roc > 0
				? "text-green"
				: roc < 0
					? "text-red"
					: "text-muted-foreground";

	// For the deployed/returned bar: represent relative fill widths.
	// Deployed bar is always full (100%), Returned bar is totalReturned / totalExposure.
	const returnedPct =
		totalExposure > 0
			? Math.min(200, (totalReturned / totalExposure) * 100)
			: 0;
	const returnedBarColor =
		totalReturned >= totalExposure ? "bg-green" : "bg-red";

	return (
		<Card>
			<CardHeader className="pb-2">
				<h3 className="text-base font-semibold">Exposure Analysis</h3>
				<p className="text-sm text-muted-foreground">
					Capital deployed across {count} position{count === 1 ? "" : "s"}. For long options, max loss is limited to total premium paid.
				</p>
			</CardHeader>
			<CardContent className="space-y-5">
				<div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
					<Metric
						label="Capital at Risk"
						value={fmtMoneyInt(totalExposure)}
					/>
					<Metric label="Avg Premium" value={fmtMoney(avgPremium)} />
					<Metric label="Avg DTE" value={fmt(avgDte, 1)} />
					{hasSummaries && roc !== null ? (
						<Metric
							label="ROC"
							value={
								<span
									className={cn(
										"text-sm font-semibold tabular-nums",
										rocColor,
									)}
								>
									{fmtPctDec(roc)}
								</span>
							}
						/>
					) : (
						<Metric
							label="Max Gain Potential"
							value={fmtMoneyInt(totalMaxGain)}
						/>
					)}
				</div>

				{hasSummaries && totalExposure > 0 && (
					<div className="space-y-2">
						<div className="flex items-center justify-between text-xs text-muted-foreground">
							<span>Deployed vs Returned</span>
							<span className="tabular-nums">
								{fmtMoneyInt(totalExposure)} &rarr;{" "}
								{fmtMoneyInt(totalReturned)}
							</span>
						</div>
						<div className="space-y-1.5">
							<div className="h-2 w-full overflow-hidden rounded-full bg-muted">
								<div
									className="h-full rounded-full bg-amber"
									style={{ width: "100%" }}
								/>
							</div>
							<div className="h-2 w-full overflow-hidden rounded-full bg-muted">
								<div
									className={cn(
										"h-full rounded-full transition-all",
										returnedBarColor,
									)}
									style={{ width: `${returnedPct}%` }}
								/>
							</div>
						</div>
						<div className="flex justify-between text-[11px] text-muted-foreground">
							<span className="flex items-center gap-1.5">
								<span className="inline-block h-2 w-2 rounded-full bg-amber" />
								Deployed
							</span>
							<span className="flex items-center gap-1.5">
								<span
									className={cn(
										"inline-block h-2 w-2 rounded-full",
										returnedBarColor,
									)}
								/>
								Returned
							</span>
						</div>
					</div>
				)}
			</CardContent>
		</Card>
	);
}
