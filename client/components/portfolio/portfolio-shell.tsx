"use client";

import { ArrowDownRight, ArrowRight, ArrowUpRight, Coins, Gauge, Wallet } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { DashboardSkeleton } from "@/components/layout/dashboard-skeleton";
import { Section } from "@/components/layout/section";
import { Stat, StatStrip } from "@/components/layout/stat-strip";
import { ClaudeLogo } from "@/components/ui/brand-icons";
import { useVisiblePoll } from "@/hooks/use-visible-poll";
import { api } from "@/lib/api";
import { fmtMoney, fmtPnlInt } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { EquityCurvePoint, PortfolioResponse } from "@/types/portfolio";

import { EquityCurveChart } from "./equity-curve-chart";

const REFRESH_SECONDS = 60;

/*
PortfolioShell is the v2 dashboard: one personal brokerage account managed
by the agent. It shows the live book (equity, cash, holdings), the equity
curve vs buy-and-hold SPY, and today's moves with the agent's rationale +
overall stance. This is a single-account view, not a multi-user product.
*/
export function PortfolioShell() {
  const [data, setData] = useState<PortfolioResponse | null>(null);
  const [curve, setCurve] = useState<EquityCurvePoint[]>([]);

  const load = useCallback(() => {
    api.getPortfolio().then(setData).catch(() => {});
    api
      .getEquityCurve()
      .then((r) => setCurve(r.points ?? []))
      .catch(() => {});
  }, []);

  useEffect(() => {
    load();
  }, [load]);
  // Always live: refresh whenever the tab is visible, regardless of market
  // hours, so the book/summary stay current even when the market is closed.
  useVisiblePoll(load, REFRESH_SECONDS * 1000, true);

  if (!data) {
    return (
      <div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
        <DashboardSkeleton />
      </div>
    );
  }

  if (!data.enabled) {
    return (
      <div className="mx-auto max-w-[1200px] px-4 py-6 sm:px-7">
        <div className="rounded-lg border border-border bg-muted/30 px-6 py-16 text-center">
          <h2 className="text-lg font-semibold">The portfolio manager isn&apos;t running yet</h2>
          <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
            Once it&apos;s switched on, this page shows the live book: holdings, cash, the equity curve against SPY, and every move the model makes with its reasoning.
          </p>
        </div>
      </div>
    );
  }

  const investedPct = data.equity > 0 ? ((data.equity - data.settled_cash - data.unsettled_cash) / data.equity) * 100 : 0;
  // positions can be a nil slice (JSON null) for an enabled but all-cash
  // account, so default it before any reduce/length access.
  const positions = data.positions ?? [];
  const totalUnrealized = positions.reduce((s, p) => s + p.unrealized_pnl, 0);
  const dayChangePct = (() => {
    if (curve.length < 2) return null;
    const prev = curve[curve.length - 2].account_equity;
    const last = curve[curve.length - 1].account_equity;
    return prev > 0 ? ((last - prev) / prev) * 100 : null;
  })();

  return (
    <div className="animate-in fade-in mx-auto max-w-[1200px] px-4 py-6 duration-300 sm:px-7">
      {data.drawdown_halted && (
        <div className="mb-6 rounded-md border border-red-border bg-red-bg px-4 py-3 text-sm text-red">
          Drawdown breaker tripped: the account is below its high-water floor, so the model is holding and de-risking only. No new buys.
        </div>
      )}

      <SummaryStrip equity={data.equity} settledCash={data.settled_cash} investedPct={investedPct} unrealized={totalUnrealized} dayChangePct={dayChangePct} />

      <Section title="Account vs SPY" subtitle="Return indexed to 100 at the start of the window. The dashed line is buy-and-hold SPY." className="mt-8 border-t border-border/40 pt-8">
        <EquityCurveChart points={curve} />
      </Section>

      {(data.summary?.trim() || data.stance.trim() || data.action_items?.trim()) && (
        <div className="mt-10 grid items-start gap-x-10 gap-y-6 border-t border-border/40 pt-8 sm:grid-cols-2">
          {(data.summary?.trim() || data.stance.trim()) && (
            <section className="border-l-2 border-claude/50 pl-4">
              <div className="flex items-center gap-2">
                <ClaudeLogo className="h-4 w-4" />
                <h2 className="text-[11px] font-semibold uppercase tracking-[0.16em] text-claude">Today&apos;s synopsis</h2>
              </div>
              <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-foreground/85">{data.summary?.trim() ? data.summary : data.stance}</p>
            </section>
          )}
          {data.action_items?.trim() && (
            <section className="border-l-2 border-claude/50 pl-4">
              <div className="flex items-center gap-2">
                <ClaudeLogo className="h-4 w-4" />
                <h2 className="text-[11px] font-semibold uppercase tracking-[0.16em] text-claude">{actionItemsLabel(data.date)}</h2>
              </div>
              <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-foreground/85">{data.action_items}</p>
            </section>
          )}
        </div>
      )}

      <a href={`/transcripts/${data.date}`} className="group mt-6 flex items-center gap-3 border-t border-border/50 pt-5">
        <ClaudeLogo className="h-5 w-5 shrink-0" />
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold text-foreground transition-colors group-hover:text-claude">Open today&apos;s full session</span>
          <span className="block text-xs text-muted-foreground">Every quote and chart Claude read, the reasoning in between, and every order it placed, tool call by tool call.</span>
        </span>
        <ArrowRight className="h-4 w-4 shrink-0 text-claude transition-transform group-hover:translate-x-0.5" />
      </a>

      <Section title="Explore" subtitle="The full book, the trade history, and today's moves live on their own pages." className="mt-10 border-t border-border/40 pt-8">
        <div className="divide-y divide-border/50 border-t border-border/50">
          <ExploreLink
            href="/holdings"
            title="Holdings"
            blurb={`${positions.length} open position${positions.length === 1 ? "" : "s"} the account holds right now.`}
          />
          <ExploreLink href="/closed" title="Closed trades" blurb="Every completed round trip, with realized P&L." />
        </div>
      </Section>
    </div>
  );
}

