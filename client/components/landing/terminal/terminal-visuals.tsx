import type { ReactNode } from "react";

import { CountUp } from "@/components/landing/count-up";
import { cn } from "@/lib/utils";

/*
Supporting visuals for the terminal landing page. Everything here reads
as command output: mono type, hard hairlines, no cards and no soft
shadows. Each one fills the opposite column of a LogSection.
*/

/* ── Shell prompt line that opens every section. The command is the
      section's title card, the trailing comment is the aside. ── */
export function PromptLine({ command, comment }: { command: string; comment?: string }) {
  return (
    <p className="flex flex-wrap items-baseline gap-x-3 font-mono text-[13px] leading-relaxed">
      <span aria-hidden className="font-bold text-green phosphor">
        $
      </span>
      <span className="font-semibold text-foreground">{command}</span>
      {comment && <span className="text-muted-foreground/80">&middot; {comment}</span>}
    </p>
  );
}

/* ── The account equity, printed like a quote readout. The headline
      figure of the whole page, so it gets the phosphor bloom. ── */
export function EquityReadout({ equity }: { equity: number | null }) {
  return (
    <div className="font-mono">
      <div className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">acct_equity</div>
      <div className="phosphor mt-3 text-[52px] font-extrabold leading-none tracking-tight tabular-nums text-green sm:text-[64px]">
        <CountUp to={Math.round(equity ?? 5000)} prefix="$" durationMs={2600} />
      </div>
      <dl className="mt-6 space-y-2 border-t border-dashed border-border pt-4 text-[13px]">
        <div className="flex items-baseline justify-between gap-6">
          <dt className="text-muted-foreground">status</dt>
          <dd className="font-semibold text-green">{equity ? "live, trading" : "live, not a simulation"}</dd>
        </div>
        <div className="flex items-baseline justify-between gap-6">
          <dt className="text-muted-foreground">benchmark</dt>
          <dd className="font-semibold text-foreground">buy-and-hold SPY</dd>
        </div>
        <div className="flex items-baseline justify-between gap-6">
          <dt className="text-muted-foreground">my expectations</dt>
          <dd className="font-semibold text-red">written off</dd>
        </div>
      </dl>
    </div>
  );
}

/* ── crontab -l. The three real crons that drive the day, verbatim from
      the server, with the punchline that no strategy lives here. ── */
const CRONS = [
  {
    expr: "30 12 * * MON-FRI",
    job: "the session",
    detail: "reads the book, the news, and the tape, then trades",
  },
  {
    expr: "*/15 10-15 * * MON-FRI",
    job: "the sweep",
    detail: "reconciles its orders against the broker, finds out what actually filled",
  },
  {
    expr: "00 16 * * MON-FRI",
    job: "the close",
    detail: "snapshots equity vs SPY, then emails everyone the day's damage",
  },
];

export function CronTable() {
  return (
    <div className="font-mono text-[13px]">
      <ol className="space-y-5">
        {CRONS.map((c) => (
          <li key={c.job} className="border-l-2 border-green/40 pl-4">
            <div className="phosphor text-[12px] font-bold tracking-tight text-green">{c.expr}</div>
            <div className="mt-1 font-bold text-foreground">{c.job}</div>
            <div className="mt-0.5 leading-relaxed text-muted-foreground">{c.detail}</div>
          </li>
        ))}
      </ol>
      <p className="mt-5 border-t border-dashed border-border pt-3 text-[12px] text-muted-foreground/80">there is no strategy in this file. it makes one up every session.</p>
    </div>
  );
}

/* ── guards.go, dumped like a config file. The joke is how little is left
      in it: every allocation limit is gone, so those keys print "none" in
      red, and the one real rule is the broker's. ── */
const GUARD_LINES: { key: string; value: ReactNode; tone?: "red" | "muted" }[] = [
  { key: "allocation_split", value: "none", tone: "red" },
  { key: "per_name_cap", value: "none", tone: "red" },
  { key: "per_order_cap", value: "none", tone: "red" },
  { key: "drawdown_halt", value: "none", tone: "red" },
  { key: "settled_cash", value: "settled cash only (the broker insists)", tone: "muted" },
];

export function GuardsReadout() {
  return (
    <div className="font-mono text-[13px]">
      <div className="flex items-baseline justify-between gap-4">
        <span className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">guards.go</span>
        <span className="phosphor text-[11px] font-bold uppercase tracking-[0.14em] text-green">enforced in code</span>
      </div>
      <dl className="mt-4 border-b border-border">
        {GUARD_LINES.map((c) => (
          <div key={c.key} className="flex items-baseline justify-between gap-6 border-t border-border py-3">
            <dt className="shrink-0 font-bold text-foreground">{c.key}</dt>
            <dd className={cn("text-right tabular-nums", c.tone === "red" ? "text-red" : "text-muted-foreground")}>{c.value}</dd>
          </div>
        ))}
      </dl>
      <p className="mt-3 text-[12px] text-muted-foreground/80">that is the whole file.</p>
    </div>
  );
}

/* ── ls -l tools/. The fixed tool surface, with permission bits doing
      the talking: reading can only read, execution can execute. ── */
