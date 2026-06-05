"use client";

import { useEffect, useRef, useState } from "react";

/*
A number that counts up from 0 to `to` the first time it scrolls into view,
with an ease-out so it shoots up fast and settles. Formatted with thousands
separators and an optional prefix (e.g. "$"). Inherits color/size from its
parent, so it picks up the gradient/size of whatever wraps it.
*/
export function CountUp({ to, prefix = "", suffix = "", durationMs = 1100 }: { to: number; prefix?: string; suffix?: string; durationMs?: number }) {
  const ref = useRef<HTMLSpanElement>(null);
  const [value, setValue] = useState(0);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    let raf = 0;
    let started = false;

    const run = () => {
      const start = performance.now();
      const step = (now: number) => {
        const t = Math.min(1, (now - start) / durationMs);
        const eased = 1 - (1 - t) ** 3; // easeOutCubic
        setValue(Math.round(eased * to));
        if (t < 1) raf = requestAnimationFrame(step);
      };
      raf = requestAnimationFrame(step);
    };

    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting && !started) {
            started = true;
            run();
            io.disconnect();
          }
        }
      },
      { threshold: 0.4 },
    );
    io.observe(el);
    return () => {
      io.disconnect();
      cancelAnimationFrame(raf);
    };
  }, [to, durationMs]);

  return (
    <span ref={ref}>
      {prefix}
      {value.toLocaleString("en-US")}
      {suffix}
    </span>
  );
}
