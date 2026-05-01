"use client";

import { fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade } from "@/types/trade";

interface SymbolTabsProps {
  trades: DashboardTrade[];
  activeSymbol: string;
  onSelect: (symbol: string) => void;
}

interface SymbolEntry {
  symbol: string;
  rank: number;
  contractType: string;
  strike: number;
  score: number;
  pnl: number | null;
}

/*
SymbolTabs renders one pill per pick in a horizontally-scrollable row so
every trade is visible at a glance, not buried in a dropdown. Each pill
carries the rank, ticker, contract side + strike, conviction score, and
(when the trade has settled) live P&L. The active pill is auto-scrolled
into view so keyboard / programmatic switches don't strand the user.
*/
export function SymbolTabs({ trades, activeSymbol, onSelect }: SymbolTabsProps) {
  const symbolMap = new Map<string, SymbolEntry>();

  for (const dt of trades) {
    const sym = dt.trade.symbol;
    const existing = symbolMap.get(sym);
    const tradePnl = dt.summary ? (dt.summary.closing_price - dt.summary.entry_price) * 100 : null;

    if (!existing) {
      symbolMap.set(sym, {
        symbol: sym,
        rank: dt.trade.rank,
        contractType: dt.trade.contract_type,
        strike: dt.trade.strike_price,
        score: dt.trade.score,
        pnl: tradePnl,
      });
    } else {
      symbolMap.set(sym, {
        ...existing,
        rank: Math.min(existing.rank, dt.trade.rank),
        score: Math.max(existing.score, dt.trade.score),
        pnl: tradePnl !== null ? (existing.pnl ?? 0) + tradePnl : existing.pnl,
      });
    }
  }

  const entries = Array.from(symbolMap.values()).sort((a, b) => a.rank - b.rank);

  /*
  Auto-scroll the active pill into view when the selection changes.
  Using a callback ref instead of useEffect keeps the dependency
  surface minimal — the callback only runs when the *active* button
  mounts, which is exactly when we want to scroll.
  */
  const focusOnMount = (el: HTMLButtonElement | null) => {
    if (!el) return;
    el.scrollIntoView({ block: "nearest", inline: "center", behavior: "smooth" });
  };

  return (
    <div
      role="tablist"
      aria-label="Pick a trade to chart"
      className="-mx-1 flex min-w-0 flex-1 snap-x snap-mandatory items-center gap-1.5 overflow-x-auto px-4 py-1 [mask-image:linear-gradient(to_right,transparent_0,black_20px,black_calc(100%-20px),transparent_100%)] [scrollbar-width:none] sm:gap-2 [-webkit-mask-image:linear-gradient(to_right,transparent_0,black_20px,black_calc(100%-20px),transparent_100%)] [&::-webkit-scrollbar]:hidden"
    >
      {entries.map((entry) => {
        const isActive = entry.symbol === activeSymbol;
        return (
          <button
            key={entry.symbol}
            ref={isActive ? focusOnMount : null}
            type="button"
            role="tab"
            aria-selected={isActive}
            onClick={() => onSelect(entry.symbol)}
            className={cn(
              "group relative inline-flex shrink-0 snap-start items-center gap-2 rounded-full px-3 py-1.5 text-left transition-all sm:px-3.5",
              isActive ? "lg-card text-foreground shadow-sm" : "lg-pill text-muted-foreground hover:text-foreground"
            )}
          >
            <span
              className={cn(
                "inline-flex h-5 min-w-[22px] items-center justify-center rounded-full px-1 font-mono text-[10px] font-bold tabular-nums",
                isActive ? "bg-foreground text-background" : "bg-foreground/10 text-foreground/70"
              )}
            >
              #{entry.rank}
            </span>
            <span className="font-mono text-sm font-semibold tracking-tight">${entry.symbol}</span>
            <span className={cn("hidden text-[10px] font-medium uppercase tracking-wider sm:inline", entry.contractType === "CALL" ? "text-green" : "text-red")}>
              {entry.contractType} ${entry.strike}
            </span>
            {entry.score > 0 && (
              <span className="hidden rounded-md border border-foreground/10 px-1 py-0.5 text-[10px] font-semibold tabular-nums text-muted-foreground sm:inline">{entry.score}/10</span>
            )}
            {entry.pnl !== null && <span className={cn("text-xs font-semibold tabular-nums", pnlColor(entry.pnl))}>{fmtPnlInt(entry.pnl)}</span>}
          </button>
        );
      })}
    </div>
  );
}