// actionItemsLabel frames the plan: after a Friday (or weekend) the next
// session is next week, otherwise tomorrow. Parsed from the date parts so it
// never shifts a day across timezones.
function actionItemsLabel(date: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date.trim());
  if (!m) return "Action items for next session";
  const day = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])).getDay();
  return day === 5 || day === 6 || day === 0 ? "Next week's action items" : "Tomorrow's action items";
}

function ExploreLink({ href, title, blurb }: { href: string; title: string; blurb: string }) {
  return (
    <Link href={href} className="group flex items-center justify-between gap-3 py-3.5">
      <span className="min-w-0">
        <span className="block font-semibold group-hover:underline">{title}</span>
        <span className="block text-xs text-muted-foreground">{blurb}</span>
      </span>
      <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
    </Link>
  );
}

function SummaryStrip({ equity, settledCash, investedPct, unrealized, dayChangePct }: { equity: number; settledCash: number; investedPct: number; unrealized: number; dayChangePct: number | null }) {
  const investedClamped = Math.min(100, Math.max(0, investedPct));
  return (
    <div>
      <StatStrip cols={4}>
        <Stat
          label="Account equity"
          value={fmtMoney(equity)}
          icon={Wallet}
          sub={dayChangePct !== null ? <span className={cn("font-semibold", dayChangePct >= 0 ? "text-green" : "text-red")}>{`${dayChangePct >= 0 ? "+" : ""}${dayChangePct.toFixed(2)}% today`}</span> : undefined}
        />
        <Stat
          label="Invested"
          value={`${investedPct.toFixed(0)}%`}
          icon={Gauge}
          sub={
            <span className="mt-1 block h-1.5 w-full max-w-[140px] overflow-hidden rounded-full bg-foreground/[0.08]">
              <span className="block h-full rounded-full bg-gradient-brand" style={{ width: `${investedClamped}%` }} />
            </span>
          }
        />
        <Stat label="Settled cash" value={fmtMoney(settledCash)} icon={Coins} sub={equity > 0 ? `${((settledCash / equity) * 100).toFixed(0)}% of the book` : "dry powder"} />
        <Stat
          label="Unrealized P&L"
          value={fmtPnlInt(unrealized)}
          tone={unrealized > 0 ? "positive" : unrealized < 0 ? "negative" : "neutral"}
          icon={unrealized >= 0 ? ArrowUpRight : ArrowDownRight}
          sub="open positions"
        />
      </StatStrip>
    </div>
  );
}

