/**
US equities market hours: Mon-Fri 9:30 AM to 4:00 PM ET. Holidays
are not enumerated client-side (the Go cron owns the canonical
holiday list); on a holiday weekday the dashboard simply shows the
"after-hours" copy until the next trading day, which still reads
correctly.
*/

export type MarketStatus = {
  open: boolean;
  reason: string;
  nextOpen?: string;
};

interface EtParts {
  weekday: number; // 0 = Sunday, 6 = Saturday
  hour: number;
  minute: number;
}

function etParts(now: Date): EtParts {
  const fmt = new Intl.DateTimeFormat("en-US", {
    timeZone: "America/New_York",
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
  const parts = fmt.formatToParts(now);
  const weekdayMap: Record<string, number> = { Sun: 0, Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6 };
  const wd = parts.find((p) => p.type === "weekday")?.value ?? "Mon";
  const hour = Number.parseInt(parts.find((p) => p.type === "hour")?.value ?? "0", 10);
  const minute = Number.parseInt(parts.find((p) => p.type === "minute")?.value ?? "0", 10);
  return { weekday: weekdayMap[wd] ?? 1, hour: hour % 24, minute };
}

function nextWeekdayLabel(weekday: number): string {
  const labels = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
  return labels[weekday] ?? "Monday";
}

export function getMarketStatus(now: Date = new Date()): MarketStatus {
  const { weekday, hour, minute } = etParts(now);
  const minutesIntoDay = hour * 60 + minute;
  const openMinutes = 9 * 60 + 30;
  const closeMinutes = 16 * 60;

  if (weekday === 0 || weekday === 6) {
    const daysUntilMon = weekday === 6 ? 2 : 1;
    const nextDay = (weekday + daysUntilMon) % 7;
    return {
      open: false,
      reason: weekday === 6 ? "It's Saturday. Markets reopen Monday at 9:30 AM ET." : "It's Sunday. Markets reopen Monday at 9:30 AM ET.",
      nextOpen: `${nextWeekdayLabel(nextDay)} 9:25 AM ET`,
    };
  }

  if (minutesIntoDay < openMinutes) {
    return {
      open: false,
      reason: "Pre-market. Trading opens at 9:30 AM ET, picks publish at 9:25 AM ET.",
      nextOpen: `Today 9:25 AM ET`,
    };
  }

  if (minutesIntoDay >= closeMinutes) {
    if (weekday === 5) {
      return {
        open: false,
        reason: "Markets are closed for the day. Reopens Monday at 9:30 AM ET.",
        nextOpen: "Monday 9:25 AM ET",
      };
    }
    return {
      open: false,
      reason: "Markets are closed for the day. Reopens tomorrow at 9:30 AM ET.",
      nextOpen: `${nextWeekdayLabel((weekday + 1) % 7)} 9:25 AM ET`,
    };
  }

  return { open: true, reason: "" };
}
