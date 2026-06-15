#!/usr/bin/env python3
"""
Generate local/seed.sql for the VibeTradez local Docker stack.

AUTO-GENERATES realistic seed data for the autonomous portfolio manager so
the local dashboard renders something believable on a fresh boot:

  - a year of daily equity-curve points (account vs buy-and-hold SPY), as a
    regime-switching walk rather than smooth noise,
  - the last several daily sessions with varied, plausible stance notes,
  - the moves behind those sessions (equity, options, sells, holds) with
    realistic sizes and rationales,
  - a full mock session transcript for EACH of those days (the reasoning +
    tool calls, viewable at /transcript/<date>/portfolio),
  - a few subscribers.

The random seed is fixed so output is stable. Dates are relative to the day
the script runs. Re-run with:

    python3 local/generate-seed.py > local/seed.sql
"""

import random
import json
import uuid
from collections import defaultdict
from datetime import date, timedelta

random.seed(7)


def gen_uuid():
    """A deterministic, URL-safe UUID driven by the seeded RNG, so seed output
    stays stable while mirroring the gen_random_uuid() default the server uses
    for portfolio_decisions.uuid (the stable id behind each trade's route)."""
    return str(uuid.UUID(int=random.getrandbits(128), version=4))

# Rough price anchors so strikes, limits, and notionals look real.
PRICES = {
    "AAPL": 228, "MSFT": 432, "NVDA": 119, "MDT": 86, "AMD": 161,
    "GOOGL": 178, "AVGO": 174, "COST": 905, "PLTR": 63, "UNH": 521,
    "V": 291, "JPM": 214, "XOM": 117, "LLY": 905, "NFLX": 698,
}
SYMBOLS = list(PRICES)

SESSION_DAYS = 6  # how many recent days get a RICH session + moves + transcript
HISTORY_DAYS = 150  # window the completed round trips are scheduled across
NUM_CLOSED = 46  # how many completed round trips to generate (paginated on /closed)


def trading_days(n):
    out, d = [], date.today()
    while len(out) < n:
        if d.weekday() < 5:
            out.append(d)
        d -= timedelta(days=1)
    return list(reversed(out))


def sql_str(s):
    return "'" + str(s).replace("'", "''") + "'"


def gen_equity_curve(days):
    """Regime-switching walk: drift and vol shift every few weeks, so the
    curve has real-looking runs and drawdowns instead of smooth noise."""
    acct, spy, hwm = 5200.0, 561.0, 5200.0
    drift, vol = 0.0012, 0.014
    rows = []
    for i, d in enumerate(days):
        if i % random.randint(18, 34) == 0:
            drift = random.choice([0.0030, 0.0018, 0.0008, 0.0, -0.0018, -0.0032])
            vol = random.choice([0.009, 0.013, 0.019, 0.026])
        acct *= 1 + random.gauss(drift, vol)
        spy *= 1 + random.gauss(0.00045, 0.0085)
        acct = max(acct, 300.0)
        hwm = max(hwm, acct)
        invested = min(0.93, max(0.05, random.gauss(0.6, 0.18)))
        settled = round(acct * (1 - invested), 2)
        rows.append((d.isoformat(), round(acct, 2), settled, 0.0, round(hwm, 2), round(spy, 2)))
    return rows


STANCES = [
    "Net long and comfortable. The core (MDT, COST) is doing the heavy lifting, so I let it. Trimmed a little NVDA into strength to bank gains and keep dry powder.",
    "Raised cash today. Two positions hit my trim targets and nothing new was compelling enough to redeploy into, so I am happy sitting on dry powder.",
    "Quiet, deliberate session. Held everything and added a starter in AMD on the pullback to its 50-day. Still running a touch defensive versus the index.",
    "Leaning into the tape. Added to the winners and opened a small NVDA call for convexity, sized to premium I am fine writing off. Watching the inflation print midweek.",
    "Rotated. Closed PLTR after the run and moved the proceeds toward COST, which has been the steadiest line in the book.",
    "Defensive. The drawdown has me trimming rather than adding. Protecting the high-water mark matters more than chasing a bounce here.",
    "Flat by design. Everything I own still earns its slot and the setups I am watching have not triggered. Cash is a position.",
    "Pressed the strongest names a little. Concentration is intentional at this size: I would rather own more of real conviction than spread it thin.",
    "Took profits on the AMD trade and let MDT ride. A little lighter on options, heavier on equity I am willing to hold for weeks.",
    "Cautious add. Started UNH on weakness with a tight invalidation level. If it does not work I cut it fast, and there is still settled cash for one more name.",
    "Mostly housekeeping. Tilted toward the names beating the benchmark and away from the laggards. No new risk added.",
    "Opportunistic. A gap down in GOOGL gave a cleaner entry than I expected, so I sized up with conviction. Otherwise holding.",
    "Patient. The book is fine versus SPY and I see no reason to fiddle. Logged the watchlist for tomorrow and stepped back.",
    "Trimmed broadly into strength to bank some of the month's gains. Core intact, just lighter and with more cash than usual.",
    "Risk-off lean. Volatility ticked up, so I cut the weakest position and parked the cash. I would rather re-enter higher than ride a name I have lost conviction in.",
    "Added selectively and held the rest. I am already carrying enough option premium for my taste, so any new directional view goes through equity from here.",
]

SUMMARIES = [
    "Started the session a little defensive after the open. Read the book and the tape, confirmed the core (MDT, COST) still earns its slot, and trimmed a bit of NVDA into strength to bank some gains. Added a small starter where the setup was clean. Net long but comfortable, with cash held back as dry powder. Watching the inflation print midweek before doing anything bigger.",
    "Mostly housekeeping today. Nothing new cleared the bar, so I tilted toward the names beating the benchmark and away from the laggards rather than adding fresh risk. Confirmed a prior trim had filled, banked the gain, and left the rest of the book alone. Positioned roughly in line with SPY and content to wait for a better entry.",
    "Pressed the strongest names a touch and opened a small call for convexity into a catalyst, sized to premium I can lose without it hurting the book. Checked earnings dates first so I am not holding into a surprise print. Left the steady compounders untouched. Leaning long and a bit aggressive, but every position still has a defined invalidation level.",
    "Risk-off lean. Volatility ticked up, so I cut the weakest position, parked the proceeds in cash, and resisted the urge to chase the bounce. Protecting the high-water mark matters more than being a hero here. The core stays intact; I would rather re-enter higher than ride a name I have lost conviction in.",
    "Quiet, deliberate day. Reviewed every holding, found the theses intact, and added a starter in a name on a clean pullback to its 50-day. Re-checked a resting exit from a prior session, saw it had filled, and updated the book. Running a touch defensive versus the index and watching the watchlist for tomorrow.",
    "Took profits where the move had run and let the real conviction names ride. Rotated some of the proceeds into the steadiest line in the book rather than reaching for something new. A little lighter on options, heavier on equity I am happy to hold for weeks. Comfortable and patient from here.",
]

ACTION_ITEMS = [
    "Confirm the trim filled at the open. If MDT holds its 50-day, add the second tranche I sized for. Watch the inflation print midweek before deploying the rest of the cash.",
    "Nothing forcing my hand. Re-check the laggards for a reason to cut, watch GOOGL for a cleaner entry, and keep the dry powder until a setup actually triggers.",
    "Watch the call I opened into its catalyst; set a mental stop at the premium I paid. Reassess COST if it gaps. Earnings calendar is clear this week, so no print risk to manage.",
    "Defensive footing: if the drawdown deepens, trim the weakest name first. Otherwise hold and wait. Re-run the watchlist tomorrow and look for a re-entry higher.",
    "Confirm yesterday's exit actually settled. Look at adding to the winner on any pullback to the 20-day. Keep the option premium modest, I am already carrying a few calls.",
    "Let the winners run, no new risk planned. Check UNH into its invalidation level and cut it fast if it breaks. Otherwise patient, and reassess the cash level end of week.",
]

