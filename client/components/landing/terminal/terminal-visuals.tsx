import { Ban, Clock, Lock, type LucideIcon, Mail, RefreshCw, Sunrise, Sunset, TrendingUp } from "lucide-react";
import type { ReactNode } from "react";

import { CountUp } from "@/components/landing/count-up";
import { cn } from "@/lib/utils";
import type { EquityCurvePoint } from "@/types/portfolio";

/*
Supporting visuals for the marketing landing — OPEN editorial data readouts
in the product's design language: big calm numbers, hairline-divided rows,
chips, soft sage tints. No boxed cards. Each fills the opposite column of a
feature section and reads as a data readout sitting on the page, not a tile.
*/

// A soft decorative sage sparkline, shown only as a fallback before there are
// at least two real equity snapshots. Purely ornamental (aria-hidden).
function HeroSparkline({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 320 80" preserveAspectRatio="none" className={className} aria-hidden>
      <defs>
        <linearGradient id="heroSpark" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--primary)" stopOpacity={0.22} />
          <stop offset="100%" stopColor="var(--primary)" stopOpacity={0} />
        </linearGradient>
      </defs>
      <path d="M0 62 C 28 58, 44 50, 70 52 S 120 40, 150 44 S 210 22, 250 26 S 300 10, 320 14 L320 80 L0 80 Z" fill="url(#heroSpark)" />
      <path d="M0 62 C 28 58, 44 50, 70 52 S 120 40, 150 44 S 210 22, 250 26 S 300 10, 320 14" fill="none" stroke="var(--primary)" strokeWidth={2.25} strokeLinecap="round" />
    </svg>
  );
}

// Project the real equity series into the 320x80 viewBox as line + area paths.
// Pure geometry, computed server-side, so the hero ships a real chart with no
// client JS. A flat series (min === max) draws a centered line rather than
// dividing by zero.
function buildSpark(values: number[]) {
  const W = 320;
  const H = 80;
  const padY = 12;
  const n = values.length;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min;
  const x = (i: number) => (n === 1 ? W : (i / (n - 1)) * W);
  const y = (v: number) => (span === 0 ? H / 2 : padY + (1 - (v - min) / span) * (H - padY * 2));
  const line = values.map((v, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)} ${y(v).toFixed(1)}`).join(" ");
  return { line, area: `${line} L${W} ${H} L0 ${H} Z` };
}

// The hero sparkline driven by the account's real daily equity. Green when the
// window ends at or above where it started, red when it's down (so it is no
// longer "always green"). Falls back to the decorative curve until there are
// two real snapshots to draw a line between.
function EquitySparkline({ points, className }: { points: EquityCurvePoint[]; className?: string }) {
  const values = points.map((p) => p.account_equity).filter((v) => v > 0);
  if (values.length < 2) return <HeroSparkline className={className} />;

  const { line, area } = buildSpark(values);
  const up = values[values.length - 1] >= values[0];
  const color = up ? "var(--green)" : "var(--red)";
  const gradId = up ? "heroEquityUp" : "heroEquityDown";
  return (
    <svg viewBox="0 0 320 80" preserveAspectRatio="none" className={className} role="img" aria-label={`Account equity across ${values.length} daily snapshots, ${up ? "up" : "down"} over the window`}>
      <defs>
        <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity={0.22} />
          <stop offset="100%" stopColor={color} stopOpacity={0} />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${gradId})`} />
      <path d={line} fill="none" stroke={color} strokeWidth={2.25} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

/* ── The live account readout: the hero's headline figure, open on the page
      (no box) — the big number leads, a soft sparkline underneath, then a
      hairline-divided fact row. ── */
export function EquityReadout({ equity, points = [] }: { equity: number | null; points?: EquityCurvePoint[] }) {
  return (
    <div className="lg:pl-8">
      <div className="flex items-center justify-between">
        <span className="inline-flex items-center gap-2 text-xs font-medium text-muted-foreground">
          <span className="relative flex h-2 w-2" aria-hidden>
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-60" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
          </span>
          Live account
        </span>
        <span className="text-[11px] font-medium uppercase tracking-[0.1em] text-muted-foreground">vs SPY</span>
      </div>
      <div className="mt-5 text-[11px] font-medium uppercase tracking-[0.1em] text-muted-foreground">Account equity</div>
      <div className="mt-1.5 text-[clamp(48px,8vw,72px)] font-semibold leading-none tracking-tight tabular-nums text-foreground">
        <CountUp to={Math.round(equity ?? 5000)} prefix="$" durationMs={2200} />
      </div>
      <div className="-mx-1 mt-6 h-16">
        <EquitySparkline points={points} className="h-full w-full" />
      </div>
      <dl className="mt-5 grid grid-cols-2 gap-3 border-t border-border/70 pt-5 text-sm">
        <div>
          <dt className="text-xs text-muted-foreground">Status</dt>
          <dd className="mt-0.5 font-medium text-green">{equity ? "Live, trading" : "Live, not a simulation"}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">Benchmark</dt>
          <dd className="mt-0.5 font-medium text-foreground">Buy-and-hold SPY</dd>
        </div>
      </dl>
    </div>
  );
}

/* ── The daily schedule, as clean cards. ── */
const CRONS: { icon: LucideIcon; time: string; job: string; detail: string }[] = [
  { icon: Sunrise, time: "9:45 AM ET", job: "The open", detail: "Reacts to the overnight tape and the opening move, once spreads settle." },
  { icon: Clock, time: "12:30 PM ET", job: "Midday", detail: "The primary read: the book, the news and hype, and the tape, then trades." },
  { icon: Sunset, time: "3:30 PM ET", job: "The pre-close", detail: "Decides what to carry overnight versus trim, before the bell." },
  { icon: RefreshCw, time: "Every 15m", job: "The sweep", detail: "Reconciles its orders against the broker and finds out what actually filled." },
  { icon: Mail, time: "4:00 PM ET", job: "The close", detail: "Snapshots equity vs SPY, then emails everyone the day's damage." },
];

