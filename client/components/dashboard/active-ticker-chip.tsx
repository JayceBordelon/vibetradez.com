"use client";

import { AnimatePresence, motion } from "motion/react";

import { fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade } from "@/types/trade";

interface ActiveTickerChipProps {
  trades: DashboardTrade[];
  activeSymbol: string;
}

/*
ActiveTickerChip renders next to the "Price Chart" section title and
mirrors whichever pick the user currently has selected in SymbolTabs.
On every selection change motion's AnimatePresence cross-fades the
previous chip down + out while the new chip slides up + in, giving
the title row a small "you selected this" cue without yanking layout
on either side. mode="popLayout" keeps the slot a fixed shape so
neighbouring elements (the section title, the timeframe row) don't
shift.
*/
export function ActiveTickerChip({ trades, activeSymbol }: ActiveTickerChipProps) {
  const dt = trades.find((t) => t.trade.symbol === activeSymbol);
  if (!dt) return null;

  const { trade, summary } = dt;
  const pnl = summary ? (summary.closing_price - summary.entry_price) * 100 : null;

  return (
    <div className="relative inline-flex h-9 min-w-0 items-center sm:h-8" aria-live="polite">
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.div
          key={activeSymbol}
          initial={{ opacity: 0, y: 12, filter: "blur(4px)" }}
          animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
          exit={{ opacity: 0, y: 12, filter: "blur(4px)" }}
          transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
          className="lg-pill inline-flex items-center gap-1.5 px-2.5 py-1 text-[11px] tabular-nums sm:gap-2 sm:px-3"
        >
          <span className="inline-flex h-5 min-w-[22px] items-center justify-center rounded-full bg-foreground px-1 font-mono text-[10px] font-bold text-background">#{trade.rank}</span>
          <span className="font-mono text-xs font-semibold text-foreground">${trade.symbol}</span>
          <span className={cn("hidden text-[10px] font-medium uppercase tracking-wider sm:inline", trade.contract_type === "CALL" ? "text-green" : "text-red")}>
            {trade.contract_type} ${trade.strike_price}
          </span>
          {pnl !== null && <span className={cn("text-[11px] font-semibold", pnlColor(pnl))}>{fmtPnlInt(pnl)}</span>}
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