RATIONALE = {
    "buy_equity": [
        "Adding to the core on a pullback to the 50-day. Trend intact, volume confirming.",
        "Starting a position. Clean base, improving relative strength, plenty liquid to exit fast.",
        "Pressing a winner that keeps making higher highs.",
        "Weakness looks like noise, not a broken thesis. Buying the dip with a defined invalidation.",
        "Rotating proceeds from a trim into the steadier name in the book.",
    ],
    "buy_option": [
        "Small call for convexity into the catalyst. Defined risk, premium I am comfortable losing.",
        "Buying time, not lottery tickets: enough DTE that theta is not punishing, strike where the chain is liquid.",
        "Cheap optionality on a name I already like. If it works I ride it, if not the premium is the max loss.",
    ],
    "sell_equity": [
        "Trimming into strength to bank some gains and free up cash.",
        "Thesis intact but the position got too big after the run. Right-sizing it.",
        "Cutting a laggard that stopped earning its slot. Freeing capital for a better setup.",
        "Hit my trim target. Taking part of the move and letting the rest run.",
    ],
    "sell_option": [
        "Closing the call into the rip. Took the move, no reason to round-trip the premium.",
        "Rolling risk off as expiration approaches rather than holding into the theta cliff.",
    ],
    "hold": [
        "Looked, liked what I own, changed nothing. The watchlist setups have not triggered.",
        "Nothing cleared the bar today. Holding the book and keeping the cash as dry powder.",
        "Comfortable with current exposure. Adding here would just be activity for its own sake.",
    ],
}


def occ(symbol, exp, kind, strike):
    return f"{symbol.ljust(6)}{exp.strftime('%y%m%d')}{'C' if kind == 'CALL' else 'P'}{int(round(strike * 1000)):08d}"


def jitter(px):
    return round(px * random.uniform(0.985, 1.015), 2)


def make_decision(d, action):
    ds = d.isoformat()
    sym = random.choice(SYMBOLS)
    px = jitter(PRICES[sym])
    if action == "buy_option":
        exp = d + timedelta(days=random.choice([21, 28, 35, 49]))
        strike = round(px * random.uniform(1.02, 1.08))
        contracts = random.randint(1, 3)
        limit = round(random.uniform(0.9, 4.2), 2)
        return dict(date=ds, action="buy_option", asset_type="OPTION",
                    symbol=occ(sym, exp, "CALL", strike), underlying=sym,
                    contract_type="CALL", strike=strike, expiration=exp.isoformat(),
                    quantity=contracts, limit_price=limit,
                    notional=round(contracts * limit * 100, 2), status="submitted",
                    rationale=random.choice(RATIONALE["buy_option"]))
    if action == "sell_option":
        exp = d + timedelta(days=random.choice([14, 21, 28]))
        strike = round(px * random.uniform(1.01, 1.06))
        contracts = random.randint(1, 2)
        limit = round(random.uniform(1.2, 5.0), 2)
        return dict(date=ds, action="sell_option", asset_type="OPTION",
                    symbol=occ(sym, exp, "CALL", strike), underlying=sym,
                    contract_type="CALL", strike=strike, expiration=exp.isoformat(),
                    quantity=contracts, limit_price=limit,
                    notional=round(contracts * limit * 100, 2), status="submitted",
                    rationale=random.choice(RATIONALE["sell_option"]))
    if action == "hold":
        return dict(date=ds, action="hold", asset_type="", symbol="", underlying="",
                    contract_type="", strike=None, expiration=None, quantity=0,
                    limit_price=0, notional=0, status="",
                    rationale=random.choice(RATIONALE["hold"]))
    # buy_equity / sell_equity
    qty = random.randint(2, max(2, int(1400 / px)) + 2)
    return dict(date=ds, action=action, asset_type="EQUITY", symbol=sym, underlying=sym,
                contract_type="", strike=None, expiration=None, quantity=qty,
                limit_price=px, notional=round(qty * px, 2), status="submitted",
                rationale=random.choice(RATIONALE[action]))


def gen_positions(day, equity, settled_cash):
    """The currently-held book, sized so it sums to the invested portion of
    the latest equity-curve point (equity minus cash). Mixed equity + one
    option, with varied unrealized P&L."""
    target = max(0.0, equity - settled_cash)
    if target < 120:
        return []
    names = random.sample(SYMBOLS, k=random.randint(3, 4))
    weights = [random.uniform(0.6, 1.6) for _ in names]
    tot = sum(weights)
    rows = []
    for i, (sym, w) in enumerate(zip(names, weights)):
        slice_mv = target * w / tot
        if slice_mv < 80:
            continue
        pnl = random.uniform(-0.13, 0.30)  # unrealized return vs cost
        px = PRICES[sym]
        if i == len(names) - 1 and len(names) >= 3:
            exp = day + timedelta(days=random.choice([21, 35, 49]))
            strike = round(px * random.uniform(1.02, 1.08))
            contracts = max(1, round(slice_mv / (px * 0.025 * 100)))
            mark = round(slice_mv / (contracts * 100), 2)
            mv = round(contracts * mark * 100, 2)
            rows.append(dict(symbol=occ(sym, exp, "CALL", strike), underlying=sym,
                             asset_type="OPTION", contract_type="CALL", strike=strike,
                             expiration=exp.isoformat(), quantity=contracts,
                             market_value=mv, cost_basis=round(mv / (1 + pnl), 2)))
        else:
            p = jitter(px)
            qty = max(1, round(slice_mv / p))
            mv = round(qty * p, 2)
            rows.append(dict(symbol=sym, underlying=sym, asset_type="EQUITY",
                             contract_type="", strike=None, expiration=None,
                             quantity=qty, market_value=mv, cost_basis=round(mv / (1 + pnl), 2)))
    return rows


def opening_buy_for_position(pos, day):
    """The buy that opened a currently-held position, so /holdings can show
    the entry rationale and date. Symbol matches the snapshot exactly."""
    ds = day.isoformat()
    qty = pos["quantity"]
    if pos["asset_type"] == "OPTION":
        entry = round(pos["cost_basis"] / (qty * 100), 2)
        return dict(date=ds, action="buy_option", asset_type="OPTION",
                    symbol=pos["symbol"], underlying=pos["underlying"],
                    contract_type=pos["contract_type"], strike=pos["strike"],
                    expiration=pos["expiration"], quantity=qty, limit_price=entry,
                    notional=round(qty * entry * 100, 2), status="filled",
                    rationale=random.choice(RATIONALE["buy_option"]))
    entry = round(pos["cost_basis"] / qty, 2)
    return dict(date=ds, action="buy_equity", asset_type="EQUITY",
                symbol=pos["symbol"], underlying=pos["underlying"],
                contract_type="", strike=None, expiration=None, quantity=qty,
                limit_price=entry, notional=round(qty * entry, 2), status="filled",
                rationale=random.choice(RATIONALE["buy_equity"]))


