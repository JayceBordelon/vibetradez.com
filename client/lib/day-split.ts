/*
Day-split math: decompose a position's day into the overnight gap (prior
close to today's open) and the session move (open to now). The anchors
come from the server: open_value from the live quote, prev_close_value
from our EOD snapshot ledger, whose presence is the held-overnight signal.

Returns exactly one of two shapes:
  split:   { today, overnight }  both anchors known (or opened today,
                                 where overnight is honestly zero)
  unsplit: { day }               held overnight but no open print (quote
                                 source down): the change since yesterday's
                                 close, unsplittable
  neither: all nulls             nothing to anchor against
*/

export interface DayParts {
  today: number | null;
  overnight: number | null;
  day: number | null;
}

export function dayParts(p: { market_value: number; cost_basis: number; open_value?: number; prev_close_value?: number }): DayParts {
  const open = p.open_value ?? 0;
  const prevClose = p.prev_close_value ?? 0;
  const heldOvernight = prevClose > 0;

  if (heldOvernight && open > 0) {
    return { today: p.market_value - open, overnight: open - prevClose, day: null };
  }
  if (heldOvernight) {
    return { today: null, overnight: null, day: p.market_value - prevClose };
  }
  // Opened today: its whole life is the session; overnight is honestly 0.
  return { today: p.market_value - p.cost_basis, overnight: 0, day: null };
}

/*
aggregateOvernight sums the overnight gap across the book: only positions
held overnight (and with an open print) contribute; the caller derives the
session component as the residual against yesterday's account close so
realized intraday P&L stays included. Returns null when no position can
anchor an overnight, so the UI can fall back to the unsplit day change.
*/
export function aggregateOvernight(positions: Array<{ market_value: number; cost_basis: number; open_value?: number; prev_close_value?: number }>): number | null {
  let total = 0;
  let any = false;
  for (const p of positions) {
    const open = p.open_value ?? 0;
    const prevClose = p.prev_close_value ?? 0;
    if (prevClose > 0 && open > 0) {
      total += open - prevClose;
      any = true;
    }
  }
  return any ? total : null;
}
