"use client";

import { useEffect, useRef, useState } from "react";

/*
useLiveQuotes consumes the server's SSE quote stream (/api/quotes/stream)
and returns the latest per-unit mark for every symbol seen: equities keyed
by ticker, options keyed by their OCC symbol, both straight from the
Schwab streamer. Consumers re-price position marks per tick.

Implementation notes:
  - fetch-based SSE rather than EventSource: every /api route requires the
    X-VT-Source header, which EventSource cannot send.
  - Reconnects with capped backoff; a 503 (no hub wired: trading disabled
    or no Schwab) backs off long and the UI quietly stays on polling.
  - Updates are batched per animation frame so a burst of ticks costs one
    render, not one per tick.
*/

export interface LiveQuote {
  mark: number;
  bid?: number;
  ask?: number;
  /** Wall-clock ms (Date.now) when this mark arrived, so consumers can drop
   *  a mark that's gone stale after the stream stopped ticking. */
  ts: number;
}

type TickMsg = { type: "equity" | "option"; symbol: string; mark: number; bid?: number; ask?: number };

export function useLiveQuotes(enabled = true): Map<string, LiveQuote> {
  const [quotes, setQuotes] = useState<Map<string, LiveQuote>>(() => new Map());
  const pending = useRef<Map<string, LiveQuote> | null>(null);
  const frame = useRef(0);

  useEffect(() => {
    if (!enabled) return;
    let alive = true;
    let attempt = 0;
    let abort: AbortController | null = null;

    const flush = () => {
      frame.current = 0;
      const batch = pending.current;
      pending.current = null;
      if (!batch || !alive) return;
      setQuotes((prev) => {
        const next = new Map(prev);
        for (const [k, v] of batch) next.set(k, v);
        return next;
      });
    };

    const onTick = (msg: TickMsg) => {
      if (!msg.symbol || !(msg.mark > 0)) return;
      pending.current ??= new Map();
      pending.current.set(msg.symbol, { mark: msg.mark, bid: msg.bid, ask: msg.ask, ts: Date.now() });
      if (!frame.current) frame.current = requestAnimationFrame(flush);
    };

    const connect = async () => {
      while (alive) {
        abort = new AbortController();
        try {
          const res = await fetch("/api/quotes/stream", {
            headers: { "X-VT-Source": "dashboard", Accept: "text/event-stream" },
            signal: abort.signal,
            cache: "no-store",
          });
          if (res.status === 503) {
            // No hub on this deployment: stay on polling, retry rarely.
            await sleep(5 * 60_000);
            continue;
          }
          if (!res.ok || !res.body) throw new Error(`stream ${res.status}`);
          attempt = 0;
          const reader = res.body.getReader();
          const decoder = new TextDecoder();
          let buf = "";
          for (;;) {
            const { done, value } = await reader.read();
            if (done) break;
            buf += decoder.decode(value, { stream: true });
            // SSE frames are separated by a blank line.
            for (let i = buf.indexOf("\n\n"); i !== -1; i = buf.indexOf("\n\n")) {
              const frameText = buf.slice(0, i);
              buf = buf.slice(i + 2);
              for (const line of frameText.split("\n")) {
                if (!line.startsWith("data: ")) continue; // heartbeats are comments
                try {
                  onTick(JSON.parse(line.slice(6)) as TickMsg);
                } catch {
                  // malformed frame: skip
                }
              }
            }
          }
        } catch {
          // fall through to backoff
        }
        if (!alive) return;
        attempt += 1;
        await sleep(Math.min(30_000, 1_000 * 2 ** attempt));
      }
    };

    void connect();
    return () => {
      alive = false;
      abort?.abort();
      if (frame.current) cancelAnimationFrame(frame.current);
    };
  }, [enabled]);

  return quotes;
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}