interface ToolDir {
  bits: string;
  dir: string;
  note: string;
  names: string[];
  accent?: "green" | "red" | "claude";
}

const TOOL_DIRS: ToolDir[] = [
  {
    bits: "dr--r--r--",
    dir: "reading/",
    note: "look, never touch",
    names: ["get_portfolio", "get_stock_quotes", "get_option_chain", "get_price_history", "get_fundamentals", "get_track_record", "get_market_context", "get_recent_decisions", "get_order_status", "web_search", "web_fetch"],
  },
  {
    bits: "drwx------",
    dir: "execution/",
    note: "real money",
    names: ["buy_equity", "sell_equity", "buy_option", "sell_option", "cancel_order", "hold"],
    accent: "green",
  },
  {
    bits: "drw-r--r--",
    dir: "documentation/",
    note: "writes the log",
    names: ["write_summary"],
    accent: "claude",
  },
];

export function ToolLs() {
  return (
    <div className="space-y-6 font-mono text-[13px]">
      {TOOL_DIRS.map((d) => (
        <div key={d.dir}>
          <div className="flex flex-wrap items-baseline gap-x-3">
            <span className="text-[12px] tracking-tight text-muted-foreground/80">{d.bits}</span>
            <span className={cn("font-bold", d.accent === "green" ? "phosphor text-green" : d.accent === "claude" ? "text-claude" : "text-foreground")}>{d.dir}</span>
            <span className="ml-auto text-[12px] text-muted-foreground/70">{d.note}</span>
          </div>
          <p className="mt-2 break-words border-l-2 border-border pl-4 leading-relaxed text-foreground/75">{d.names.join("  ")}</p>
        </div>
      ))}
      <p className="border-t border-dashed border-border pt-3 text-[12px] text-muted-foreground/80">every tool hand-built and tested in CI. if it is not on this list, she cannot do it.</p>
    </div>
  );
}

/* ── The hero's bottom strip: three facts printed as one ledger row. ── */
export function HeroStatStrip({ equity }: { equity: number | null }) {
  return (
    <dl className="grid grid-cols-1 divide-y divide-dashed divide-border border-y border-dashed border-border font-mono sm:grid-cols-3 sm:divide-x sm:divide-y-0">
      <div className="flex flex-col gap-1 px-1 py-4 sm:px-6 sm:first:pl-1">
        <dt className="text-[10px] font-bold uppercase tracking-[0.2em] text-muted-foreground">acct_equity</dt>
        <dd className="phosphor text-xl font-extrabold tabular-nums text-green sm:text-2xl">
          <CountUp to={Math.round(equity ?? 5000)} prefix="$" durationMs={2600} />
        </dd>
      </div>
      <div className="flex flex-col gap-1 px-1 py-4 sm:px-6">
        <dt className="text-[10px] font-bold uppercase tracking-[0.2em] text-muted-foreground">benchmark</dt>
        <dd className="text-xl font-extrabold text-foreground sm:text-2xl">SPY</dd>
      </div>
      <div className="flex flex-col gap-1 px-1 py-4 sm:px-6">
        <dt className="text-[10px] font-bold uppercase tracking-[0.2em] text-muted-foreground">adult_supervision</dt>
        <dd className="text-xl font-extrabold text-red sm:text-2xl">none</dd>
      </div>
    </dl>
  );
}

/* ── Ticker tape. A dashed-bordered marquee seam between the hero and
      the log, scrolling the kind of lines the session actually emits. ── */
const TAPE: { text: string; tone?: "green" | "red" | "muted" }[] = [
  { text: "NVDA +2.14%", tone: "green" },
  { text: "buy_equity NVDA x9 @ 141.19 · filled" },
  { text: "hold MSFT", tone: "muted" },
  { text: "SPY · the one to beat", tone: "muted" },
  { text: "buy_option NVDA 145C · all in", tone: "green" },
  { text: "write_summary · very confident note filed" },
  { text: "settled_cash $3,920.10", tone: "muted" },
  { text: "sell_equity · loser cut, lesson pending", tone: "red" },
  { text: "no allocation cap · full discretion", tone: "muted" },
  { text: "cancel_order · second thoughts", tone: "muted" },
];

export function TickerTape() {
  return (
    <div aria-hidden className="overflow-hidden border-y border-dashed border-border py-2.5 [-webkit-mask-image:linear-gradient(to_right,transparent,black_6%,black_94%,transparent)] [mask-image:linear-gradient(to_right,transparent,black_6%,black_94%,transparent)]">
      <div className="flex w-max animate-marquee gap-10 whitespace-nowrap font-mono text-[12px]" style={{ animationDuration: "45s" }}>
        {[...TAPE, ...TAPE].map((t, i) => (
          <span key={`${t.text}-${i}`} className={cn("inline-flex items-center gap-10", t.tone === "green" ? "text-green" : t.tone === "red" ? "text-red" : t.tone === "muted" ? "text-muted-foreground" : "text-foreground/80")}>
            {t.text}
            <span className="text-muted-foreground/40">///</span>
          </span>
        ))}
      </div>
    </div>
  );
}
