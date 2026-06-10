"use client";

import { useEffect, useState } from "react";

/*
Live New York wall clock for the status bar. Renders a placeholder on
the server and starts ticking after mount, so there is no hydration
mismatch from the two environments reading different clocks.
*/

const FMT = new Intl.DateTimeFormat("en-US", {
  timeZone: "America/New_York",
  hour12: false,
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
});

export function SessionClock() {
  const [now, setNow] = useState<string | null>(null);

  useEffect(() => {
    const tick = () => setNow(FMT.format(new Date()));
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, []);

  return (
    <span className="tabular-nums" suppressHydrationWarning>
      {now ?? "--:--:--"} ET
    </span>
  );
}
