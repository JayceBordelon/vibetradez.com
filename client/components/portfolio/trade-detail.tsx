"use client";

import { ArrowLeft, ArrowUpRight } from "lucide-react";
import Link from "next/link";

import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { AnimatedNumber } from "@/components/ui/animated-number";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { dayParts } from "@/lib/day-split";
import { fmtMoney, fmtPrice, pnlColor } from "@/lib/format";
import { repriceHoldings } from "@/lib/live-pricing";
import { cn } from "@/lib/utils";
import type { ClosedTrade, Holding } from "@/types/portfolio";
import { KindChip } from "./trade-cards";
import { TradeHistoryChart } from "./trade-history-chart";

/*
Detail bodies for a single open holding or closed trade, echoing the v1
trade-detail layout: a back link, a header with the instrument and its
badges, a strip of the headline numbers, the position/contract specifics,
and the model's reasoning with a link to the session transcript.
*/

function BackLink({ href, label }: { href: string; label: string }) {
  return (
    <Link href={href} className="mb-6 inline-flex min-h-9 items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground">
      <ArrowLeft className="h-4 w-4" />
      {label}
    </Link>
  );
}

/*
The model's reasoning, rendered in Claude's clay so it reads as Claude's
own voice. A soft tinted block with a left accent rule — one of the few
intentional containers, kept minimal (no hard border or drop shadow). When
a session is linked, the whole block is the link and opens the transcript.
*/
function Reasoning({ title, body, transcriptDate, transcriptLabel }: { title: string; body: string; transcriptDate?: string; transcriptLabel?: string }) {
  const base = "block rounded-r-xl border-l-2 border-claude/40 bg-claude-light px-5 py-4";
  const inner = (
    <>
      <div className="flex items-center gap-2">
        <ClaudeLogo className="h-4 w-4" />
        <span className="text-[11px] font-semibold uppercase tracking-wider text-claude">{title}</span>
      </div>
      <p className="mt-2 max-w-2xl text-sm leading-relaxed text-foreground/85">{body}</p>
      {transcriptDate && transcriptLabel && (
        <span className="mt-4 inline-flex items-center gap-2 text-sm font-semibold text-claude underline decoration-claude/30 underline-offset-4 transition-colors group-hover:decoration-claude">
          <ClaudeLogo className="h-4 w-4" />
          {transcriptLabel}
          <ArrowUpRight className="h-3.5 w-3.5" />
        </span>
      )}
    </>
  );

  if (transcriptDate) {
    return (
      <a href={`/transcripts/${transcriptDate}`} target="_blank" rel="noopener noreferrer" className={cn(base, "group transition-colors hover:bg-claude-light/70")}>
        {inner}
      </a>
    );
  }
  return <div className={base}>{inner}</div>;
}

// Facts is a borderless, hairline-divided definition list so the trade
// specifics flow with the page instead of sitting in a boxed card.
function Facts({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h2 className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">{title}</h2>
      <dl className="mt-1 divide-y divide-border/50 border-t border-border/50">{children}</dl>
    </section>
  );
}

function Fact({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-2.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="truncate text-right font-medium tabular-nums">{value}</span>
    </div>
  );
}

export function HoldingDetail({ h: initial }: { h: Holding }) {
  // The streamed quote for this position's symbol re-prices the headline
  // numbers tick by tick; without a tick yet the server values stand.
  const quotes = useLiveQuotes(true);
  const h = repriceHoldings([initial], quotes)[0];
  const pct = h.cost_basis > 0 ? (h.unrealized_pnl / h.cost_basis) * 100 : 0;
  const tone = h.unrealized_pnl > 0 ? "positive" : h.unrealized_pnl < 0 ? "negative" : "neutral";
  return (
    <div className="animate-soft-rise mx-auto min-w-0 max-w-[1100px] px-4 py-8 sm:px-7">
      <BackLink href="/holdings" label="Back to holdings" />

      <div className="flex flex-wrap items-center gap-2">
        <KindChip kind={h.kind} contractType={h.contract_type} />
        {/* Neutral on purpose: red/green are reserved for P&L direction
            everywhere else, and an "Open" badge in red read as a loss. */}
        <span className="rounded-full bg-secondary px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-secondary-foreground">Open</span>
      </div>
      <h1 className="mt-2.5 font-display text-2xl font-bold tracking-tight sm:text-3xl">{h.label}</h1>
      {h.opened_date && <p className="mt-1 text-sm text-muted-foreground">Opened {h.opened_date}</p>}

      <div className="mt-6">
        <StatStrip cols={4}>
          <Stat
            label="Market value"
            value={<AnimatedNumber value={h.market_value} kind="money" />}
            sub={(() => {
              const parts = dayParts(h);
              if (parts.day !== null) {
                return (
                  <span className={cn("font-semibold", pnlColor(parts.day))}>
                    <AnimatedNumber value={parts.day} kind="pnlInt" /> day
                  </span>
                );
              }
              if (parts.today === null) return undefined;
              return (
                <span className="font-semibold">
                  <span className={pnlColor(parts.today)}>
                    <AnimatedNumber value={parts.today} kind="pnlInt" /> today
                  </span>
                  <span className="text-muted-foreground"> · </span>
                  <span className={(parts.overnight ?? 0) === 0 ? "text-muted-foreground" : pnlColor(parts.overnight ?? 0)}>
                    <AnimatedNumber value={parts.overnight ?? 0} kind="pnlInt" /> overnight
                  </span>
                </span>
              );
            })()}
          />
          <Stat label="Cost basis" value={<AnimatedNumber value={h.cost_basis} kind="money" />} />
          <Stat label="Unrealized P&L" value={<AnimatedNumber value={h.unrealized_pnl} kind="pnlInt" />} tone={tone} sub={<AnimatedNumber value={pct} kind="pctSigned1" />} />
          {h.kind === "option" ? <Stat label="Days to expiry" value={h.dte != null ? `${h.dte}d` : "-"} /> : <Stat label="Shares" value={`${h.quantity}`} />}
        </StatStrip>
      </div>

      <div className="mt-8">
        <h2 className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">{h.kind === "option" ? "Contract value & underlying since open" : "Price since open"}</h2>
        <div className="mt-4">
          <TradeHistoryChart
            kind={h.kind}
            symbol={h.symbol}
            underlying={h.underlying}
            openedDate={h.opened_date}
            strike={h.strike}
            entryPrice={h.kind === "option" ? (h.quantity > 0 ? h.cost_basis / (h.quantity * 100) : undefined) : h.quantity > 0 ? h.cost_basis / h.quantity : undefined}
          />
        </div>
      </div>

      <div className="mt-8 grid items-start gap-x-10 gap-y-6 sm:grid-cols-2">
        <Facts title="Position">
          {h.kind === "option" ? (
            <>
              <Fact label="Contracts" value={`${h.quantity}`} />
              <Fact label="Strike" value={h.strike ? fmtPrice(h.strike) : "-"} />
              <Fact label="Contract" value={h.contract_type || "-"} />
              <Fact label="Expiration" value={h.expiration || "-"} />
              <Fact label="Avg premium" value={h.quantity > 0 ? fmtMoney(h.cost_basis / (h.quantity * 100)) : "-"} />
            </>
          ) : (
            <>
              <Fact label="Shares" value={`${h.quantity}`} />
              <Fact label="Avg cost" value={h.quantity > 0 ? fmtMoney(h.cost_basis / h.quantity) : "-"} />
              <Fact label="Cost basis" value={fmtMoney(h.cost_basis)} />
              <Fact label="Market value" value={fmtMoney(h.market_value)} />
            </>
          )}
          <Fact label="Underlying" value={h.underlying} />
        </Facts>

        {h.open_rationale && <Reasoning title="Why the model opened it" body={h.open_rationale} transcriptDate={h.opened_date} transcriptLabel="View the entry session" />}
      </div>
    </div>
  );
}