def make_closed_trade(underlying, kind, open_day, close_day, win):
    """A completed round trip: the buy that opened it and the sell that
    flattened it. Returns (buy, sell). The backend derives realized P&L by
    pairing these two in the decision log, so the symbol must match."""
    px = PRICES[underlying]
    if kind == "option":
        exp = close_day + timedelta(days=random.choice([10, 17, 24]))
        strike = round(px * random.uniform(1.02, 1.07))
        contracts = random.randint(1, 3)
        entry = round(random.uniform(1.1, 3.8), 2)
        mult = random.uniform(1.30, 1.95) if win else random.uniform(0.30, 0.78)
        exit_px = round(max(0.05, entry * mult), 2)
        sym = occ(underlying, exp, "CALL", strike)
        buy = dict(date=open_day.isoformat(), action="buy_option", asset_type="OPTION",
                   symbol=sym, underlying=underlying, contract_type="CALL", strike=strike,
                   expiration=exp.isoformat(), quantity=contracts, limit_price=entry,
                   notional=round(contracts * entry * 100, 2), status="filled",
                   rationale=random.choice(RATIONALE["buy_option"]))
        sell = dict(date=close_day.isoformat(), action="sell_option", asset_type="OPTION",
                    symbol=sym, underlying=underlying, contract_type="CALL", strike=strike,
                    expiration=exp.isoformat(), quantity=contracts, limit_price=exit_px,
                    notional=round(contracts * exit_px * 100, 2), status="filled",
                    rationale=random.choice(RATIONALE["sell_option"]))
        return buy, sell
    entry = jitter(px)
    mult = random.uniform(1.04, 1.17) if win else random.uniform(0.86, 0.985)
    exit_px = round(entry * mult, 2)
    qty = random.randint(3, max(3, int(1200 / entry)) + 3)
    buy = dict(date=open_day.isoformat(), action="buy_equity", asset_type="EQUITY",
               symbol=underlying, underlying=underlying, contract_type="", strike=None,
               expiration=None, quantity=qty, limit_price=entry,
               notional=round(qty * entry, 2), status="filled",
               rationale=random.choice(RATIONALE["buy_equity"]))
    sell = dict(date=close_day.isoformat(), action="sell_equity", asset_type="EQUITY",
                symbol=underlying, underlying=underlying, contract_type="", strike=None,
                expiration=None, quantity=qty, limit_price=exit_px,
                notional=round(qty * exit_px, 2), status="filled",
                rationale=random.choice(RATIONALE["sell_equity"]))
    return buy, sell


def gen_closed_round_trips(window, symbols, n):
    """Schedule n completed round trips across the window, reusing tickers in
    NON-OVERLAPPING cycles. The backend pairs buys with sells per symbol walking
    the log chronologically, so each fully-closed equity cycle on a ticker is its
    own trade as long as it closes before the next one opens; option cycles use
    unique OCC symbols. Mix of winners and losers, varied hold times and sizes."""
    trips = []
    eq_free = {s: 0 for s in symbols}  # earliest open index still free per equity ticker
    seen_occ = set()
    attempts = 0
    while len(trips) < n and attempts < n * 40:
        attempts += 1
        kind = "option" if random.random() < 0.32 else "equity"
        sym = random.choice(symbols)
        if kind == "equity":
            start = eq_free[sym]
            if start > len(window) - 4:
                continue
            oidx = random.randint(start, len(window) - 4)
            cidx = min(len(window) - 1, oidx + random.randint(2, 22))
            if cidx <= oidx:
                continue
            buy, sell = make_closed_trade(sym, "equity", window[oidx], window[cidx], random.random() < 0.58)
            eq_free[sym] = cidx + 1  # next cycle on this ticker must open after this one closes
            trips.append((buy, sell))
        else:
            oidx = random.randint(0, len(window) - 4)
            cidx = min(len(window) - 1, oidx + random.randint(3, 18))
            if cidx <= oidx:
                continue
            buy, sell = make_closed_trade(sym, "option", window[oidx], window[cidx], random.random() < 0.55)
            if buy["symbol"] in seen_occ:  # identical OCC would merge; skip and retry
                continue
            seen_occ.add(buy["symbol"])
            trips.append((buy, sell))
    trips.sort(key=lambda bs: bs[0]["date"])
    return trips


