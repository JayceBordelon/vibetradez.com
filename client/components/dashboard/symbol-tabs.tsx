"use client";

import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
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

  return (
    <ToggleGroup type="single" value={activeSymbol} onValueChange={(v) => v && onSelect(v)} variant="outline" className="flex-wrap justify-start gap-1.5">
      {uniqueSymbols.map(({ symbol, pnl }) => (
        <ToggleGroupItem key={symbol} value={symbol} className="h-9 px-3 font-mono text-sm font-semibold">
          ${symbol}
          {pnl != null && <span className={cn("ml-1.5 text-xs", pnlColor(pnl))}>{fmtPnlInt(pnl)}</span>}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}