export function CronTable() {
  return (
    <div>
      <div className="divide-y divide-border/60 border-t border-border/60">
        {CRONS.map((c) => (
          <div key={c.job} className="flex items-start gap-3.5 py-4">
            <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-secondary text-primary">
              <c.icon className="h-4.5 w-4.5" />
            </span>
            <div className="min-w-0">
              <div className="flex flex-wrap items-baseline gap-x-2.5">
                <span className="font-medium text-foreground">{c.job}</span>
                <span className="text-xs font-medium text-muted-foreground tabular-nums">{c.time}</span>
              </div>
              <p className="mt-0.5 text-sm leading-relaxed text-muted-foreground">{c.detail}</p>
            </div>
          </div>
        ))}
      </div>
      <p className="pt-4 text-sm text-muted-foreground">There is no fixed strategy. It makes one up every session.</p>
    </div>
  );
}

/* ── The guardrails, as a clean spec card. ── */
const GUARD_LINES: { icon: LucideIcon; key: string; value: ReactNode; tone?: "red" }[] = [
  { icon: TrendingUp, key: "Instruments", value: "Options only" },
  { icon: Ban, key: "Equity buys", value: "Disabled", tone: "red" },
  { icon: Lock, key: "Per-contract cap", value: "~10% of equity" },
  { icon: Lock, key: "Per-name cap", value: "~25% of equity" },
  { icon: Lock, key: "Cash rule", value: "Settled cash only" },
];

export function GuardsReadout() {
  return (
    <div>
      <div className="flex items-center justify-between pb-3.5">
        <span className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">Hard caps</span>
        <span className="rounded-full bg-secondary px-2.5 py-0.5 text-[11px] font-medium text-secondary-foreground">Enforced in code</span>
      </div>
      <dl className="divide-y divide-border/60 border-t border-border/60">
        {GUARD_LINES.map((c) => (
          <div key={c.key} className="flex items-center justify-between gap-4 py-3.5">
            <dt className="flex items-center gap-2.5 text-sm text-muted-foreground">
              <c.icon className={cn("h-4 w-4 shrink-0", c.tone === "red" ? "text-red" : "text-primary")} />
              {c.key}
            </dt>
            <dd className={cn("text-right text-sm font-medium tabular-nums", c.tone === "red" ? "text-red" : "text-foreground")}>{c.value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

/* ── The toolbox: the fixed tool surface, grouped by what it can do. ── */
interface ToolDir {
  dir: string;
  note: string;
  names: string[];
  accent: "neutral" | "green" | "claude";
}

const TOOL_DIRS: ToolDir[] = [
  {
    dir: "Reading",
    note: "Look, never touch",
    accent: "neutral",
    names: ["get_portfolio", "get_stock_quotes", "get_option_chain", "get_price_history", "get_fundamentals", "get_track_record", "get_market_context", "get_recent_decisions", "get_order_status", "get_ticker_news", "get_trending_tickers", "get_social_sentiment", "web_search", "web_fetch"],
  },
  {
    dir: "Execution",
    note: "Real money",
    accent: "green",
    names: ["buy_option", "sell_option", "sell_equity", "cancel_order", "hold"],
  },
  {
    dir: "Documentation",
    note: "Writes the log",
    accent: "claude",
    names: ["write_summary"],
  },
];

export function ToolLs() {
  return (
    <div>
      <div className="divide-y divide-border/60 border-t border-border/60">
        {TOOL_DIRS.map((d) => (
          <div key={d.dir} className="py-4">
            <div className="mb-2.5 flex items-center gap-2">
              <span className={cn("h-2 w-2 rounded-full", d.accent === "green" ? "bg-green" : d.accent === "claude" ? "bg-claude" : "bg-muted-foreground/50")} aria-hidden />
              <span className="text-sm font-medium text-foreground">{d.dir}</span>
              <span className="ml-auto text-xs text-muted-foreground">{d.note}</span>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {d.names.map((n) => (
                <span key={n} className="rounded-md bg-muted px-2 py-0.5 font-mono text-[11px] text-muted-foreground">
                  {n}
                </span>
              ))}
            </div>
          </div>
        ))}
      </div>
      <p className="pt-4 text-sm text-muted-foreground">Every tool is hand-built and tested in CI. If it's not on this list, she can't do it.</p>
    </div>
  );
}

/* ── The hero's bottom strip: three facts as an open, hairline-divided row. ── */
export function HeroStatStrip({ equity }: { equity: number | null }) {
  return (
    <dl className="grid grid-cols-3 divide-x divide-border border-t border-border/60 pt-5 [&>*]:px-4 [&>*:first-child]:pl-0 [&>*:last-child]:pr-0">
      <Tile label="Account equity">
        <span className="tabular-nums text-foreground">
          <CountUp to={Math.round(equity ?? 5000)} prefix="$" durationMs={2200} />
        </span>
      </Tile>
      <Tile label="Benchmark">
        <span className="text-foreground">SPY</span>
      </Tile>
      <Tile label="Supervision">
        <span className="text-red">None</span>
      </Tile>
    </dl>
  );
}

function Tile({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="truncate text-[11px] font-medium uppercase tracking-[0.06em] text-muted-foreground">{label}</dt>
      <dd className="mt-1 truncate text-xl font-semibold tracking-tight sm:text-2xl">{children}</dd>
    </div>
  );
}
