"use client";

import { ChevronDown, ChevronRight } from "lucide-react";
import { Fragment, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { ClaudeLogo, OpenAILogo } from "@/components/ui/brand-icons";
import { Card, CardContent } from "@/components/ui/card";
import {
	Collapsible,
	CollapsibleContent,
	CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { Metric } from "@/components/ui/metric";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import {
	calcBreakeven,
	calcMaxLoss,
	calcMoneyness,
	calcRiskReward,
	sentimentColor,
	sentimentLabel,
} from "@/lib/calculations";
import {
	fmt,
	fmtMoney,
	fmtPctDec,
	fmtPnlInt,
	pnlColor,
} from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade } from "@/types/trade";

interface TradeTableProps {
	trades: DashboardTrade[];
}

// ---- Computed values for a single DashboardTrade ----
interface RowComputed {
	hasSummary: boolean;
	pnl: number;
	pnlPct: number;
	stockMove: number;
	resultLabel: string;
	resultVariant: "default" | "destructive" | "outline" | "secondary";
	accentBorder: string;
	entry: string;
	close: string;
}

function computeRow(dt: DashboardTrade): RowComputed {
	const { trade, summary } = dt;
	const hasSummary = !!summary;
	const pnl = hasSummary
		? (summary.closing_price - summary.entry_price) * 100
		: 0;
	const pnlPct = hasSummary
		? ((summary.closing_price - summary.entry_price) /
				summary.entry_price) *
			100
		: 0;
	const stockMove = hasSummary
		? ((summary.stock_close - summary.stock_open) / summary.stock_open) *
			100
		: 0;

	const resultLabel = hasSummary
		? pnl > 0
			? "PROFIT"
			: pnl < 0
				? "LOSS"
				: "FLAT"
		: "OPEN";
	const resultVariant: RowComputed["resultVariant"] = hasSummary
		? pnl > 0
			? "default"
			: pnl < 0
				? "destructive"
				: "outline"
		: "secondary";

	const accentBorder = !hasSummary
		? "border-l-transparent"
		: pnlPct > 1
			? "border-l-green/40"
			: pnlPct < -1
				? "border-l-red/40"
				: "border-l-transparent";

	return {
		hasSummary,
		pnl,
		pnlPct,
		stockMove,
		resultLabel,
		resultVariant,
		accentBorder,
		entry: hasSummary
			? fmtMoney(summary.entry_price)
			: fmtMoney(trade.estimated_price),
		close: hasSummary ? fmtMoney(summary.closing_price) : "—",
	};
}

export function TradeTable({ trades }: TradeTableProps) {
	return (
		<div className="min-w-0">
			{/* Desktop table */}
			<div className="hidden overflow-x-auto md:block">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead className="w-10" />
							<TableHead className="w-10 text-center">#</TableHead>
							<TableHead>Trade</TableHead>
							<TableHead className="text-center">Scores</TableHead>
							<TableHead className="text-right">Entry</TableHead>
							<TableHead className="text-right">Close</TableHead>
							<TableHead className="text-right">Stock</TableHead>
							<TableHead className="text-right">P&amp;L</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{trades.map((dt) => (
							<DesktopTradeRow key={dt.trade.symbol} dt={dt} />
						))}
					</TableBody>
				</Table>
			</div>

			{/* Mobile cards */}
			<div className="space-y-3 md:hidden">
				{trades.map((dt) => (
					<TradeRowCard key={dt.trade.symbol} dt={dt} />
				))}
			</div>
		</div>
	);
}

// ---- Desktop row ----
// We cannot wrap <Collapsible> around multiple <TableRow>s (it renders a div
// which would break table structure), so we manage state locally and scope the
// Collapsible to a single cell in the detail row for smooth animation.
function DesktopTradeRow({ dt }: { dt: DashboardTrade }) {
	const [open, setOpen] = useState(false);
	const { trade } = dt;
	const moneyness = calcMoneyness(trade);
	const row = computeRow(dt);

	return (
		<Fragment>
			<TableRow
				className={cn(
					"cursor-pointer border-l-2 transition-colors hover:bg-muted/50",
					row.accentBorder,
				)}
				aria-expanded={open}
				onClick={() => setOpen((v) => !v)}
			>
				<TableCell className="w-10">
					{open ? (
						<ChevronDown className="h-4 w-4 text-muted-foreground" />
					) : (
						<ChevronRight className="h-4 w-4 text-muted-foreground" />
					)}
				</TableCell>
				<TableCell className="text-center text-sm tabular-nums text-muted-foreground">
					{trade.rank}
				</TableCell>
				<TableCell>
					<div className="flex flex-wrap items-center gap-1.5">
						<span className="font-mono text-sm font-semibold">
							${trade.symbol}
						</span>
						<Badge
							variant="outline"
							className={cn(
								trade.contract_type === "CALL"
									? "border-green-border text-green"
									: "border-red-border text-red",
							)}
						>
							{trade.contract_type}
						</Badge>
						<Badge variant={row.resultVariant}>
							{row.resultLabel}
						</Badge>
						<Badge variant={moneyness.variant}>
							{moneyness.label}
						</Badge>
					</div>
				</TableCell>
				<TableCell className="text-center text-xs tabular-nums">
					<ScorePill
						gpt={trade.gpt_score}
						claude={trade.claude_score}
					/>
				</TableCell>
				<TableCell className="text-right font-mono text-sm tabular-nums">
					{row.entry}
				</TableCell>
				<TableCell className="text-right font-mono text-sm tabular-nums">
					{row.close}
				</TableCell>
				<TableCell
					className={cn(
						"text-right font-mono text-sm tabular-nums",
						row.hasSummary
							? pnlColor(row.stockMove)
							: "text-muted-foreground",
					)}
				>
					{row.hasSummary ? fmtPctDec(row.stockMove) : "—"}
				</TableCell>
				<TableCell
					className={cn(
						"text-right text-base font-semibold tabular-nums",
						row.hasSummary
							? pnlColor(row.pnl)
							: "text-muted-foreground",
					)}
				>
					{row.hasSummary ? fmtPnlInt(row.pnl) : "—"}
				</TableCell>
			</TableRow>
			{open && (
				<TableRow className="hover:bg-transparent">
					<TableCell colSpan={8} className="bg-muted/30 p-0">
						<div className="animate-in fade-in fill-mode-backwards duration-150">
							<TradeDetail dt={dt} />
						</div>
					</TableCell>
				</TableRow>
			)}
		</Fragment>
	);
}

function ScorePill({ gpt, claude }: { gpt: number; claude: number }) {
	if (gpt === 0 && claude === 0) {
		return <span className="text-muted-foreground">—</span>;
	}
	return (
		<span className="inline-flex items-center gap-1.5 rounded-md border bg-muted/40 px-1.5 py-0.5 font-semibold">
			<OpenAILogo className="h-3 w-3" />
			<span>{gpt || "—"}</span>
			<span className="text-muted-foreground">·</span>
			<ClaudeLogo className="h-3 w-3" />
			<span>{claude || "—"}</span>
		</span>
	);
}

// ---- Mobile card ----
function TradeRowCard({ dt }: { dt: DashboardTrade }) {
	const [open, setOpen] = useState(false);
	const { trade } = dt;
	const moneyness = calcMoneyness(trade);
	const row = computeRow(dt);

	return (
		<Card
			className={cn(
				"animate-in fade-in fill-mode-backwards duration-200 border-l-2",
				row.accentBorder,
			)}
		>
			<CardContent className="space-y-3 p-4">
				<div className="flex flex-wrap items-center gap-1.5">
					<Badge variant="secondary">#{trade.rank}</Badge>
					<span className="font-mono text-base font-semibold">
						${trade.symbol}
					</span>
					<Badge
						variant="outline"
						className={cn(
							trade.contract_type === "CALL"
								? "border-green-border text-green"
								: "border-red-border text-red",
						)}
					>
						{trade.contract_type}
					</Badge>
					<Badge variant={row.resultVariant}>{row.resultLabel}</Badge>
					<Badge variant={moneyness.variant}>{moneyness.label}</Badge>
				</div>

				<div
					className={cn(
						"text-2xl font-semibold tabular-nums",
						row.hasSummary
							? pnlColor(row.pnl)
							: "text-muted-foreground",
					)}
				>
					{row.hasSummary ? fmtPnlInt(row.pnl) : "—"}
				</div>

				<div className="grid grid-cols-3 gap-3 text-sm">
					<Metric label="Entry" value={row.entry} />
					<Metric label="Close" value={row.close} />
					<Metric label="DTE" value={`${trade.dte}d`} />
				</div>

				<Collapsible open={open} onOpenChange={setOpen}>
					<CollapsibleTrigger asChild>
						<button
							type="button"
							className="group/card flex w-full items-center justify-between rounded-md border bg-muted/40 px-3 py-2 text-xs font-medium text-muted-foreground transition-colors hover:bg-muted"
						>
							<span>View full details</span>
							<ChevronDown className="h-3.5 w-3.5 transition-transform group-data-[state=open]/card:rotate-180" />
						</button>
					</CollapsibleTrigger>
					<CollapsibleContent className="overflow-hidden data-[state=closed]:animate-collapsible-up data-[state=open]:animate-collapsible-down">
						<div className="pt-3">
							<TradeDetail dt={dt} compact />
						</div>
					</CollapsibleContent>
				</Collapsible>
			</CardContent>
		</Card>
	);
}

// ---- Shared detail content ----
function TradeDetail({
	dt,
	compact = false,
}: {
	dt: DashboardTrade;
	compact?: boolean;
}) {
	const { trade, summary } = dt;
	const moneyness = calcMoneyness(trade);
	const breakeven = calcBreakeven(trade);
	const maxLoss = calcMaxLoss(trade);
	const riskReward = calcRiskReward(trade);

	return (
		<div className={cn("space-y-4", compact ? "" : "p-4")}>
			<div className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm sm:grid-cols-3 md:grid-cols-4">
				<Metric label="Strike" value={fmtMoney(trade.strike_price)} />
				<Metric
					label="Expiration"
					value={`${trade.expiration} (${trade.dte}d)`}
				/>
				<Metric label="Breakeven" value={fmtMoney(breakeven)} />
				<Metric
					label="Max Loss"
					value={
						<span className="text-sm font-semibold tabular-nums text-red">
							{fmtPnlInt(-maxLoss)}
						</span>
					}
				/>
				<Metric
					label="Risk / Reward"
					value={
						riskReward ? `1:${riskReward.toFixed(1)}` : "N/A"
					}
				/>
				<Metric label="Moneyness" value={moneyness.label} />
				<Metric
					label="Sentiment"
					value={
						<span
							className={cn(
								"text-sm font-semibold tabular-nums",
								sentimentColor(trade.sentiment_score),
							)}
						>
							{sentimentLabel(trade.sentiment_score)} (
							{fmt(trade.sentiment_score, 2)})
						</span>
					}
				/>
				<Metric
					label="Stock at Entry"
					value={fmtMoney(trade.current_price)}
				/>
			</div>

			{trade.catalyst && (
				<div className="rounded-md bg-amber-bg px-3 py-2 text-sm">
					<span className="font-semibold text-amber">Catalyst:</span>{" "}
					{trade.catalyst}
				</div>
			)}

			{trade.thesis && (
				<p className="text-sm leading-relaxed text-muted-foreground">
					<span className="font-semibold text-foreground">
						Thesis:
					</span>{" "}
					{trade.thesis}
				</p>
			)}

			{(trade.gpt_rationale || trade.claude_rationale) && (
				<div className="space-y-3 rounded-md border bg-card-elevated p-3">
					{trade.gpt_rationale && (
						<div>
							<div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
								<OpenAILogo className="h-3.5 w-3.5" />
								<span>OpenAI analysis</span>
								{trade.gpt_score > 0 && (
									<span className="rounded bg-background px-1.5 py-0.5 tabular-nums text-foreground">
										{trade.gpt_score}/10
									</span>
								)}
							</div>
							<p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
								{trade.gpt_rationale}
							</p>
						</div>
					)}
					{trade.claude_rationale && (
						<div>
							<div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
								<ClaudeLogo className="h-3.5 w-3.5" />
								<span>Claude analysis</span>
								{trade.claude_score > 0 && (
									<span className="rounded bg-background px-1.5 py-0.5 tabular-nums text-foreground">
										{trade.claude_score}/10
									</span>
								)}
							</div>
							<p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
								{trade.claude_rationale}
							</p>
						</div>
					)}
				</div>
			)}

			{summary && (
				<div className="rounded-md bg-card-elevated px-3 py-2 text-sm">
					<span className="font-semibold">EOD Result:</span> Entry{" "}
					<span className="font-mono tabular-nums">
						{fmtMoney(summary.entry_price)}
					</span>{" "}
					&rarr; Close{" "}
					<span className="font-mono tabular-nums">
						{fmtMoney(summary.closing_price)}
					</span>{" "}
					&middot; Stock{" "}
					<span className="font-mono tabular-nums">
						{fmtMoney(summary.stock_open)}
					</span>{" "}
					&rarr;{" "}
					<span className="font-mono tabular-nums">
						{fmtMoney(summary.stock_close)}
					</span>
					{summary.notes && (
						<span className="ml-1 text-muted-foreground">
							— {summary.notes}
						</span>
					)}
				</div>
			)}
		</div>
	);
}