export function ClosedDetail({ t }: { t: ClosedTrade }) {
  // Win = strictly profitable; a $0 scratch is a loss (paid the spread),
  // matching the win-rate stats on the dashboard and /closed.
  const win = t.realized_pnl > 0;
  const tone = win ? "positive" : "negative";
  return (
    <div className="animate-soft-rise mx-auto min-w-0 max-w-[1100px] px-4 py-8 sm:px-7">
      <BackLink href="/closed" label="Back to closed trades" />

      <div className="flex flex-wrap items-center gap-2">
        <KindChip kind={t.kind} contractType={t.contract_type} />
        <span className={cn("rounded-full px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider", win ? "bg-green-bg text-green" : "bg-red-bg text-red")}>{win ? "Win" : "Loss"}</span>
      </div>
      <h1 className="mt-2.5 font-display text-2xl font-bold tracking-tight sm:text-3xl">{t.label}</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Held {t.opened_date} to {t.closed_date} ({t.hold_days} days)
      </p>

      <div className="mt-6">
        <StatStrip cols={4}>
          <Stat label="Realized P&L" value={<AnimatedNumber value={t.realized_pnl} kind="pnlInt" />} tone={tone} />
          <Stat label="Return" value={<AnimatedNumber value={t.realized_pct} kind="pctSigned1" />} tone={tone} />
          <Stat label="Entry" value={t.entry_price > 0 ? <AnimatedNumber value={t.entry_price} kind="money" /> : "-"} />
          <Stat label="Exit" value={t.exit_price > 0 ? <AnimatedNumber value={t.exit_price} kind="money" /> : "-"} />
        </StatStrip>
      </div>

      <div className="mt-8">
        <h2 className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">{t.kind === "option" ? "Contract value & underlying over the hold" : "Price over the hold"}</h2>
        <div className="mt-4">
          <TradeHistoryChart kind={t.kind} symbol={t.symbol} underlying={t.underlying} openedDate={t.opened_date} closedDate={t.closed_date} strike={t.strike} entryPrice={t.entry_price > 0 ? t.entry_price : undefined} />
        </div>
      </div>

      <div className="mt-8 grid items-start gap-x-10 gap-y-6 sm:grid-cols-2">
        <Facts title="Trade">
          <Fact label={t.kind === "option" ? "Contracts" : "Shares"} value={`${t.quantity}`} />
          {t.kind === "option" && <Fact label="Strike" value={t.strike ? fmtPrice(t.strike) : "-"} />}
          {t.kind === "option" && <Fact label="Expiration" value={t.expiration || "-"} />}
          <Fact label="Cost basis" value={fmtMoney(t.cost_basis)} />
          <Fact label="Proceeds" value={fmtMoney(t.proceeds)} />
          <Fact label="Hold time" value={`${t.hold_days} days`} />
          <Fact label="Underlying" value={t.underlying} />
        </Facts>

        <div className="space-y-6">
          {t.open_rationale && <Reasoning title="Why the model opened it" body={t.open_rationale} transcriptDate={t.opened_date} transcriptLabel="View the opening session" />}
          {t.close_rationale && <Reasoning title="Why the model closed it" body={t.close_rationale} transcriptDate={t.closed_date} transcriptLabel="View the closing session" />}
        </div>
      </div>
    </div>
  );
}
