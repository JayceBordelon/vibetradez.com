const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

function pad(n: number): string {
  return n < 10 ? `0${n}` : `${n}`;
}

export function toDateStr(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function parseDate(s: string): Date {
  const [y, m, d] = s.split("-").map(Number);
  return new Date(y, m - 1, d);
}

export function formatDateShort(dateStr: string): string {
  const d = parseDate(dateStr);
  return `${DAYS[d.getDay()]}, ${MONTHS[d.getMonth()]} ${d.getDate()}`;
}

export function formatDayName(dateStr: string): string {
  return DAYS[parseDate(dateStr).getDay()];
}

export function formatMonthDay(dateStr: string): string {
  const d = parseDate(dateStr);
  return `${MONTHS[d.getMonth()]} ${d.getDate()}`;
}

export function getRangeBounds(mode: string, offset: number): { start: string; end: string } {
  const now = new Date();

  if (mode === "week") {
    const ref = new Date(now);
    ref.setDate(ref.getDate() - offset * 7);
    const day = ref.getDay();
    const diffToMon = day === 0 ? -6 : 1 - day;
    const start = new Date(ref);
    start.setDate(ref.getDate() + diffToMon);
    const end = new Date(start);
    end.setDate(start.getDate() + 4);
    return { start: toDateStr(start), end: toDateStr(end) };
  }

  if (mode === "month") {
    const start = new Date(now.getFullYear(), now.getMonth() - offset, 1);
    const end = new Date(now.getFullYear(), now.getMonth() - offset + 1, 0);
    return { start: toDateStr(start), end: toDateStr(end) };
  }

  if (mode === "year") {
    const start = new Date(now.getFullYear() - offset, 0, 1);
    const end = new Date(now.getFullYear() - offset, 11, 31);
    return { start: toDateStr(start), end: toDateStr(end) };
  }

  return { start: "2020-01-01", end: toDateStr(now) };
}

export function getRangeLabel(mode: string, offset: number): string {
  if (mode === "week") {
    const b = getRangeBounds("week", offset);
    return `${formatMonthDay(b.start)} to ${formatMonthDay(b.end)}`;
  }
  if (mode === "month") {
    const d = new Date();
    d.setMonth(d.getMonth() - offset);
    return `${MONTHS[d.getMonth()]} ${d.getFullYear()}`;
  }
  if (mode === "year") {
    return `${new Date().getFullYear() - offset}`;
  }
  return "All Time";
}

export function maxRangeOffset(mode: string, dates: string[]): number {
  if (dates.length === 0) return 0;
  const oldest = parseDate(dates[dates.length - 1]);
  const now = new Date();

  if (mode === "week") {
    return Math.ceil((now.getTime() - oldest.getTime()) / (7 * 86400000));
  }
  if (mode === "month") {
    return (now.getFullYear() - oldest.getFullYear()) * 12 + now.getMonth() - oldest.getMonth();
  }
  if (mode === "year") {
    return now.getFullYear() - oldest.getFullYear();
  }
  return 0;
}
