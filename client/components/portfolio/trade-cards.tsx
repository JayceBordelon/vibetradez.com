import { ChevronRight } from "lucide-react";
import Link from "next/link";

import { fmtMoney, fmtPnlInt, fmtPrice, pnlColor } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { ClosedTrade, Holding, TradeKind } from "@/types/portfolio";

/*
Flowing list rows for the /holdings and /closed pages. Rather than a stack of
bordered cards, each position is a hairline-divided row so the lists read as
one continuous surface on the page. Options and equities keep their own
emphasis (a CALL/PUT side badge, strike/expiry/DTE for options; shares and
cost for equities). Each row is a navigation surface to its detail page.
*/

type Section = "holdings" | "closed";

// The detail view is opened on the list page itself via a ?trade=<uuid> query
// param rather than a nested route, so there is no separate trade URL to 404
// on. The uuid is the trade's stable id (unique across stocks and options).
export function tradeHref(section: Section, id: string): string {
  return `/${section}?trade=${encodeURIComponent(id)}`;
}

/*
KindChip is the single instrument badge. For an option it shows the contract
side (CALL green / PUT red), since "Option" plus a separate CALL/PUT badge was
redundant; for a stock it shows a neutral "Stock". Falls back to "Option" only
if a contract type somehow isn't present.
*/
export function KindChip({ kind, contractType }: { kind: TradeKind; contractType?: string }) {
  if (kind === "option" && contractType) {
    const isCall = contractType === "CALL";
    return <span className={cn("rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider", isCall ? "border-green-border text-green" : "border-red-border text-red")}>{contractType}</span>;
  }
  return <span className="rounded bg-foreground/[0.06] px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{kind === "option" ? "Option" : "Stock"}</span>;
}

function Row({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link href={href} className="group -mx-3 flex items-center justify-between gap-4 rounded-lg px-3 py-3.5 transition-colors hover:bg-foreground/[0.03]">
      {children}
    </Link>
  );
}

export function HoldingRow({ h }: { h: Holding }) {
  const pct = h.cost_basis > 0 ? (h.unrealized_pnl / h.cost_basis) * 100 : 0;
  const up = h.unrealized_pnl >= 0;
  const sub =
    h.kind === "option"
      ? [`${h.quantity} contracts`, h.strike ? `${fmtPrice(h.strike)} strike` : null, h.expiration ? `exp ${h.expiration}` : null, h.dte != null ? `${h.dte}d left` : null].filter(Boolean).join(" · ")
      : [`${h.quantity} shares`, h.quantity > 0 ? `avg ${fmtMoney(h.cost_basis / h.quantity)}` : null, h.opened_date ? `opened ${h.opened_date}` : null].filter(Boolean).join(" · ");
  return (
    <Row href={tradeHref("holdings", h.id)}>
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="truncate text-base font-semibold">{h.label}</span>
          <KindChip kind={h.kind} contractType={h.contract_type} />
        </div>
        <div className="mt-1 truncate text-xs text-muted-foreground">{sub}</div>
      </div>
      <div className="flex shrink-0 items-center gap-3">
        <div className="text-right">
          <div className="font-semibold tabular-nums">{fmtMoney(h.market_value)}</div>
          <div className={cn("text-sm font-medium tabular-nums", pnlColor(h.unrealized_pnl))}>
            {fmtPnlInt(h.unrealized_pnl)} ({up ? "+" : ""}
            {pct.toFixed(1)}%)
          </div>
        </div>
        <ChevronRight className="h-4 w-4 text-muted-foreground transition-colors group-hover:text-foreground" />
      </div>
    </Row>
  );
}

export function ClosedRow({ t }: { t: ClosedTrade }) {
  const win = t.realized_pnl >= 0;
  const sub = [`${t.opened_date} to ${t.closed_date}`, `${t.hold_days}d held`, `entry ${fmtMoney(t.entry_price)}`, `exit ${fmtMoney(t.exit_price)}`].join(" · ");
  return (
    <Row href={tradeHref("closed", t.id)}>
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="truncate text-base font-semibold">{t.label}</span>
          <KindChip kind={t.kind} contractType={t.contract_type} />
          <span className={cn("rounded-full px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider", win ? "bg-green-bg text-green" : "bg-red-bg text-red")}>{win ? "Win" : "Loss"}</span>
        </div>
        <div className="mt-1 truncate text-xs text-muted-foreground">{sub}</div>
      </div>
      <div className="flex shrink-0 items-center gap-3">
        <div className="text-right">
          <div className={cn("font-bold tabular-nums", pnlColor(t.realized_pnl))}>{fmtPnlInt(t.realized_pnl)}</div>
          <div className={cn("text-sm font-semibold tabular-nums", pnlColor(t.realized_pnl))}>
            {win ? "+" : ""}
            {t.realized_pct.toFixed(1)}%
          </div>
        </div>
        <ChevronRight className="h-4 w-4 text-muted-foreground transition-colors group-hover:text-foreground" />
      </div>
    </Row>
  );
}
