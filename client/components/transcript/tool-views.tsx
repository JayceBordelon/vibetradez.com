"use client";

import { CheckCircle2, FileText, Globe, Search, Square, XCircle } from "lucide-react";
import { Component, type ReactNode } from "react";

import { cn } from "@/lib/utils";

import {
  asArray,
  asDict,
  type Dict,
  fmtCompact,
  fmtCompactUsd,
  fmtDateShort,
  fmtPct,
  fmtQty,
  fmtSignedPct,
  fmtSignedUsd,
  fmtUsd,
  type ParsedResult,
  parseOcc,
  parseResult,
  pickBool,
  pickNum,
  pickRaw,
  pickStr,
  safeStringify,
  splitSentences,
} from "./format";

/*
ToolBody renders the purpose-built view for one tool call: order tickets for
the money movers, statement tables for the readers, terminal panes for the
sandbox, source cards for the web tools. Anything unrecognized or misshapen
lands in a generic key-value view, and a render error inside any view is
caught by the boundary and shown as the raw string, so a surprising
production payload can never take the page down.
*/
export function ToolBody({ name, input, rawResult }: { name?: string; input?: unknown; rawResult?: string }) {
  const inputDict = asDict(input);
  const r = parseResult(rawResult);
  return <BodyBoundary raw={rawResult}>{renderBody(name, inputDict, r)}</BodyBoundary>;
}

function renderBody(name: string | undefined, input: Dict | undefined, r: ParsedResult): ReactNode {
  switch (name) {
    case "buy_equity":
    case "sell_equity":
    case "buy_option":
    case "sell_option":
      return <OrderTicket name={name} input={input} r={r} />;
    case "cancel_order":
      return <CancelTicket input={input} r={r} />;
    case "hold":
      return <HoldRow input={input} r={r} />;
    case "write_summary":
      return <WriteSummaryCard input={input} r={r} />;
    case "web_search":
    case "web_fetch":
      return <WebSourceBody name={name} input={input} r={r} />;
    case "code_execution":
    case "bash_code_execution":
    case "text_editor_code_execution":
      return <TerminalBody name={name} input={input} r={r} />;
    default:
      break;
  }

  // Reading tools: refusals, missing results, and empty results share one
  // treatment, then each tool gets its bespoke view with a generic fallback.
  if (r.errorMessage !== undefined) return <ErrorNote message={r.errorMessage} />;
  if (!r.recorded) return <NoResultNote />;
  if (r.empty) return <EmptyResultNote />;

  let view: ReactNode = null;
  switch (name) {
    case "get_portfolio":
      view = renderPortfolio(r);
      break;
    case "get_stock_quotes":
      view = renderQuotes(r);
      break;
    case "get_option_chain":
      view = renderOptionChain(r);
      break;
    case "get_price_history":
      view = renderPriceHistory(r);
      break;
    case "get_fundamentals":
      view = renderFundamentals(r);
      break;
    case "get_recent_decisions":
      view = renderRecentDecisions(r);
      break;
    case "get_order_status":
      view = renderOrderStatus(r);
      break;
    default:
      view = null;
      break;
  }
  return view ?? <GenericResult r={r} />;
}

/*
BodyBoundary keeps a misbehaving renderer from crashing the transcript: if
anything throws while rendering a tool body, the raw result string is shown
instead. Production data never takes the page down.
*/
class BodyBoundary extends Component<{ raw?: string; children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    if (this.state.failed) {
      return <pre className="wrap-anywhere whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-foreground/70">{this.props.raw ?? "(no result recorded)"}</pre>;
    }
    return this.props.children;
  }
}

// ── shared atoms ─────────────────────────────────────────────────────────

