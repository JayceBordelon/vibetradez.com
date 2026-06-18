export function fmt(n: number, d = 2): string {
  return Number(n).toFixed(d);
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
  if (!Number.isFinite(n)) return "$0";
  // Round to the displayed unit (whole dollars) BEFORE choosing the sign, so
  // a value that rounds to $0 reads as a neutral "$0" rather than a signed
  // "+$0"/"-$0" whose sign contradicts its own (zero) magnitude.
  const rounded = Math.round(n);
  if (rounded === 0) return "$0";
  const body = usd0.format(Math.abs(rounded));
  return rounded > 0 ? `+${body}` : `-${body}`;
}

// Signed P&L with cents, for readouts that sit next to other cents-precision
// money (the dashboard stat strip shows $4,589.75 equity, so a whole-dollar
// +$206 beside it read as a different unit).
export function fmtPnl(n: number): string {
  if (!Number.isFinite(n)) return "$0.00";
  // Round to cents before choosing the sign (see fmtPnlInt): a sub-cent
  // value shows neutral "$0.00", never "-$0.00".
  const rounded = Math.round(n * 100) / 100;
  if (rounded === 0) return "$0.00";
  const body = usd.format(Math.abs(rounded));
  return rounded > 0 ? `+${body}` : `-${body}`;
}

export function pnlColor(v: number): string {
  if (v > 0) return "text-green";
  if (v < 0) return "text-red";
  return "text-muted-foreground";
}

