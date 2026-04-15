"use client";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { fmtPnlInt, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { DashboardTrade } from "@/types/trade";

interface SymbolTabsProps {
  trades: DashboardTrade[];
  activeSymbol: string;
  onSelect: (symbol: string) => void;
}

export function SymbolTabs({ trades, activeSymbol, onSelect }: SymbolTabsProps) {
  const symbolMap = new Map<string, { symbol: string; pnl: number | null }>();

  for (const dt of trades) {
    const sym = dt.trade.symbol;
    const existing = symbolMap.get(sym);
    const tradePnl = dt.summary ? (dt.summary.closing_price - dt.summary.entry_price) * 100 : null;

    if (!existing) {
      symbolMap.set(sym, { symbol: sym, pnl: tradePnl });
    } else if (tradePnl !== null) {
      symbolMap.set(sym, {
        symbol: sym,
        pnl: (existing.pnl ?? 0) + tradePnl,
      });
    }
  }

  const uniqueSymbols = Array.from(symbolMap.values());
  const active = symbolMap.get(activeSymbol);

  return (
    <Select value={activeSymbol} onValueChange={onSelect}>
      <SelectTrigger className="w-full min-w-[160px] sm:w-[220px]" aria-label="Select ticker">
        <SelectValue placeholder="Select ticker">
          {active && (
            <span className="flex w-full items-center justify-between gap-3">
              <span className="font-mono text-sm font-semibold">${active.symbol}</span>
              {active.pnl != null && <span className={cn("text-xs tabular-nums", pnlColor(active.pnl))}>{fmtPnlInt(active.pnl)}</span>}
            </span>
          )}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {uniqueSymbols.map(({ symbol, pnl }) => (
          <SelectItem key={symbol} value={symbol}>
            <span className="flex w-full items-center justify-between gap-4">
              <span className="font-mono text-sm font-semibold">${symbol}</span>
              {pnl != null && <span className={cn("text-xs tabular-nums", pnlColor(pnl))}>{fmtPnlInt(pnl)}</span>}
            </span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
