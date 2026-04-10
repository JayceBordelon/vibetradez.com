export function fmt(n: number, d = 2): string {
	return Number(n).toFixed(d);
}

export function fmtPct(n: number): string {
	return `${n > 0 ? "+" : ""}${fmt(n, 0)}%`;
}

export function fmtPctDec(n: number): string {
	return `${n > 0 ? "+" : ""}${fmt(n, 1)}%`;
}

export function fmtMoney(n: number): string {
	return `$${fmt(n)}`;
}

export function fmtMoneyInt(n: number): string {
	return `$${fmt(Math.abs(n), 0)}`;
}

export function fmtPnlInt(n: number): string {
	if (n > 0) return `+$${fmt(n, 0)}`;
	if (n < 0) return `-$${fmt(Math.abs(n), 0)}`;
	return "$0";
}

export function pnlColor(v: number): string {
	if (v > 0) return "text-green";
	if (v < 0) return "text-red";
	return "text-muted-foreground";
}
