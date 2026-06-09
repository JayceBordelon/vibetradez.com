export function fmt(n: number, d = 2): string {
  return Number(n).toFixed(d);
}

export function fmtPct(n: number): string {
  return `${n > 0 ? "+" : ""}${fmt(n, 0)}%`;
}

export function fmtPctDec(n: number): string {
  return `${n > 0 ? "+" : ""}${fmt(n, 1)}%`;
}

/*
Currency is formatted with Intl.NumberFormat so it carries proper thousands
separators ($1,480.42) and locale-correct grouping, rather than a bare
toFixed. usd keeps cents; usd0 is whole-dollar for compact P&L / notional.
*/
const usd = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", minimumFractionDigits: 2, maximumFractionDigits: 2 });
const usd0 = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 });
// Prices/strikes: grouped, cents only when present ($79.50 strike, $1,234 strike).
const usdAuto = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", minimumFractionDigits: 0, maximumFractionDigits: 2 });

export function fmtPrice(n: number): string {
  return usdAuto.format(Number.isFinite(n) ? n : 0);
}

export function fmtMoney(n: number): string {
  return usd.format(Number.isFinite(n) ? n : 0);
}

export function fmtMoneyInt(n: number): string {
  return usd0.format(Math.abs(Number.isFinite(n) ? n : 0));
}

export function fmtPnlInt(n: number): string {
  if (!Number.isFinite(n) || n === 0) return "$0";
  const body = usd0.format(Math.abs(n));
  return n > 0 ? `+${body}` : `-${body}`;
}

// Signed P&L with cents, for readouts that sit next to other cents-precision
// money (the dashboard stat strip shows $4,589.75 equity, so a whole-dollar
// +$206 beside it read as a different unit).
export function fmtPnl(n: number): string {
  if (!Number.isFinite(n) || n === 0) return "$0.00";
  const body = usd.format(Math.abs(n));
  return n > 0 ? `+${body}` : `-${body}`;
}

export function plural(n: number, unit: string): string {
  return n === 1 ? unit : `${unit}s`;
}

export function pnlColor(v: number): string {
  if (v > 0) return "text-green";
  if (v < 0) return "text-red";
  return "text-muted-foreground";
}

