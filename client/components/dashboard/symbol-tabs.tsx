"use client";

import { Sparkles } from "lucide-react";

import { ClaudeLogo, OpenAILogo } from "@/components/ui/brand-icons";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
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
  pickedByOpenAI: boolean;
  pickedByClaude: boolean;
  pnl: number | null;
}

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
        pickedByOpenAI: dt.trade.picked_by_openai,
        pickedByClaude: dt.trade.picked_by_claude,
        pnl: tradePnl,
      });
    } else {
      symbolMap.set(sym, {
        ...existing,
        rank: Math.min(existing.rank, dt.trade.rank),
        pickedByOpenAI: existing.pickedByOpenAI || dt.trade.picked_by_openai,
        pickedByClaude: existing.pickedByClaude || dt.trade.picked_by_claude,
        pnl: tradePnl !== null ? (existing.pnl ?? 0) + tradePnl : existing.pnl,
      });
    }
  }

  const entries = Array.from(symbolMap.values()).sort((a, b) => a.rank - b.rank);
  const active = symbolMap.get(activeSymbol);

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
        <Sparkles className="h-3 w-3" />
        <span>AI Pick</span>
      </div>
      <Select value={activeSymbol} onValueChange={onSelect}>
        <SelectTrigger className="h-auto w-full min-w-[200px] py-2 sm:w-[320px]" aria-label="Select an AI-picked trade to chart">
          <SelectValue placeholder="Select a pick">{active && <PickSummary entry={active} />}</SelectValue>
        </SelectTrigger>
        <SelectContent className="min-w-[260px]">
          {entries.map((entry) => (
            <SelectItem key={entry.symbol} value={entry.symbol} className="py-2">
              <PickSummary entry={entry} />
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function PickSummary({ entry }: { entry: SymbolEntry }) {
  return (
    <span className="flex w-full items-center justify-between gap-4 text-left">
      <span className="flex items-center gap-2">
        <span className="inline-flex h-5 min-w-[24px] items-center justify-center rounded bg-muted px-1.5 font-mono text-[10px] font-bold tracking-tight text-muted-foreground">#{entry.rank}</span>
        <span className="flex flex-col leading-tight">
          <span className="font-mono text-sm font-semibold">${entry.symbol}</span>
          <span className="font-mono text-[11px] text-muted-foreground">
            {entry.contractType} ${entry.strike}
          </span>
        </span>
      </span>
      <span className="flex items-center gap-2">
        <span className="flex items-center gap-1">
          {entry.pickedByOpenAI && <OpenAILogo className="h-3.5 w-3.5" />}
          {entry.pickedByClaude && <ClaudeLogo className="h-3.5 w-3.5" />}
        </span>
        {entry.pnl != null && <span className={cn("text-xs font-semibold tabular-nums", pnlColor(entry.pnl))}>{fmtPnlInt(entry.pnl)}</span>}
      </span>
    </span>
  );
}