function SlipLabel({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-muted-foreground", className)}>{children}</div>;
}

function Cell({ label, value, tone }: { label: string; value: ReactNode; tone?: "green" | "red" | "neutral" }) {
  return (
    <div className="flex flex-col">
      <span className="text-[10px] uppercase tracking-wider text-muted-foreground">{label}</span>
      <span className={cn("font-mono text-sm tabular-nums", tone === "green" ? "text-green" : tone === "red" ? "text-red" : "text-foreground/90")}>{value}</span>
    </div>
  );
}

function ErrorNote({ message }: { message: string }) {
  return (
    <div className="flex items-start gap-2 rounded-md border border-red-border bg-red-bg/60 px-3 py-2">
      <XCircle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-red" aria-hidden />
      <span className="sr-only">Tool returned an error</span>
      <p className="text-xs leading-relaxed text-red">{message}</p>
    </div>
  );
}

function NoResultNote() {
  return <p className="text-xs italic text-muted-foreground">No result was recorded for this call. The session capture closed before the tool returned.</p>;
}

function EmptyResultNote({ text }: { text?: string }) {
  return <p className="text-xs italic text-muted-foreground">{text ?? "The tool returned an empty result."}</p>;
}

function RationaleQuote({ text }: { text?: string }) {
  if (!text) return null;
  return <blockquote className="border-l-2 border-claude-border pl-3 font-serif text-[13.5px] italic leading-relaxed text-foreground/85">{text}</blockquote>;
}

function prim(v: unknown): string {
  if (v === null || v === undefined) return "null";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  const s = safeStringify(v);
  return s.length > 140 ? `${s.slice(0, 140)}…` : s;
}

/*
GenericResult is the graceful fallback for unknown tools and unexpected
shapes: flat key-value rows for objects, readable text for prose, pretty
JSON for arrays. The raw toggle on the slip always carries the verbatim
payload, so nothing is lost here.
*/
function GenericResult({ r }: { r: ParsedResult }) {
  if (r.obj) {
    const entries = Object.entries(r.obj);
    if (entries.length === 0) return <EmptyResultNote />;
    return (
      <dl className="grid gap-x-8 gap-y-0.5 sm:grid-cols-2">
        {entries.map(([k, v]) => (
          <div key={k} className="flex items-baseline justify-between gap-3 border-b border-border/40 py-1 last:border-0">
            <dt className="shrink-0 font-mono text-[11px] text-muted-foreground">{k}</dt>
            <dd className="min-w-0 truncate text-right font-mono text-xs tabular-nums text-foreground/85">{prim(v)}</dd>
          </div>
        ))}
      </dl>
    );
  }
  if (r.arr) {
    return <pre className="wrap-anywhere whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-foreground/75">{safeStringify(r.arr, 2)}</pre>;
  }
  return <p className="wrap-anywhere whitespace-pre-wrap text-[13px] leading-relaxed text-foreground/85">{r.raw}</p>;
}

// instrumentLabel renders a symbol for display: OCC option symbols decode to
// a compact contract line, everything else passes through.
function instrumentLabel(sym?: string): { main: string; detail?: string } {
  const occ = parseOcc(sym);
  if (occ) {
    return { main: `${occ.root} ${fmtQty(occ.strike)}${occ.type === "CALL" ? "C" : "P"}`, detail: `exp ${fmtDateShort(occ.expiration)}` };
  }
  return { main: sym ?? "" };
}

// ── get_portfolio: the book as a broker statement ────────────────────────

function renderPortfolio(r: ParsedResult): ReactNode {
  const o = r.obj;
  if (!o) return null;
  const equity = pickNum(o, "equity", "Equity");
  const cash = pickNum(o, "settled_cash", "SettledCash");
  const unsettled = pickNum(o, "unsettled_cash", "UnsettledCash");
  const hwm = pickNum(o, "high_water_mark", "HighWaterMark");
  const positions = asArray(pickRaw(o, "positions", "Positions"));
  if (equity === undefined && cash === undefined && positions.length === 0) return null;

  return (
    <div className="space-y-2.5">
      <div className="flex flex-wrap gap-x-6 gap-y-1.5">
        {equity !== undefined && <Cell label="Account equity" value={fmtUsd(equity)} />}
        {cash !== undefined && <Cell label="Settled cash" value={fmtUsd(cash)} />}
        {unsettled !== undefined && unsettled > 0 && <Cell label="Unsettled" value={fmtUsd(unsettled)} />}
        {hwm !== undefined && hwm > 0 && <Cell label="High-water mark" value={fmtUsd(hwm)} />}
      </div>
      {positions.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[320px] border-collapse text-xs">
            <thead>
              <tr className="border-b border-border/60 text-left text-[10px] uppercase tracking-wider text-muted-foreground">
                <th className="py-1.5 pr-2 font-semibold">Position</th>
                <th className="px-2 py-1.5 text-right font-semibold">Qty</th>
                <th className="px-2 py-1.5 text-right font-semibold">Value</th>
                <th className="py-1.5 pl-2 text-right font-semibold">Unrealized P&L</th>
              </tr>
            </thead>
            <tbody>
              {positions.map((pos, i) => {
                const p = asDict(pos);
                if (!p) return null;
                const sym = pickStr(p, "symbol", "Symbol") ?? "?";
                const asset = (pickStr(p, "asset_type", "AssetType") ?? "").toUpperCase();
                const qty = pickNum(p, "quantity", "Quantity");
                const mv = pickNum(p, "market_value", "MarkValue", "mark_value");
                const cb = pickNum(p, "cost_basis", "CostBasis");
                const pnl = pickNum(p, "unrealized_pnl") ?? (mv !== undefined && cb !== undefined ? mv - cb : undefined);
                const pnlPct = pnl !== undefined && cb !== undefined && cb > 0 ? (pnl / cb) * 100 : undefined;
                const label = instrumentLabel(sym);
                return (
                  <tr key={`${sym}-${i}`} className="border-b border-border/40 last:border-0">
                    <td className="py-1.5 pr-2">
                      <span className="font-mono font-medium text-foreground/90">{label.main}</span>
                      {asset && (
                        <span className="ml-1.5 rounded-sm border border-border/70 px-1 py-px align-middle text-[9px] uppercase tracking-wider text-muted-foreground">
                          {asset === "OPTION" ? "opt" : "eq"}
                        </span>
                      )}
                      {/* Contract detail drops to its own line on mobile so the
                          P&L column stays on screen at 390px. */}
                      {label.detail && <span className="block text-[10px] text-muted-foreground sm:ml-1.5 sm:inline">{label.detail}</span>}
                    </td>
                    <td className="px-2 py-1.5 text-right font-mono tabular-nums text-foreground/80">{qty !== undefined ? fmtQty(qty) : "n/a"}</td>
                    <td className="px-2 py-1.5 text-right font-mono tabular-nums text-foreground/90">{mv !== undefined ? fmtUsd(mv) : "n/a"}</td>
                    <td className={cn("py-1.5 pl-2 text-right font-mono tabular-nums", pnl === undefined ? "text-muted-foreground" : pnl >= 0 ? "text-green" : "text-red")}>
                      {pnl !== undefined ? fmtSignedUsd(pnl) : "n/a"}
                      {pnlPct !== undefined && <span className="ml-1 hidden text-[10px] opacity-80 sm:inline">({fmtSignedPct(pnlPct)})</span>}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      {positions.length === 0 && <p className="text-xs italic text-muted-foreground">No open positions. The account is all cash.</p>}
    </div>
  );
}

// ── get_stock_quotes: inline quote chips ─────────────────────────────────

function renderQuotes(r: ParsedResult): ReactNode {
  const o = r.obj;
  if (!o) return null;
  const entries = Object.entries(o)
    .map(([sym, v]) => ({ sym, q: asDict(v) }))
    .filter((e): e is { sym: string; q: Dict } => e.q !== undefined);
  if (entries.length === 0) return null;
  return (
    <ul className="flex flex-wrap gap-2">
      {entries.map(({ sym, q }) => {
        const last = pickNum(q, "last", "lastPrice", "mark");
        const bid = pickNum(q, "bid", "bidPrice");
        const ask = pickNum(q, "ask", "askPrice");
        const vol = pickNum(q, "totalVolume", "volume");
        const chgPct = pickNum(q, "netPercentChange", "markPercentChange");
        return (
          <li key={sym} className="min-w-[124px] rounded-md border border-border/60 bg-card-elevated/70 px-2.5 py-1.5">
            <div className="flex items-baseline justify-between gap-3">
              <span className="font-mono text-xs font-semibold text-foreground/90">{sym}</span>
              <span className="font-mono text-sm tabular-nums text-foreground">{last !== undefined ? fmtUsd(last) : "n/a"}</span>
            </div>
            <div className="mt-0.5 flex flex-wrap items-baseline gap-x-2 font-mono text-[10px] tabular-nums text-muted-foreground">
              {bid !== undefined && ask !== undefined && (
                <span>
                  {fmtQty(bid)} bid {fmtQty(ask)} ask
                </span>
              )}
              {vol !== undefined && <span>vol {fmtCompact(vol)}</span>}
              {chgPct !== undefined && <span className={chgPct >= 0 ? "text-green" : "text-red"}>{fmtSignedPct(chgPct)}</span>}
            </div>
          </li>
        );
      })}
    </ul>
  );
}

// ── get_option_chain: compact chain table ────────────────────────────────

function renderOptionChain(r: ParsedResult): ReactNode {
  const o = r.obj;
  if (!o) return null;
  const calls = asArray(pickRaw(o, "calls"));
  const puts = asArray(pickRaw(o, "puts"));
  const side = calls.length > 0 ? "Calls" : "Puts";
  const rows = (calls.length > 0 ? calls : puts).map(asDict).filter((d): d is Dict => d !== undefined);
  if (rows.length === 0) return null;
  const sym = pickStr(o, "symbol");
  const under = pickNum(o, "underlyingPrice", "underlying_price");
  const dte = pickNum(rows[0], "daysToExpiration");

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        {sym && <span className="font-mono text-sm font-semibold text-foreground/90">{sym}</span>}
        <span className="text-[11px] text-muted-foreground">{side}</span>
        {under !== undefined && <span className="font-mono text-[11px] tabular-nums text-muted-foreground">underlying {fmtUsd(under)}</span>}
        {dte !== undefined && <span className="font-mono text-[11px] tabular-nums text-muted-foreground">{dte}d to expiry</span>}
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[460px] border-collapse text-xs">
          <thead>
            <tr className="border-b border-border/60 text-[10px] uppercase tracking-wider text-muted-foreground">
              <th className="py-1.5 pr-2 text-left font-semibold">Strike</th>
              <th className="px-2 py-1.5 text-right font-semibold">Bid</th>
              <th className="px-2 py-1.5 text-right font-semibold">Ask</th>
              <th className="px-2 py-1.5 text-right font-semibold">Mark</th>
              <th className="px-2 py-1.5 text-right font-semibold">OI</th>
              <th className="px-2 py-1.5 text-right font-semibold">Vol</th>
              <th className="py-1.5 pl-2 text-right font-semibold">Delta</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((c, i) => {
              const strike = pickNum(c, "strikePrice", "strike");
              const bid = pickNum(c, "bid");
              const ask = pickNum(c, "ask");
              const mark = pickNum(c, "mark", "last");
              const oi = pickNum(c, "openInterest", "open_interest");
              const vol = pickNum(c, "totalVolume", "volume");
              const delta = pickNum(c, "delta");
              const itm = pickBool(c, "inTheMoney");
              const n = (v?: number, d = 2) => (v !== undefined ? v.toFixed(d) : "n/a");
              return (
                <tr key={`${strike}-${i}`} className="border-b border-border/40 last:border-0 font-mono tabular-nums">
                  <td className="py-1.5 pr-2 text-left font-medium text-foreground/90">
                    {strike !== undefined ? fmtQty(strike) : "n/a"}
                    {itm && <span className="ml-1.5 rounded-sm bg-green-bg px-1 py-px text-[9px] font-sans uppercase tracking-wider text-green">itm</span>}
                  </td>
                  <td className="px-2 py-1.5 text-right text-foreground/80">{n(bid)}</td>
                  <td className="px-2 py-1.5 text-right text-foreground/80">{n(ask)}</td>
                  <td className="px-2 py-1.5 text-right text-foreground/90">{n(mark)}</td>
                  <td className="px-2 py-1.5 text-right text-foreground/70">{oi !== undefined ? fmtCompact(oi) : "n/a"}</td>
                  <td className="px-2 py-1.5 text-right text-foreground/70">{vol !== undefined ? fmtCompact(vol) : "n/a"}</td>
                  <td className="py-1.5 pl-2 text-right text-foreground/80">{n(delta)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ── get_price_history: trend stat row ────────────────────────────────────

function SmaBadge({ period, value, last }: { period: number; value?: number; last?: number }) {
  if (value === undefined || value === 0) return null;
  const above = last !== undefined ? last >= value : undefined;
  return (
    <span
      className={cn(
        "inline-flex items-baseline gap-1.5 rounded-md border px-2 py-1",
        above === undefined ? "border-border/60" : above ? "border-green-border/70 bg-green-light" : "border-red-border/70 bg-red-light"
      )}
    >
      <span className="text-[10px] uppercase tracking-wider text-muted-foreground">SMA {period}</span>
      <span className="font-mono text-xs tabular-nums text-foreground/90">{fmtUsd(value)}</span>
      {above !== undefined && <span className={cn("text-[10px] font-medium", above ? "text-green" : "text-red")}>{above ? "above" : "below"}</span>}
    </span>
  );
}

function renderPriceHistory(r: ParsedResult): ReactNode {
  const o = r.obj;
  if (!o) return null;
  const last = pickNum(o, "last_close");
  if (last === undefined) return null;
  const sym = pickStr(o, "symbol");
  const hi = pickNum(o, "high_52w");
  const lo = pickNum(o, "low_52w");
  const fromHigh = pickNum(o, "pct_from_52w_high");
  const r1m = pickNum(o, "return_1m_pct");
  const r3m = pickNum(o, "return_3m_pct");
  const vol = pickNum(o, "annualized_vol_20d_pct");
  const rangePct = hi !== undefined && lo !== undefined && hi > lo ? Math.min(100, Math.max(0, ((last - lo) / (hi - lo)) * 100)) : undefined;

  return (
    <div className="space-y-2.5">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        {sym && <span className="font-mono text-sm font-semibold text-foreground/90">{sym}</span>}
        <span className="font-mono text-sm tabular-nums text-foreground">{fmtUsd(last)}</span>
        <span className="text-[11px] text-muted-foreground">last close</span>
      </div>
      <div className="flex flex-wrap gap-1.5">
        <SmaBadge period={20} value={pickNum(o, "sma_20")} last={last} />
        <SmaBadge period={50} value={pickNum(o, "sma_50")} last={last} />
        <SmaBadge period={200} value={pickNum(o, "sma_200")} last={last} />
      </div>
      <div className="flex flex-wrap gap-x-6 gap-y-1.5">
        {fromHigh !== undefined && <Cell label="From 52w high" value={fmtSignedPct(fromHigh)} tone={fromHigh >= 0 ? "green" : "red"} />}
        {r1m !== undefined && <Cell label="1m return" value={fmtSignedPct(r1m)} tone={r1m >= 0 ? "green" : "red"} />}
        {r3m !== undefined && <Cell label="3m return" value={fmtSignedPct(r3m)} tone={r3m >= 0 ? "green" : "red"} />}
        {vol !== undefined && <Cell label="20d vol (ann.)" value={fmtPct(vol)} />}
      </div>
      {rangePct !== undefined && (
        <div className="max-w-md">
          <div className="relative h-1 rounded-full bg-muted">
            <span className="absolute top-1/2 h-3 w-1.5 -translate-y-1/2 rounded-full bg-foreground/80" style={{ left: `calc(${rangePct}% - 3px)` }} aria-hidden />
          </div>
          <div className="mt-1 flex justify-between font-mono text-[10px] tabular-nums text-muted-foreground">
            <span>52w low {lo !== undefined ? fmtUsd(lo) : ""}</span>
            <span>52w high {hi !== undefined ? fmtUsd(hi) : ""}</span>
          </div>
        </div>
      )}
    </div>
  );
}

// ── get_fundamentals: key-value stat grid ────────────────────────────────

function renderFundamentals(r: ParsedResult): ReactNode {
  const o = r.obj;
  if (!o) return null;
  const stats: { label: string; value: string }[] = [];
  const cap = pickNum(o, "market_cap", "MarketCap");
  if (cap !== undefined) stats.push({ label: "Market cap", value: fmtCompactUsd(cap) });
  const pe = pickNum(o, "pe_ratio", "pe");
  if (pe !== undefined) stats.push({ label: "P/E", value: pe.toFixed(1) });
  const eps = pickNum(o, "eps_ttm", "eps");
  if (eps !== undefined) stats.push({ label: "EPS (TTM)", value: fmtUsd(eps) });
  const beta = pickNum(o, "beta");
  if (beta !== undefined) stats.push({ label: "Beta", value: beta.toFixed(2) });
  const dy = pickNum(o, "dividend_yield");
  if (dy !== undefined && dy > 0) stats.push({ label: "Dividend yield", value: fmtPct(dy, 2) });
  const next = pickStr(o, "next_earnings_date", "next_earnings");
  if (next) stats.push({ label: "Next earnings", value: fmtDateShort(next) ?? next });
  const hi = pickNum(o, "high_52");
  const lo = pickNum(o, "low_52");
  if (hi !== undefined && lo !== undefined) stats.push({ label: "52w range", value: `${fmtUsd(lo)} to ${fmtUsd(hi)}` });
  const adv = pickNum(o, "avg_10_day_volume");
  if (adv !== undefined) stats.push({ label: "Avg 10d volume", value: fmtCompact(adv) });
  if (stats.length === 0) return null;
  const sym = pickStr(o, "symbol");

  return (
    <div className="space-y-2">
      {sym && <span className="font-mono text-sm font-semibold text-foreground/90">{sym}</span>}
      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-4">
        {stats.map((s) => (
          <div key={s.label} className="rounded-md border border-border/50 bg-card-elevated/50 px-2.5 py-1.5">
            <dt className="text-[10px] uppercase tracking-wider text-muted-foreground">{s.label}</dt>
            <dd className="mt-0.5 font-mono text-xs tabular-nums text-foreground/90">{s.value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

// ── get_recent_decisions: mini decision ledger ───────────────────────────

function actionDot(action: string): string {
  if (action.startsWith("buy")) return "bg-green";
  if (action.startsWith("sell")) return "bg-red";
  if (action === "cancel_order") return "bg-amber";
  return "bg-muted-foreground/50";
}

function renderRecentDecisions(r: ParsedResult): ReactNode {
  const o = r.obj;
  if (!o) return null;
  const decisions = asArray(pickRaw(o, "decisions", "Decisions")).map(asDict).filter((d): d is Dict => d !== undefined);
  const stancesRaw = asArray(pickRaw(o, "stances", "Stances"));
  const priorSynopsis = pickStr(o, "prior_synopsis");
  const priorPlan = pickStr(o, "prior_action_items");
  if (decisions.length === 0 && stancesRaw.length === 0 && !priorSynopsis) return null;

  const stances = stancesRaw
    .map((s) => {
      if (typeof s === "string") return { date: undefined as string | undefined, stance: s };
      const d = asDict(s);
      return { date: pickStr(d, "date", "Date"), stance: pickStr(d, "stance", "Stance") };
    })
    .filter((s) => s.stance);

  return (
    <div className="space-y-3">
      {decisions.length > 0 && (
        <ul className="space-y-1.5">
          {decisions.map((d, i) => {
            const action = pickStr(d, "action", "Action") ?? "hold";
            const sym = pickStr(d, "symbol", "Symbol");
            const date = pickStr(d, "date", "Date");
            const notional = pickNum(d, "notional", "Notional");
            const rationale = pickStr(d, "rationale", "Rationale");
            const label = instrumentLabel(sym);
            return (
              <li key={`${action}-${i}`} className="flex items-start gap-2.5">
                <span className={cn("mt-1.5 h-2 w-2 shrink-0 rounded-full", actionDot(action))} aria-hidden />
                <div className="min-w-0">
                  <div className="flex flex-wrap items-baseline gap-x-2">
                    <span className="font-mono text-xs font-medium text-foreground/90">{action}</span>
                    {label.main && <span className="font-mono text-xs text-foreground/80">{label.main}</span>}
                    {notional !== undefined && notional > 0 && <span className="font-mono text-[11px] tabular-nums text-muted-foreground">{fmtUsd(notional)}</span>}
                    {date && <span className="text-[10px] text-muted-foreground">{fmtDateShort(date) ?? date}</span>}
                  </div>
                  {rationale && <p className="text-[11px] leading-snug text-muted-foreground">{rationale}</p>}
                </div>
              </li>
            );
          })}
        </ul>
      )}
      {stances.length > 0 && (
        <div>
          <SlipLabel className="mb-1">Recent stances</SlipLabel>
          <ul className="space-y-1">
            {stances.map((s, i) => (
              <li key={`${s.date}-${i}`} className="font-serif text-[12.5px] italic leading-snug text-muted-foreground">
                {s.date && <span className="mr-1.5 font-sans text-[10px] not-italic">{s.date}</span>}
                {s.stance}
              </li>
            ))}
          </ul>
        </div>
      )}
      {(priorSynopsis || priorPlan) && (
        <div className="rounded-md border border-border/50 bg-muted/30 px-3 py-2">
          <SlipLabel className="mb-1">Carried over from the prior session</SlipLabel>
          {priorSynopsis && <p className="font-serif text-[12.5px] italic leading-relaxed text-foreground/80">{priorSynopsis}</p>}
          {priorPlan && <p className="mt-1 text-[11px] leading-relaxed text-muted-foreground">Plan: {priorPlan}</p>}
        </div>
      )}
    </div>
  );
}

// ── get_order_status ─────────────────────────────────────────────────────

function statusPillClass(status: string): string {
  const s = status.toUpperCase();
  if (s === "FILLED") return "border-green-border bg-green-bg text-green";
  if (["WORKING", "PENDING_ACTIVATION", "ACCEPTED", "QUEUED", "OPEN", "AWAITING_PARENT_ORDER"].includes(s)) return "border-amber/40 bg-amber-bg text-amber";
  if (["CANCELED", "CANCELLED", "REJECTED", "EXPIRED", "REPLACED"].includes(s)) return "border-red-border bg-red-bg text-red";
  return "border-border bg-muted text-muted-foreground";
}

function renderOrderStatus(r: ParsedResult): ReactNode {
  const o = r.obj;
  if (!o) return null;
  const orderId = pickStr(o, "order_id", "OrderID");
  const status = pickStr(o, "status", "Status");
  if (!orderId && !status) return null;
  const qty = pickNum(o, "quantity", "Quantity");
  const filledQty = pickNum(o, "filled_quantity", "FilledQuantity");
  const limit = pickNum(o, "limit_price", "LimitPrice");
  const fill = pickNum(o, "fill_price", "FillPrice");

  return (
    <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
      {orderId && <code className="rounded-sm bg-muted px-1.5 py-0.5 font-mono text-[11px] text-foreground/85">{orderId}</code>}
      {status && (
        <span className={cn("inline-flex items-center rounded-full border px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider", statusPillClass(status))}>
          {status}
        </span>
      )}
      {filledQty !== undefined && (
        <Cell label="Filled" value={qty !== undefined ? `${fmtQty(filledQty)} of ${fmtQty(qty)}` : fmtQty(filledQty)} />
      )}
      {limit !== undefined && limit > 0 && <Cell label="Limit" value={fmtUsd(limit)} />}
      {fill !== undefined && fill > 0 && <Cell label="Fill" value={fmtUsd(fill)} />}
    </div>
  );
}

// ── order tickets: buy / sell / cancel ───────────────────────────────────

function OrderTicket({ name, input, r }: { name: string; input?: Dict; r: ParsedResult }) {
  const isBuy = name.startsWith("buy_");
  const isOption = name.endsWith("_option");
  const refused = r.errorMessage;
  const ok = pickBool(r.obj, "ok") === true;

  const symbol = pickStr(input, "symbol", "occ_symbol") ?? pickStr(r.obj, "symbol") ?? "";
  const occ = isOption ? parseOcc(symbol) : undefined;
  const underlying = pickStr(input, "underlying") ?? occ?.root ?? symbol;
  const contractType = pickStr(input, "contract_type") ?? occ?.type;
  const strike = pickNum(input, "strike") ?? occ?.strike;
  const expiration = pickStr(input, "expiration") ?? occ?.expiration;
  const qty = pickNum(input, "quantity", "contracts") ?? pickNum(r.obj, "quantity");
  const limit = pickNum(input, "limit_price") ?? pickNum(r.obj, "limit_price");
  const notional = pickNum(r.obj, "notional") ?? (qty !== undefined && limit !== undefined ? qty * limit * (isOption ? 100 : 1) : undefined);
  const orderId = pickStr(r.obj, "order_id");
  const cashLeft = pickNum(r.obj, "settled_cash_left");
  const rationale = pickStr(input, "rationale");
  const unit = isOption ? (qty === 1 ? "contract" : "contracts") : qty === 1 ? "share" : "shares";

  return (
    <div className={cn("flex overflow-hidden rounded-md border", refused ? "border-red-border/80 bg-red-light/60 dark:bg-red-light" : isBuy ? "border-green-border/60 bg-green-light dark:bg-green-light" : "border-red-border/50 bg-red-light dark:bg-red-light")}>
      <div className={cn("w-1 shrink-0", isBuy ? "bg-green" : "bg-red")} aria-hidden />
      <div className="min-w-0 flex-1 space-y-2 px-3 py-2.5 sm:px-4">
        <div className="flex flex-wrap items-center gap-2">
          <span
            className={cn(
              "inline-flex items-center rounded-full border px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider",
              isBuy ? "border-green-border bg-green-bg text-green" : "border-red-border bg-red-bg text-red"
            )}
          >
            {isBuy ? "Buy" : "Sell"}
          </span>
          <span className="rounded-sm border border-border/70 px-1.5 py-px font-mono text-[9px] uppercase tracking-wider text-muted-foreground">
            {isOption ? (contractType ?? "option") : "equity"}
          </span>
          {refused && (
            <span className="ml-auto inline-flex items-center gap-1 rounded-full border border-red-border bg-red-bg px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-red">
              <XCircle className="h-3 w-3" aria-hidden />
              Refused
            </span>
          )}
        </div>
        <div className="flex flex-wrap items-baseline gap-x-2.5 gap-y-0.5">
          <span className="font-mono text-base font-semibold tracking-tight text-foreground">{underlying || symbol || "order"}</span>
          {isOption && strike !== undefined && (
            <span className="font-mono text-sm tabular-nums text-foreground/85">
              {fmtQty(strike)} {contractType === "PUT" ? "put" : "call"}
            </span>
          )}
          {isOption && expiration && <span className="text-[11px] text-muted-foreground">exp {fmtDateShort(expiration) ?? expiration}</span>}
        </div>
        <div className="flex flex-wrap gap-x-6 gap-y-1.5">
          {qty !== undefined && <Cell label="Quantity" value={`${fmtQty(qty)} ${unit}`} />}
          {limit !== undefined && <Cell label="Limit" value={fmtUsd(limit)} />}
          {notional !== undefined && <Cell label="Notional" value={fmtUsd(notional)} tone={isBuy ? "red" : "green"} />}
          {cashLeft !== undefined && <Cell label="Settled cash after" value={fmtUsd(cashLeft)} />}
        </div>
        <RationaleQuote text={rationale} />
        {refused ? (
          <p className="flex items-start gap-1.5 text-xs leading-relaxed text-red">
            <XCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden />
            <span className="sr-only">Order refused</span>
            {refused}
          </p>
        ) : ok ? (
          <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-foreground/80">
            <CheckCircle2 className="h-3.5 w-3.5 text-green" aria-hidden />
            <span className="sr-only">Order accepted</span>
            Order placed
            {orderId && <code className="rounded-sm bg-muted px-1.5 py-0.5 font-mono text-[11px]">{orderId}</code>}
          </p>
        ) : r.recorded ? (
          <GenericResult r={r} />
        ) : (
          <NoResultNote />
        )}
      </div>
    </div>
  );
}

function CancelTicket({ input, r }: { input?: Dict; r: ParsedResult }) {
  const orderId = pickStr(input, "order_id") ?? pickStr(r.obj, "order_id");
  const rationale = pickStr(input, "rationale");
  const refused = r.errorMessage;
  const ok = pickBool(r.obj, "ok") === true;
  return (
    <div className="flex overflow-hidden rounded-md border border-amber/35 bg-amber-bg/40 dark:bg-amber-bg">
      <div className="w-1 shrink-0 bg-amber" aria-hidden />
      <div className="min-w-0 flex-1 space-y-2 px-3 py-2.5 sm:px-4">
        <div className="flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center rounded-full border border-amber/40 bg-amber-bg px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-amber">
            Cancel
          </span>
          {orderId && <code className="rounded-sm bg-muted px-1.5 py-0.5 font-mono text-[11px] text-foreground/85">{orderId}</code>}
        </div>
        <RationaleQuote text={rationale} />
        {refused ? (
          <ErrorNote message={refused} />
        ) : ok ? (
          <p className="flex items-center gap-1.5 text-xs text-foreground/80">
            <CheckCircle2 className="h-3.5 w-3.5 text-green" aria-hidden />
            <span className="sr-only">Cancel confirmed</span>
            Order canceled at the broker
          </p>
        ) : r.recorded ? (
          <GenericResult r={r} />
        ) : (
          <NoResultNote />
        )}
      </div>
    </div>
  );
}

function HoldRow({ input, r }: { input?: Dict; r: ParsedResult }) {
  const symbol = pickStr(input, "symbol") ?? pickStr(r.obj, "symbol");
  const rationale = pickStr(input, "rationale");
  const label = symbol ? instrumentLabel(symbol) : undefined;
  return (
    <div className="rounded-md border border-border/50 bg-muted/20 px-3 py-2">
      <div className="flex flex-wrap items-baseline gap-x-2.5 gap-y-1">
        <span className="inline-flex items-center rounded-full border border-border bg-muted px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
          Hold
        </span>
        <span className="font-mono text-sm font-medium text-foreground/85">{label ? label.main : "cash reserve"}</span>
        {label?.detail && <span className="text-[10px] text-muted-foreground">{label.detail}</span>}
      </div>
      {rationale && <p className="mt-1 font-serif text-[12.5px] italic leading-relaxed text-muted-foreground">{rationale}</p>}
      {r.errorMessage && <div className="mt-1.5"><ErrorNote message={r.errorMessage} /></div>}
    </div>
  );
}

// ── write_summary: the session record ────────────────────────────────────

function WriteSummaryCard({ input, r }: { input?: Dict; r: ParsedResult }) {
  const synopsis = pickStr(input, "synopsis");
  const actionItems = pickStr(input, "action_items");
  if (!synopsis && !actionItems) return <GenericResult r={r} />;
  const items = actionItems ? splitSentences(actionItems) : [];
  return (
    <div className="rounded-md border border-claude-border bg-claude-light px-3.5 py-3">
      {synopsis && (
        <>
          <SlipLabel className="mb-1 text-claude">Today's synopsis</SlipLabel>
          <p className="font-serif text-[13.5px] leading-relaxed text-foreground/90">{synopsis}</p>
        </>
      )}
      {items.length > 0 && (
        <>
          <SlipLabel className={cn("mb-1 text-claude", synopsis && "mt-3")}>Plan for the next session</SlipLabel>
          <ul className="space-y-1">
            {items.map((item) => (
              <li key={item} className="flex items-start gap-2 text-[12.5px] leading-relaxed text-foreground/85">
                <Square className="mt-1 h-2.5 w-2.5 shrink-0 text-claude/70" aria-hidden />
                {item}
              </li>
            ))}
          </ul>
        </>
      )}
      {r.errorMessage && <div className="mt-2"><ErrorNote message={r.errorMessage} /></div>}
    </div>
  );
}

// ── web_search / web_fetch: source cards ─────────────────────────────────

function WebSourceBody({ name, input, r }: { name: string; input?: Dict; r: ParsedResult }) {
  const isSearch = name === "web_search";
  const query = pickStr(input, "query");
  const url = pickStr(input, "url");

  let body: ReactNode;
  if (r.errorMessage !== undefined) {
    body = <ErrorNote message={r.errorMessage} />;
  } else if (!r.recorded) {
    body = <NoResultNote />;
  } else if (r.empty) {
    body = <EmptyResultNote text={isSearch ? "The search returned nothing." : "The fetch returned nothing readable. The page may be paywalled or blocked."} />;
  } else if (r.arr || asArray(pickRaw(r.obj, "results", "content")).length > 0) {
    const items = (r.arr ?? asArray(pickRaw(r.obj, "results", "content"))).map(asDict).filter((d): d is Dict => d !== undefined);
    body =
      items.length > 0 ? (
        <ul className="space-y-1.5">
          {items.map((item, i) => {
            const title = pickStr(item, "title");
            const itemUrl = pickStr(item, "url");
            const age = pickStr(item, "page_age");
            if (!title && !itemUrl) return null;
            return (
              <li key={`${itemUrl}-${i}`} className="min-w-0">
                {title && <div className="text-xs font-medium text-foreground/90">{title}</div>}
                <div className="flex min-w-0 flex-wrap items-baseline gap-x-2">
                  {itemUrl && <span className="min-w-0 truncate font-mono text-[10px] text-muted-foreground">{itemUrl}</span>}
                  {age && <span className="shrink-0 text-[10px] text-muted-foreground/80">{age}</span>}
                </div>
              </li>
            );
          })}
        </ul>
      ) : (
        <GenericResult r={r} />
      );
  } else if (r.obj) {
    body = <GenericResult r={r} />;
  } else {
    body = <p className="wrap-anywhere whitespace-pre-wrap text-[13px] leading-relaxed text-foreground/85">{r.raw}</p>;
  }

  return (
    <div className="rounded-md border border-border/60 bg-card-elevated/60 px-3 py-2.5">
      <div className="mb-1.5 flex min-w-0 items-center gap-2">
        {isSearch ? <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden /> : <Globe className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />}
        {isSearch ? (
          <span className="min-w-0 truncate font-serif text-[13px] italic text-foreground/90">{query ? `"${query}"` : "web search"}</span>
        ) : (
          <span className="min-w-0 truncate font-mono text-[11px] text-foreground/85">{url ?? "web fetch"}</span>
        )}
      </div>
      {body}
    </div>
  );
}

// ── code_execution family: terminal panes ────────────────────────────────

function TerminalBody({ name, input, r }: { name: string; input?: Dict; r: ParsedResult }) {
  const lang = name === "code_execution" ? "python" : name === "bash_code_execution" ? "bash" : "file editor";
  const isEditor = name === "text_editor_code_execution";
  const code = pickStr(input, "code");
  const command = pickStr(input, "command");
  const path = pickStr(input, "path");
  const fileText = pickStr(input, "file_text");
  const source = code ?? (isEditor ? fileText : command);
  const headerCmd = isEditor && command ? `${command}${path ? ` ${path}` : ""}` : undefined;

  const o = r.obj;
  const returnCode = pickNum(o, "return_code");
  const errorCode = pickStr(o, "error_code");
  const stdout = typeof o?.stdout === "string" ? o.stdout : undefined;
  const stderr = typeof o?.stderr === "string" ? o.stderr : undefined;
  const isFileUpdate = pickBool(o, "is_file_update");
  const failed = (errorCode !== undefined && errorCode !== "") || (returnCode !== undefined && returnCode !== 0) || r.errorMessage !== undefined;

  return (
    <div className="overflow-hidden rounded-md border border-border/70 bg-zinc-950 font-mono text-xs">
      <div className="flex items-center justify-between gap-2 border-b border-white/10 px-3 py-1.5">
        <span className="text-[10px] uppercase tracking-[0.14em] text-zinc-400">
          {lang}
          {headerCmd && <span className="ml-2 normal-case tracking-normal text-zinc-300">{headerCmd}</span>}
        </span>
        {r.recorded && !r.empty && (
          <span
            className={cn(
              "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold",
              failed ? "bg-red-500/15 text-red-300" : "bg-emerald-500/15 text-emerald-300"
            )}
          >
            <span className="sr-only">{failed ? "Execution failed" : "Execution succeeded"}</span>
            {errorCode ? errorCode : returnCode !== undefined ? `exit ${returnCode}` : failed ? "error" : "ok"}
          </span>
        )}
      </div>
      {source && <pre className="wrap-anywhere whitespace-pre-wrap break-words px-3 py-2 leading-relaxed text-zinc-100">{source}</pre>}
      {!source && input && <pre className="wrap-anywhere whitespace-pre-wrap break-words px-3 py-2 leading-relaxed text-zinc-100">{safeStringify(input, 2)}</pre>}
      {r.errorMessage !== undefined && (
        <div className="border-t border-white/10 px-3 py-2">
          <div className="mb-0.5 text-[9px] uppercase tracking-[0.14em] text-red-400/80">error</div>
          <pre className="wrap-anywhere whitespace-pre-wrap break-words leading-relaxed text-red-300">{r.errorMessage}</pre>
        </div>
      )}
      {stdout !== undefined && stdout !== "" && (
        <div className="border-t border-white/10 px-3 py-2">
          <div className="mb-0.5 text-[9px] uppercase tracking-[0.14em] text-zinc-500">stdout</div>
          <pre className="wrap-anywhere whitespace-pre-wrap break-words leading-relaxed text-zinc-300">{stdout}</pre>
        </div>
      )}
      {stderr !== undefined && stderr !== "" && (
        <div className="border-t border-white/10 px-3 py-2">
          <div className="mb-0.5 text-[9px] uppercase tracking-[0.14em] text-red-400/80">stderr</div>
          <pre className="wrap-anywhere whitespace-pre-wrap break-words leading-relaxed text-red-300">{stderr}</pre>
        </div>
      )}
      {isEditor && r.recorded && !failed && (
        <div className="flex items-center gap-1.5 border-t border-white/10 px-3 py-1.5 text-[10px] text-zinc-400">
          <FileText className="h-3 w-3" aria-hidden />
          {isFileUpdate ? "existing file updated" : "file written"}
        </div>
      )}
      {!r.recorded && <div className="border-t border-white/10 px-3 py-1.5 text-[10px] italic text-zinc-500">no result recorded</div>}
    </div>
  );
}