def build_decisions(decision_days, session_days, positions):
    """Build a coherent decision log: an opening buy for each held position
    plus a set of completed round trips (mix of equity + options, winners +
    losers). Returns {date: [decisions]} grouped across decision_days, with a
    hold filled in on any session day that would otherwise be empty."""
    by_date = defaultdict(list)
    held_names = {p["underlying"] for p in positions}

    # Opening buys for the held book, dated in the recent stretch so each
    # holding reads as opened weeks (not months) ago.
    recent_start = max(0, len(decision_days) - 18)
    for pos in positions:
        od = decision_days[random.randint(recent_start, len(decision_days) - 2)]
        # Remember the open on the position itself: the snapshot emitter
        # backfills a per-day value row for every curve day since the open,
        # so the trade-detail value chart has a real series to draw.
        pos["opened"] = od.isoformat()
        by_date[od.isoformat()].append(opening_buy_for_position(pos, od))

    # A deep set of completed round trips, on names not currently held (so each
    # holding's opening rationale stays unambiguous), reused across the window in
    # non-overlapping cycles. This is what fills the paginated /closed page.
    avail = [s for s in SYMBOLS if s not in held_names]
    closed_names = set()
    trips = list(gen_closed_round_trips(decision_days, avail, NUM_CLOSED))

    # Close one option round trip ON the latest session day, so today's
    # transcript (the kitchen-sink session the dashboard links to) narrates a
    # real sell_option ticket alongside the equity moves. The contract is
    # re-dated to expire shortly after today (and its OCC symbol regenerated
    # on both legs so the buy/sell pairing holds), otherwise the retargeted
    # close would show the model selling an already-expired option.
    today = session_days[-1]
    today_iso = today.isoformat()
    if not any(s["date"] == today_iso and s["action"] == "sell_option" for _b, s in trips):
        taken = {b["symbol"] for b, _s in trips}
        for buy, sell in trips:
            if buy["action"] == "buy_option" and buy["date"] < today_iso and sell["date"] < today_iso:
                sell["date"] = today_iso
                new_exp = today + timedelta(days=12)
                strike = buy["strike"]
                new_sym = occ(buy["underlying"], new_exp, "CALL", strike)
                while new_sym in taken:
                    strike += 1
                    new_sym = occ(buy["underlying"], new_exp, "CALL", strike)
                for leg in (buy, sell):
                    leg["symbol"] = new_sym
                    leg["strike"] = strike
                    leg["expiration"] = new_exp.isoformat()
                break

    for buy, sell in trips:
        by_date[buy["date"]].append(buy)
        by_date[sell["date"]].append(sell)
        closed_names.add(buy["underlying"])

    # Give the latest session day (the dashboard's "today") a real mix of
    # moves: a starter buy on a fresh name, a small trim of a held equity,
    # and an explicit hold, each with its own rationale.
    today = session_days[-1]
    ds = today.isoformat()
    used = set(held_names) | closed_names
    fresh = next((s for s in SYMBOLS if s not in used), None) or next((s for s in SYMBOLS if s not in held_names), None)
    if fresh:
        px = jitter(PRICES[fresh])
        qty = random.randint(2, max(2, int(1000 / px)) + 2)
        by_date[ds].append(dict(date=ds, action="buy_equity", asset_type="EQUITY", symbol=fresh,
                                underlying=fresh, contract_type="", strike=None, expiration=None,
                                quantity=qty, limit_price=px, notional=round(qty * px, 2),
                                status="submitted", rationale=random.choice(RATIONALE["buy_equity"])))
    # A small option opener for today as well, so the latest session shows
    # every order-ticket shape the transcript UI renders (buy_equity,
    # sell_equity, buy_option, sell_option, cancel_order, hold).
    opt_und = next((s for s in SYMBOLS if s not in used and s != fresh), None) or next(
        (s for s in SYMBOLS if s not in held_names and s != fresh), "MSFT"
    )
    opx = PRICES[opt_und]
    oexp = today + timedelta(days=38)
    ostrike = round(opx * 1.05)
    olimit = round(opx * 0.018, 2)
    by_date[ds].append(dict(date=ds, action="buy_option", asset_type="OPTION",
                            symbol=occ(opt_und, oexp, "CALL", ostrike), underlying=opt_und,
                            contract_type="CALL", strike=ostrike, expiration=oexp.isoformat(),
                            quantity=1, limit_price=olimit, notional=round(olimit * 100, 2),
                            status="submitted", rationale=random.choice(RATIONALE["buy_option"])))
    held_equities = [p for p in positions if p["asset_type"] == "EQUITY"]
    trimmed = None
    if held_equities:
        hp = held_equities[0]
        trimmed = hp["underlying"]
        trim_qty = max(1, int(hp["quantity"] // 4))
        if trim_qty >= hp["quantity"]:
            trim_qty = 1
        tpx = round(hp["cost_basis"] / hp["quantity"] * random.uniform(1.02, 1.10), 2)
        by_date[ds].append(dict(date=ds, action="sell_equity", asset_type="EQUITY", symbol=hp["underlying"],
                                underlying=hp["underlying"], contract_type="", strike=None, expiration=None,
                                quantity=trim_qty, limit_price=tpx, notional=round(trim_qty * tpx, 2),
                                status="submitted", rationale=random.choice(RATIONALE["sell_equity"])))
    # Every other position carried gets an explicit per-position hold for the day.
    for pos in positions:
        if pos["underlying"] == trimmed:
            continue
        by_date[ds].append(dict(date=ds, action="hold", asset_type=pos["asset_type"], symbol=pos["symbol"],
                                underlying=pos["underlying"], contract_type=pos["contract_type"], strike=pos["strike"],
                                expiration=pos["expiration"], quantity=0, limit_price=0, notional=0, status="",
                                rationale=random.choice(RATIONALE["hold"])))

    # Every session day should narrate something; fill empties with a hold.
    for d in session_days:
        if not by_date[d.isoformat()]:
            by_date[d.isoformat()].append(make_decision(d, "hold"))

    # The round-trip pairs ride along so the snapshot emitter can backfill
    # per-day book rows for the holding period of every closed trade (the
    # closed-trade detail chart reads those snapshots).
    return by_date, trips


def fmt_money(x):
    return f"${x:,.0f}"


def transcript_for_day(decisions, stance, summary, action_items, positions, kitchen_sink=False):
    """A cohesive, realistic single-day session: read the book, confirm and
    clean up a stale working order, research a name with the reading tools and
    the news, weigh an option and pass, size against the settled cash, place
    the day's moves, then document the session. Tool results mirror the
    production tool layer's shapes with believable numbers anchored to PRICES.

    kitchen_sink=True (the most recent session day, the one the dashboard
    links to) widens the session so every transcript render case appears on
    one page: the portfolio tools, the Anthropic-run web and sandbox tools
    in both success and failure states, a settled-cash refusal, a dangling
    tool_use with no recorded result, an empty-string tool_result, a long
    multi-symbol quotes sweep, and long multi-paragraph thinking."""
    ev, state = [], {"r": 0}

    def use(tool, inp):
        ev.append({"round": state["r"], "type": "tool_use", "tool_name": tool, "tool_input": inp})

    def res(tool, out):
        ev.append({"round": state["r"], "type": "tool_result", "tool_name": tool, "tool_result": out if isinstance(out, str) else json.dumps(out)})

    def say(t):
        ev.append({"round": state["r"], "type": "text", "text": t})

    def think(t):
        ev.append({"round": state["r"], "type": "thinking", "text": t})

    def step():
        state["r"] += 1

    cash = 1480.42
    invested = sum(p["market_value"] for p in positions)
    equity = round(invested + cash, 2)
    hwm = round(equity * 1.031, 2)
    held_option = next((p for p in positions if p["asset_type"] == "OPTION"), None)
    core = positions[0]["underlying"] if positions else "MDT"
    oid = [3100]

    def next_oid():
        oid[0] += 1
        return f"live-{oid[0]}"

    buys = [d for d in decisions if d["action"] in ("buy_equity", "buy_option")]
    sells = [d for d in decisions if d["action"] in ("sell_equity", "sell_option")]
    holds = [d for d in decisions if d["action"] == "hold"]

    # ── Round 0: read the account and recent history ──
    say("Starting today's session. Before I touch anything I read the live book, my recent moves, and any order I left working yesterday.")
    if kitchen_sink:
        think(
            "Three things to settle before I place anything today.\n\n"
            "First, the stale trim. I left a limit on " + core + " working overnight and I have no idea whether it filled. "
            "If it did, the book I am about to read already reflects a smaller position and re-trimming would cut the name twice. "
            "If it did not, the limit is probably miles from the market by now and needs to be repriced, not left to rot on the order book.\n\n"
            "Second, sizing discipline. The last few sessions added exposure, so before any new buy I want the order sized in the sandbox, "
            "not my mental arithmetic, so the share count is exact against the settled cash on hand.\n\n"
            "Third, the watchlist. Two names pulled back to support and one of the held calls is close to its catalyst. The plan is one clean starter, "
            "one small call if the chain is liquid enough, the repriced trim, and explicit holds on everything else so the record shows a decision on every position."
        )
    use("get_portfolio", {})
    res("get_portfolio", {
        "equity": equity, "settled_cash": cash, "unsettled_cash": 0.0, "high_water_mark": hwm,
        "positions": [{"symbol": p["symbol"], "underlying": p["underlying"], "asset_type": p["asset_type"],
                       "quantity": p["quantity"], "market_value": p["market_value"], "cost_basis": p["cost_basis"],
                       "unrealized_pnl": round(p["market_value"] - p["cost_basis"], 2)} for p in positions],
    })
    use("get_recent_decisions", {"limit": 10})
    res("get_recent_decisions", {
        "decisions": [
            {"date": "(yesterday)", "action": "sell_equity", "symbol": core, "notional": round(PRICES.get(core, 100) * 4, 2),
             "rationale": "Left a limit to trim the winner into strength."},
            {"date": "(this week)", "action": "buy_equity", "symbol": core, "notional": round(PRICES.get(core, 100) * 6, 2),
             "rationale": "Added to the core on the pullback."},
            {"date": "(this week)", "action": "hold", "symbol": "", "notional": 0,
             "rationale": "Nothing cleared the bar. Cash stayed in reserve."},
        ],
        "stances": [
            {"date": "(yesterday)", "stance": "Net long and comfortable, leaning on the core names."},
            {"date": "(two days ago)", "stance": "Trimmed into strength and kept the proceeds as dry powder."},
        ],
        "prior_synopsis": "Trimmed the winner into strength and left the rest of the book alone. Cash held back for a cleaner entry.",
        "prior_action_items": "Confirm the trim filled at the open. Re-check the laggards for a reason to cut. Keep the dry powder until a setup actually triggers.",
    })
    step()

    # ── Round 1: confirm the working order, find it stale, cancel it ──
    think("Yesterday I left a limit to trim " + core + ". Before I act on that name again I need to know whether it filled, so I am not trading a position I have already changed.")
    use("get_order_status", {"order_id": "live-2841"})
    res("get_order_status", {"order_id": "live-2841", "status": "WORKING", "filled": False, "working": True, "terminal": False,
                             "quantity": 4, "filled_quantity": 0, "fill_price": 0, "limit_price": round(PRICES.get(core, 100) * 1.05, 2)})
    think("It never filled and the limit is now well above the market, so it is just sitting there stale. I will cancel it and re-submit the trim at a price that will actually execute once I have done my research.")
    use("cancel_order", {"order_id": "live-2841", "rationale": "Stale trim limit priced above the market. Cancelling so I can re-submit it to fill."})
    res("cancel_order", {"ok": True, "action": "cancel_order", "order_id": "live-2841"})
    say("Cleared the stale order. The book above is current. Now I will look for one clean idea to add and right-size anything that has run too far.")
    step()

    # ── Round 2: weigh adding a call and pass on liquidity ──
    chain_und = held_option["underlying"] if held_option is not None else core
    if held_option is not None or kitchen_sink:
        pu = PRICES.get(chain_und, 150)
        if held_option is not None:
            say("I already hold a call on " + chain_und + ", so I checked the chain to see whether adding more optionality made sense.")
        else:
            say("Before anything else I checked the " + chain_und + " chain to see whether a defined-risk call was a better way to express the add.")
        use("get_option_chain", {"symbol": chain_und, "contract_type": "CALL", "from_date": "(near month)", "to_date": "(next month)"})
        chain_exp = "(five weeks out)"
        res("get_option_chain", {
            "symbol": chain_und, "status": "SUCCESS", "underlyingPrice": pu, "volatility": 22.8,
            "calls": [
                {"putCall": "CALL", "strikePrice": round(pu * 1.02), "bid": 4.1, "ask": 4.5, "last": 4.3, "mark": 4.3,
                 "totalVolume": 410, "openInterest": 1850, "delta": 0.46, "expirationDate": chain_exp, "daysToExpiration": 36},
                {"putCall": "CALL", "strikePrice": round(pu * 1.05), "bid": 2.1, "ask": 2.9, "last": 2.4, "mark": 2.5,
                 "totalVolume": 22, "openInterest": 180, "delta": 0.38, "expirationDate": chain_exp, "daysToExpiration": 36},
                {"putCall": "CALL", "strikePrice": round(pu * 1.08), "bid": 1.2, "ask": 1.9, "last": 1.5, "mark": 1.55,
                 "totalVolume": 9, "openInterest": 64, "delta": 0.27, "expirationDate": chain_exp, "daysToExpiration": 36},
                {"putCall": "CALL", "strikePrice": round(pu * 1.11), "bid": 0.6, "ask": 1.3, "last": 0.9, "mark": 0.95,
                 "totalVolume": 3, "openInterest": 21, "delta": 0.18, "expirationDate": chain_exp, "daysToExpiration": 36},
            ],
        })
        think("Only the near-the-money strike has real open interest and the spreads widen fast above it. Adding here would be paying up for poor liquidity, so I pass on this chain and keep the exposure I already own.")
        step()

    # ── Kitchen sink: sweep the wider watchlist in one long quote call ──
    if kitchen_sink:
        say("Next a quick sweep of the wider watchlist, so the moves below are sized against the whole tape and not just the names I am touching.")
        watch = SYMBOLS[:8]
        use("get_stock_quotes", {"symbols": ",".join(watch)})
        res("get_stock_quotes", {w: {"last": PRICES[w], "bid": round(PRICES[w] - 0.05, 2), "ask": round(PRICES[w] + 0.06, 2),
                                     "mark": PRICES[w], "totalVolume": 4_000_000 + (i * 3_137_000) % 21_000_000}
                                 for i, w in enumerate(watch)})
        think(
            "Reading the sweep. The megacaps are flat to slightly green and nothing on the list gapped overnight, which means the pullback names "
            "I flagged yesterday are still sitting at their levels rather than running away from me. Volume is unremarkable across the board, "
            "so whatever I do today is a liquidity non-event.\n\n"
            "The two names worth acting on are the starter candidate, which held its 50-day on yesterday's dip, and the trim, which is still "
            "pressing against its highs. Everything else is mid-range noise. No reason to widen the session beyond the plan from round one."
        )
        step()

    # ── Research + place each buy ──
    did_news, did_code, did_deep, did_refusal = False, False, False, False

    for dec in buys:
        und = dec["underlying"]
        px = PRICES.get(und) or round(dec["limit_price"], 2)
        if dec["action"] == "buy_equity":
            say(und + " pulled back to its rising 50-day on no bad news, so I am checking the quote, the trend, the fundamentals, and the headlines before sizing anything.")
        else:
            say("The call idea is " + und + ". Same drill before any premium goes out: quote, trend, fundamentals, and the earnings date.")
        use("get_stock_quotes", {"symbols": und})
        res("get_stock_quotes", {und: {"last": px, "bid": round(px - 0.05, 2), "ask": round(px + 0.05, 2), "mark": px, "totalVolume": 18_500_000}})
        use("get_price_history", {"symbol": und})
        res("get_price_history", {"symbol": und, "last_close": px, "sma_20": round(px * 0.99, 2), "sma_50": round(px * 0.97, 2),
                                  "sma_200": round(px * 0.90, 2), "high_52w": round(px * 1.068, 2), "low_52w": round(px * 0.71, 2),
                                  "pct_from_52w_high": -6.4, "pct_from_52w_low": 40.8, "return_1m_pct": 2.1, "return_3m_pct": 5.2,
                                  "annualized_vol_20d_pct": 24.1, "trading_days": 251})
        use("get_fundamentals", {"symbol": und})
        res("get_fundamentals", {"symbol": und, "market_cap": int(px * 12_800_000_000), "pe_ratio": 31.4, "eps_ttm": round(px / 31.4, 2),
                                 "high_52": round(px * 1.068, 2), "low_52": round(px * 0.71, 2), "beta": 1.05,
                                 "dividend_yield": 0.44, "dividend_amount": round(px * 0.0044, 2),
                                 "next_earnings_date": "2026-07-23", "avg_10_day_volume": 16_400_000})
        if not did_news:
            use("web_search", {"query": und + " stock news catalyst this week"})
            res("web_search", "Top results: (1) " + und + " expands its enterprise AI tier, bookings up double digits. (2) Two analysts reiterate buy and nudge targets higher. No negative catalyst before the late-July print.")
            use("web_fetch", {"url": "https://www.reuters.com/markets/companies/" + und.lower()})
            res("web_fetch", "Reuters: " + und + " reaffirmed full-year guidance and flagged steady enterprise demand. Nothing in the coverage points to a catalyst that would derail a multi-week hold. Saved the article text to the sandbox for a closer pass.")
            did_news = True
        if kitchen_sink and not did_deep:
            use("web_fetch", {"url": "https://www.wsj.com/finance/stocks/" + und.lower() + "-outlook"})
            res("web_fetch", "")
            say("The second fetch came back empty, that page is paywalled. The Reuters text I saved is enough, so I am grepping it for the guidance language instead.")
            use("bash_code_execution", {"command": "grep -ic 'guidance' /tmp/reuters_" + und.lower() + ".txt"})
            res("bash_code_execution", {"error_code": "", "type": "encrypted_bash_code_execution_result", "content": [],
                                        "return_code": 0, "stdout": "3\n", "stderr": ""})
            think("Guidance comes up three times in the piece and every mention is a reaffirmation. That settles the print-risk question for this week.")
            use("text_editor_code_execution", {"command": "create", "path": "/tmp/session_notes.md",
                                               "file_text": "# Session notes\n- " + und + ": trend intact, guidance reaffirmed, no print risk this week\n- Trim goes out priced at the bid\n- Order sized against settled cash before placing\n"})
            res("text_editor_code_execution", {"error_code": "", "type": "encrypted_text_editor_code_execution_result", "content": [],
                                              "is_file_update": False})
            did_deep = True
        think("Trend is intact and above the 50- and 200-day, earnings are not until late July so there is no print risk this week, and the spread is a penny. A starter fits cleanly within today's settled cash.")
        if not did_code:
            # The code-execution sandbox is auto-enabled by the web tools; here
            # the model uses it to size the order precisely against the settled
            # cash on hand instead of eyeballing it.
            think("Rather than eyeball the size, I will compute the largest whole-share order the settled cash on hand can fund, then round down for a little slack.")
            budget = round(cash, 2)
            shares = int(budget // px)
            # First attempt fails (sandbox cells share no state, so the bare
            # variables NameError). Both results mirror the real
            # encrypted_code_execution_result shape, error_code present and
            # empty on success, so the transcript UI's success/error
            # classification and the red error pill render against seed data.
            use("code_execution", {"code": (
                "shares = int(settled_cash // price)\n"
                "print(f'shares={shares} notional={shares * price:.2f}')"
            )})
            res("code_execution", {"error_code": "", "type": "encrypted_code_execution_result", "content": [], "return_code": 1, "stdout": "",
                                   "stderr": "Traceback (most recent call last):\n  File \"<stdin>\", line 1, in <module>\nNameError: name 'settled_cash' is not defined\n"})
            think("Forgot the sandbox cell has no shared state, so the inputs were undefined. Re-running with the constants inlined.")
            use("code_execution", {"code": (
                f"price = {px}\n"
                f"settled_cash = {round(cash, 2)}\n"
                "shares = int(settled_cash // price)\n"
                "print(f'shares={shares} notional={shares * price:.2f} cash_left={settled_cash - shares * price:.2f}')"
            )})
            res("code_execution", {"error_code": "", "type": "encrypted_code_execution_result", "content": [], "return_code": 0,
                                   "stdout": f"shares={shares} notional={shares * px:.2f} cash_left={budget - shares * px:.2f}\n", "stderr": ""})
            did_code = True
        if kitchen_sink and not did_refusal and dec["action"] == "buy_equity":
            # The model first tries a size that needs more than the settled cash
            # on hand and the settled-cash gate refuses it, mirroring the
            # production refusal format. The error path renders on a real order
            # ticket this way.
            qty_over = int(cash // dec["limit_price"]) + 6
            notional_over = round(qty_over * dec["limit_price"], 2)
            think("Conviction wants a bigger position than today's settled cash can fund. The only thing that can stop me is whether the cash has settled, so let the tool check it.")
            use("buy_equity", {"symbol": dec["symbol"], "quantity": qty_over, "limit_price": dec["limit_price"],
                               "rationale": "Full-conviction size on the pullback while the setup is clean."})
            res("buy_equity", {"error": f"buy_equity refused (settled_cash): order notional ${notional_over:.2f} exceeds settled cash ${round(cash, 2):.2f}. "
                                        f"Unsettled proceeds cannot be redeployed until they settle (T+1)."})
            say("Not enough settled cash for the oversized order, which is the broker's line and not a risk rule. Sizing down to what the cash on hand actually buys.")
            did_refusal = True
        say(dec["rationale"])
        if dec["action"] == "buy_equity":
            use("buy_equity", {"symbol": dec["symbol"], "quantity": dec["quantity"], "limit_price": dec["limit_price"], "rationale": dec["rationale"]})
        else:
            use("buy_option", {"occ_symbol": dec["symbol"], "underlying": und, "contract_type": dec.get("contract_type", "CALL"),
                               "strike": dec["strike"], "expiration": dec["expiration"], "contracts": dec["quantity"], "limit_price": dec["limit_price"], "rationale": dec["rationale"]})
        res(dec["action"], {"ok": True, "action": dec["action"], "symbol": dec["symbol"], "quantity": dec["quantity"],
                            "limit_price": dec["limit_price"], "notional": dec["notional"], "order_id": next_oid(), "execution_id": oid[0],
                            "settled_cash_left": round(cash - dec["notional"], 2)})
        step()

    # ── Place each sell, priced to fill ──
    last_sell_oid = "live-2841"
    for dec in sells:
        und = dec["underlying"]
        px = PRICES.get(und, round(dec["limit_price"], 2))
        if dec["action"] == "sell_equity":
            say("Re-submitting the trim on " + und + ", this time priced at the bid so it actually fills rather than sitting there.")
        else:
            say("The " + und + " call did its job and the move I bought it for has played out. Closing it rather than round-tripping the premium.")
        use("get_stock_quotes", {"symbols": und})
        res("get_stock_quotes", {und: {"last": px, "bid": round(px - 0.06, 2), "ask": round(px + 0.04, 2), "mark": px}})
        say(dec["rationale"])
        if dec["action"] == "sell_equity":
            use("sell_equity", {"symbol": dec["symbol"], "quantity": dec["quantity"], "limit_price": dec["limit_price"], "rationale": dec["rationale"]})
        else:
            use("sell_option", {"occ_symbol": dec["symbol"], "contracts": dec["quantity"], "limit_price": dec["limit_price"], "rationale": dec["rationale"]})
        last_sell_oid = next_oid()
        res(dec["action"], {"ok": True, "action": dec["action"], "symbol": dec["symbol"], "quantity": dec["quantity"],
                            "limit_price": dec["limit_price"], "order_id": last_sell_oid, "execution_id": oid[0]})
        step()

    # ── Every other position gets an explicit hold, logged by name ──
    named_holds = [h for h in holds if h.get("symbol")]
    if named_holds:
        say("The rest of the book still earns its slot, so I am holding each name unchanged and logging the decision.")
        for dec in named_holds:
            use("hold", {"symbol": dec["symbol"], "rationale": dec["rationale"]})
            res("hold", {"ok": True, "action": "hold", "symbol": dec["symbol"]})
        if kitchen_sink:
            use("hold", {"rationale": "Keeping the rest of the settled cash in reserve. Nothing else on the watchlist cleared the bar today."})
            res("hold", {"ok": True, "action": "hold"})
        step()
    else:
        use("hold", {"rationale": holds[0]["rationale"] if holds else "Opened nothing new; holding cash as dry powder."})
        res("hold", {"ok": True, "action": "hold"})
        step()

    # ── Documentation: record the day's summary, then sign off with the stance ──
    use("write_summary", {"synopsis": summary, "action_items": action_items})
    res("write_summary", {"ok": True, "action": "write_summary", "stored": True})
    say(stance)
    if kitchen_sink:
        # One last status read on the resting trim, issued right as the
        # session wrapped. The recorder closed before the result landed, so
        # the transcript carries a tool_use with no recorded result.
        use("get_order_status", {"order_id": last_sell_oid})
    return ev


def mini_transcript_for_day(decisions):
    """A lightweight but coherent session for a historical day: read the book,
    then place each move with the EXACT rationale stored on its decision, and
    sign off. This is what makes 'View the opening/closing session' on a
    /holdings or /closed detail page resolve to a real transcript whose stated
    reason matches the card. Used for every day that isn't one of the recent
    rich sessions."""
    ev, state = [], {"r": 0}

    def use(tool, inp):
        ev.append({"round": state["r"], "type": "tool_use", "tool_name": tool, "tool_input": inp})

    def res(tool, out):
        ev.append({"round": state["r"], "type": "tool_result", "tool_name": tool, "tool_result": out if isinstance(out, str) else json.dumps(out)})

    def say(t):
        ev.append({"round": state["r"], "type": "text", "text": t})

    def think(t):
        ev.append({"round": state["r"], "type": "thinking", "text": t})

    def step():
        state["r"] += 1

    oid = [4200]

    def next_oid():
        oid[0] += 1
        return f"live-{oid[0]}"

    buys = [d for d in decisions if d["action"] in ("buy_equity", "buy_option")]
    sells = [d for d in decisions if d["action"] in ("sell_equity", "sell_option")]
    holds = [d for d in decisions if d["action"] == "hold"]

    say("Opening the session. Reading the live book and my recent moves before I decide anything.")
    use("get_portfolio", {})
    res("get_portfolio", {"note": "book read; positions and cash current"})
    step()

    for dec in buys:
        und = dec["underlying"]
        px = PRICES.get(und, round(dec["limit_price"], 2))
        say(und + " is back on my radar. Checking the quote and the trend before I size anything.")
        use("get_stock_quotes", {"symbols": und})
        res("get_stock_quotes", {und: {"last": px, "bid": round(px - 0.05, 2), "ask": round(px + 0.05, 2), "mark": px, "totalVolume": 14_200_000}})
        think("Setup is clean and there is settled cash to spare. Taking the position.")
        say(dec["rationale"])
        if dec["action"] == "buy_equity":
            use("buy_equity", {"symbol": dec["symbol"], "quantity": dec["quantity"], "limit_price": dec["limit_price"], "rationale": dec["rationale"]})
        else:
            use("buy_option", {"occ_symbol": dec["symbol"], "underlying": und, "contract_type": dec.get("contract_type", "CALL"),
                               "strike": dec["strike"], "expiration": dec["expiration"], "contracts": dec["quantity"], "limit_price": dec["limit_price"], "rationale": dec["rationale"]})
        res(dec["action"], {"ok": True, "action": dec["action"], "symbol": dec["symbol"], "quantity": dec["quantity"], "limit_price": dec["limit_price"], "order_id": next_oid()})
        step()

    for dec in sells:
        und = dec["underlying"]
        px = PRICES.get(und, round(dec["limit_price"], 2))
        say("Closing " + und + ", priced to fill rather than sit on the book.")
        use("get_stock_quotes", {"symbols": und})
        res("get_stock_quotes", {und: {"last": px, "bid": round(px - 0.06, 2), "ask": round(px + 0.04, 2), "mark": px}})
        say(dec["rationale"])
        if dec["action"] == "sell_equity":
            use("sell_equity", {"symbol": dec["symbol"], "quantity": dec["quantity"], "limit_price": dec["limit_price"], "rationale": dec["rationale"]})
        else:
            use("sell_option", {"occ_symbol": dec["symbol"], "contracts": dec["quantity"], "limit_price": dec["limit_price"], "rationale": dec["rationale"]})
        res(dec["action"], {"ok": True, "action": dec["action"], "symbol": dec["symbol"], "quantity": dec["quantity"], "limit_price": dec["limit_price"], "order_id": next_oid()})
        step()

    if holds and not buys and not sells:
        say("Looked the book over and nothing cleared the bar, so I am holding and logging the decision.")
        for dec in holds:
            use("hold", {"symbol": dec["symbol"], "rationale": dec["rationale"]} if dec.get("symbol") else {"rationale": dec["rationale"]})
            res("hold", {"ok": True, "action": "hold", "symbol": dec.get("symbol", "")})
        step()

    use("write_summary", {"synopsis": "Logged the day's moves and sized each one against the settled cash on hand.", "action_items": "Reassess the open positions next session and act if a setup triggers."})
    res("write_summary", {"ok": True, "action": "write_summary", "stored": True})
    say("Session done for the day.")
    return ev


def main():
    days = trading_days(252)
    curve = gen_equity_curve(days)
    decision_days = days[-HISTORY_DAYS:]
    session_days = days[-SESSION_DAYS:]

    # Held book first, sized to the latest curve point's invested portion;
    # the decision log is then built to be coherent with it.
    last = curve[-1]
    positions = gen_positions(session_days[-1], last[1], last[2])

    by_date, round_trips = build_decisions(decision_days, session_days, positions)
    plans = {d.isoformat(): by_date.get(d.isoformat(), []) for d in session_days}
    stances = {d.isoformat(): random.choice(STANCES) for d in session_days}
    summaries = {d.isoformat(): random.choice(SUMMARIES) for d in session_days}
    action_items = {d.isoformat(): random.choice(ACTION_ITEMS) for d in session_days}

    # A transcript for EVERY day that has decisions: the rich recent sessions
    # get the full narrated run; every historical day (the opening/closing days
    # of the round trips) gets a lighter session that still narrates each move
    # with its stored rationale, so the "View the opening/closing session" link
    # on any holding or closed trade resolves and matches the card.
    session_iso = {d.isoformat() for d in session_days}
    latest_iso = session_days[-1].isoformat()
    transcripts = {}
    for ds in sorted(by_date):
        if ds in session_iso:
            # The most recent day (the one the dashboard links to) is the
            # kitchen-sink session that exercises every transcript render case.
            transcripts[ds] = transcript_for_day(plans[ds], stances[ds], summaries[ds], action_items[ds], positions,
                                                 kitchen_sink=(ds == latest_iso))
        else:
            transcripts[ds] = mini_transcript_for_day(by_date[ds])

    out = []
    p = out.append
    p("-- VibeTradez local development seed data")
    p("--")
    p("-- AUTO-GENERATED by generate-seed.py — do not hand-edit. Re-run that")
    p("-- script to refresh; the random seed is fixed so output is stable.")
    p("--")
    p("-- Realistic portfolio-manager data for the local dashboard: a year of")
    p("-- equity curve vs SPY, the last several sessions with stances + moves,")
    p("-- a full mock transcript per session day, and a few subscribers.")
    p("")
    p(SCHEMA)
    p("")
    p("TRUNCATE subscribers, transcripts, portfolio_decisions, portfolio_sessions, portfolio_equity_curve, portfolio_positions;")
    p("")

    p("-- ── Subscribers ──")
    p("INSERT INTO subscribers (email, name, active) VALUES")
    subs = [("alice@example.com", "Alice"), ("bob@example.com", "Bob"), ("carol@example.com", "Carol")]
    p(",\n".join(f"  ({sql_str(e)}, {sql_str(n)}, true)" for e, n in subs) + ";")
    p("")

    p("-- ── Equity curve (account vs SPY) ──")
    p("INSERT INTO portfolio_equity_curve (date, account_equity, settled_cash, unsettled_cash, high_water_mark, spy_close) VALUES")
    p(",\n".join(f"  ({sql_str(d)}, {ae}, {sc}, {uc}, {hwm}, {spy})" for (d, ae, sc, uc, hwm, spy) in curve) + ";")
    p("")

    # Held book, sized to the latest curve point's invested portion. One
    # snapshot row per curve day from each position's open through the
    # latest date (the EOD cron's real behavior), walking the mark from
    # cost basis to the current value so the trade-detail value chart and
    # the dashboard's unrealized line both have believable history.
    if positions:
        p("-- ── Held book (per-day snapshots from each open to the latest date) ──")
        p("INSERT INTO portfolio_positions (date, symbol, underlying, asset_type, contract_type, strike, expiration, quantity, market_value, cost_basis) VALUES")
        pvals = []
        curve_dates = [d for (d, _ae, _sc, _uc, _hwm, _spy) in curve]
        for pos in positions:
            strike = "NULL" if pos["strike"] is None else str(pos["strike"])
            expiration = "NULL" if pos["expiration"] is None else sql_str(pos["expiration"])
            opened = pos.get("opened", last[0])
            held_days = [d for d in curve_dates if opened <= d <= last[0]] or [last[0]]
            span = max(1, len(held_days) - 1)
            for i, d in enumerate(held_days):
                if i == 0:
                    mv = pos["cost_basis"]  # opens at cost
                elif i == span:
                    mv = pos["market_value"]  # ends at the current mark
                else:
                    drift = pos["cost_basis"] + (pos["market_value"] - pos["cost_basis"]) * (i / span)
                    mv = round(max(1.0, drift * (1 + random.uniform(-0.025, 0.025))), 2)
                pvals.append(
                    f"  ({sql_str(d)}, {sql_str(pos['symbol'])}, {sql_str(pos['underlying'])}, "
                    f"{sql_str(pos['asset_type'])}, {sql_str(pos['contract_type'])}, {strike}, {expiration}, "
                    f"{pos['quantity']}, {mv}, {pos['cost_basis']})"
                )
        # Closed round trips get the same per-day backfill across their
        # holding window, walking cost basis toward the exit proceeds, so
        # the closed-trade detail chart has a value series to draw. The
        # close date itself gets NO row: the real EOD cron snapshots after
        # the close, when a position sold that day is already gone, and a
        # row on the close date would resurrect the trade as a phantom
        # holding in the latest-snapshot fallback.
        for buy, sell in round_trips:
            t_days = [d for d in curve_dates if buy["date"] <= d < sell["date"]]
            if len(t_days) < 2:
                continue
            t_span = len(t_days)
            t_strike = "NULL" if buy["strike"] is None else str(buy["strike"])
            t_exp = "NULL" if buy["expiration"] is None else sql_str(buy["expiration"])
            for i, d in enumerate(t_days):
                if i == 0:
                    mv = buy["notional"]
                else:
                    drift = buy["notional"] + (sell["notional"] - buy["notional"]) * (i / t_span)
                    mv = round(max(1.0, drift * (1 + random.uniform(-0.03, 0.03))), 2)
                pvals.append(
                    f"  ({sql_str(d)}, {sql_str(buy['symbol'])}, {sql_str(buy['underlying'])}, "
                    f"{sql_str(buy['asset_type'])}, {sql_str(buy['contract_type'])}, {t_strike}, {t_exp}, "
                    f"{buy['quantity']}, {mv}, {buy['notional']})"
                )
        p(",\n".join(pvals) + ";")
        p("")

    p("-- ── Daily sessions (stance) ──")
    p("INSERT INTO portfolio_sessions (date, mode, stance, summary, action_items) VALUES")
    p(",\n".join(f"  ({sql_str(d.isoformat())}, 'live', {sql_str(stances[d.isoformat()])}, {sql_str(summaries[d.isoformat()])}, {sql_str(action_items[d.isoformat()])})" for d in session_days) + ";")
    p("")

    p("-- ── Decisions (opening buys for the held book + completed round trips) ──")
    p("INSERT INTO portfolio_decisions (uuid, date, source, action, asset_type, symbol, underlying, contract_type, strike, expiration, quantity, limit_price, notional, mode, schwab_order_id, status, rationale) VALUES")
    dvals = []
    oid = 1000
    for ds in sorted(by_date):
        for dec in by_date[ds]:
            strike = "NULL" if dec["strike"] is None else str(dec["strike"])
            expiration = "NULL" if dec["expiration"] is None else sql_str(dec["expiration"])
            order = sql_str(f"live-{oid}") if dec["action"] != "hold" else "''"
            oid += 1
            dvals.append(
                f"  ({sql_str(dec.setdefault('uuid', gen_uuid()))}, {sql_str(dec['date'])}, 'agent', {sql_str(dec['action'])}, {sql_str(dec['asset_type'])}, "
                f"{sql_str(dec['symbol'])}, {sql_str(dec['underlying'])}, {sql_str(dec['contract_type'])}, "
                f"{strike}, {expiration}, {dec['quantity']}, {dec['limit_price']}, {dec['notional']}, "
                f"'live', {order}, {sql_str(dec['status'])}, {sql_str(dec['rationale'])})"
            )
    p(",\n".join(dvals) + ";")
    p("")

    p("-- ── Session transcripts (one per decision day, viewable at /transcripts/<date>) ──")
    p("INSERT INTO transcripts (date, kind, model, events, usage, duration_ms) VALUES")

    def usage_and_duration(events):
        """Believable per-session token spend + wall-clock derived from the
        event stream. Input is the fresh (uncached) input billed each round;
        cache_read dominates because the conversation is replayed every round
        and served from the prompt cache. Scales with rounds + tool calls so
        a busy session reads heavier than a quiet one."""
        rounds = max((e["round"] for e in events), default=0) + 1
        tool_uses = sum(1 for e in events if e["type"] == "tool_use")
        ws = sum(1 for e in events if e["type"] == "tool_use" and e["tool_name"] == "web_search")
        wf = sum(1 for e in events if e["type"] == "tool_use" and e["tool_name"] == "web_fetch")
        usage = {
            "input_tokens": 540 + 110 * tool_uses,
            "output_tokens": 1400 + 880 * tool_uses + 1100 * rounds,
            "cache_read_tokens": 12000 + 8600 * rounds,
            "cache_creation_tokens": 2400 + 480 * rounds,
            "web_search_requests": ws,
            "web_fetch_requests": wf,
            "rounds": rounds,
        }
        duration_ms = 42000 + 17000 * rounds + 3500 * tool_uses
        return usage, duration_ms

    tvals = []
    for ds in sorted(transcripts):
        usage, duration_ms = usage_and_duration(transcripts[ds])
        tvals.append(
            f"  ({sql_str(ds)}, 'portfolio', 'claude-fable-5', {sql_str(json.dumps(transcripts[ds]))}::jsonb, "
            f"{sql_str(json.dumps(usage))}::jsonb, {duration_ms})"
        )
    p(",\n".join(tvals) + ";")
    p("")

    print("\n".join(out))


SCHEMA = """-- ── Schema (mirrors internal/store/store.go migrate()) ──────────────
CREATE TABLE IF NOT EXISTS subscribers (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    unsubscribed_at TIMESTAMPTZ,
    auth_user_id BIGINT
);
CREATE TABLE IF NOT EXISTS transcripts (
    id SERIAL PRIMARY KEY,
    date TEXT NOT NULL,
    kind TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    events JSONB NOT NULL,
    usage JSONB NOT NULL DEFAULT '{}',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (date, kind)
);
CREATE TABLE IF NOT EXISTS portfolio_decisions (
    id SERIAL PRIMARY KEY,
    uuid TEXT NOT NULL DEFAULT gen_random_uuid()::text,
    date TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'agent',
    action TEXT NOT NULL,
    asset_type TEXT NOT NULL DEFAULT '',
    symbol TEXT NOT NULL DEFAULT '',
    underlying TEXT NOT NULL DEFAULT '',
    contract_type TEXT NOT NULL DEFAULT '',
    strike DOUBLE PRECISION,
    expiration TEXT,
    quantity DOUBLE PRECISION NOT NULL DEFAULT 0,
    limit_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    notional DOUBLE PRECISION NOT NULL DEFAULT 0,
    mode TEXT NOT NULL DEFAULT 'live',
    schwab_order_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    fill_price DOUBLE PRECISION,
    execution_id INTEGER NOT NULL DEFAULT 0,
    rationale TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS portfolio_sessions (
    date TEXT PRIMARY KEY,
    mode TEXT NOT NULL DEFAULT 'live',
    stance TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    action_items TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS portfolio_equity_curve (
    date TEXT PRIMARY KEY,
    account_equity DOUBLE PRECISION NOT NULL DEFAULT 0,
    settled_cash DOUBLE PRECISION NOT NULL DEFAULT 0,
    unsettled_cash DOUBLE PRECISION NOT NULL DEFAULT 0,
    high_water_mark DOUBLE PRECISION NOT NULL DEFAULT 0,
    spy_close DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS portfolio_positions (
    id SERIAL PRIMARY KEY,
    date TEXT NOT NULL,
    symbol TEXT NOT NULL,
    underlying TEXT NOT NULL DEFAULT '',
    asset_type TEXT NOT NULL DEFAULT '',
    contract_type TEXT NOT NULL DEFAULT '',
    strike DOUBLE PRECISION,
    expiration TEXT,
    quantity DOUBLE PRECISION NOT NULL DEFAULT 0,
    market_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    cost_basis DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);"""


if __name__ == "__main__":
    main()
