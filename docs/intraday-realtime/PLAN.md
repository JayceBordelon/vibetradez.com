# Intraday Sessions + Realtime News/Hype Tools

**Branch:** `feat/intraday-realtime-tools` (worktree off `origin/main`).
**Two changes:** (1) run the portfolio agent 3x/day instead of once; (2) give the agent realtime news + hype/sentiment + ticker-discovery tools.

> **Status: SHIPPED on this branch** — `go build`, `go test -race ./...`, and `golangci-lint` all green.
> - **Sessions:** open `9:45` / midday `12:30` / pre-close `15:30` ET, config-overridable via `CRON_SCHEDULE_PORTFOLIO_{OPEN,MIDDAY,CLOSE}` (legacy `CRON_SCHEDULE_PORTFOLIO` still feeds midday). Each session is slot-aware (open = react to overnight tape, midday = primary read, pre-close = position for overnight). No schema migration was needed: the date-keyed session row chains the three sessions automatically, and the three transcripts merge into one daily `portfolio` transcript via `transcript.Merge`.
> - **Tools (free, no account):** `get_ticker_news` (Yahoo Finance + Google News RSS), `get_trending_tickers` and `get_social_sentiment` (StockTwits, aggregate-only to stay public-transcript safe), in `internal/marketnews/`. **Alpaca was dropped** (no account wanted) along with all paid tiers. SEC EDGAR 8-K catalysts are a possible fast-follow (deferred — needs a ticker→CIK map).
> - Prod `.env` was updated with the three cron vars (no container restart; takes effect on next deploy).
>
> The sections below are the original research + design notes; where they reference Alpaca or paid tiers, the shipped build uses the free no-account stack above instead.

---

## Change 1 — Three intraday sessions

### Recommended schedule (America/New_York)
| Slot | Time | Why |
|---|---|---|
| **open** | **9:45 ET** | 15 min after the 9:30 open — lets the opening-auction spread chaos settle while still catching overnight gaps and the morning move. (9:30 sharp is dangerous: wide spreads, volatile.) |
| **midday** | **12:30 ET** | Keep the existing, proven slot. Spreads settled, morning news on the tape. |
| **close** | **15:30 ET** | 30-min buffer before the 16:00 close so fire-and-forget LIMITs have time to fill before the EOD snapshot/recap. Fresh end-of-day read for overnight positioning. |

All three are already config-driven (`CronSchedulePortfolio` etc. in `config.go`) — make them three configurable schedules with these defaults; overridable by env.

**Tradeoff knob:** earlier pre-close (15:00) = more fill time but less-final info; later (15:45) = freshest read but tighter fills. 15:30 is the balance.

### Slot-aware mandate (prompt change)
Each session should know its role (pass the slot into the agent + prompt):
- **open** — react to overnight news/gaps; trim risk or seize clear momentum; don't over-trade into wide spreads.
- **midday** — the main thesis session (today's current behavior).
- **close** — decide what to hold overnight vs. close; manage end-of-day; know that LIMITs placed now may not fill before the close.

The prompt currently says *"You run once per trading day"* (`prompt.md:7`) — update it for the intraday cadence and explain that each session sees the prior session(s) from **today**.

### Schema changes (so 3 sessions don't overwrite each other)
Add a `slot` dimension (`open` | `midday` | `close`):
- **`portfolio_sessions`** (`store.go:191`): PK `(date)` → PK `(date, slot)`. Otherwise only the last session's stance/summary/action-items survive.
- **`transcripts`** (`store.go:130`): write distinct `kind` per slot (`portfolio-open` / `portfolio-midday` / `portfolio-close`) so the `UNIQUE(date, kind)` constraint stops overwriting. Dashboard route resolves "latest kind for date."
- **`portfolio_decisions`** (`store.go:159`): add a `slot` column so the UI/email can group "session 1 vs 2 vs 3" moves. (Multiple-per-date already works.)
- **`portfolio_equity_curve`**: leave as one row/date — the 16:00 EOD snapshot is separate and unaffected.

### Handoff / continuity
`LatestPortfolioSession()` (`portfolio.go:545`) + `PriorSession()` (`tools.go:530`) currently read the most-recent session. With slots this naturally chains: midday reads open's action items, close reads midday's. **Desirable** — each session builds on the last. Just confirm the prompt frames it that way.

### Daily recap email
`sendDailyRecapEmail` (`portfolio.go:84`) reads one session/day → would show only the close session. Decide: aggregate all 3 slots' synopses into the recap (recommended — show the day's arc), or keep just the close summary + all decisions (which it already pulls).

### Half-day handling (early 13:00 close)
The market-open gate (`calendar.go`) is half-day aware: a static 15:30 pre-close cron is **auto-skipped on half-days** (market already closed). Options:
- **v1 (simple):** accept no pre-close session on half-days (low-volume days anyway). Open + midday still run.
- **better:** make the pre-close gate dynamic — derive target from `CloseMinute(date)` and run ~30 min before the *actual* close (so half-days get a ~12:30 pre-close). Small enhancement.
- **Bug to fix regardless:** `HalfDays` map (`calendar.go:62`) is missing July 3 when July 4 is a weekday, and needs annual upkeep.

### Cost note
3 sessions = ~3x the Anthropic agent spend/day + 3x the news-API pulls. Modest, but real.

---

## Change 2 — Realtime news / hype / discovery tools

The agent already has `web_search` + `web_fetch`. We're adding **structured, low-latency, professional feeds** so it doesn't have to free-search for everything.

### Recommended data stack
- **News (linchpin):** **Alpaca News API** — real-time, Benzinga-sourced professional newswire, **free** with any Alpaca account (even unfunded paper, no trading). REST + WebSocket, official Go SDK. This alone is a big upgrade.
- **Hype / retail sentiment:** **Quiver Quantitative** — WSB/Reddit mentions + congressional + insider. Hobbyist **$30/mo**, Trader **$75/mo**. Cleanest single REST API for retail-hype signal (refreshes daily — fine for 3x/day).
- **Ticker discovery ("what's hot that I'm not watching"):** Alpaca most-actives (free) or Massive/Polygon snapshot gainers/losers ($29/mo).
- **Optional smart-money hype:** Unusual Whales options flow (~$48/mo). **Optional live-X sentiment:** Grok API (realistic proxy; raw X/Reddit APIs are economically out for a solo operator).

### Budget tiers
- **Tier A (~$30/mo) — ship first:** Alpaca News (free) + Alpaca movers (free) + Quiver Hobbyist ($30) + Finnhub free. Genuinely professional floor.
- **Free-only ($0):** Alpaca News + Alpaca movers; add paid hype later.
- **Tier B (~$300/mo):** + Massive $29 + Benzinga Analyst Insights $99 + Quiver Trader $75 + Unusual Whales $48 + Grok usage.

### New agent tools (follow existing pattern: `ToolDefinitions()` → `Dispatch()` → handler → `PortfolioReader` method → `wire.go` impl)
- `get_ticker_news(symbols)` → Alpaca News, recent real-time headlines per ticker.
- `get_market_movers()` → top gainers/losers/most-active for discovery.
- `get_social_hype(symbols)` and/or `get_trending_hype()` → Quiver WSB mentions/sentiment.
- *(optional)* `get_options_flow(symbol)` → Unusual Whales.

### New internal packages
`internal/alpacanews/`, `internal/quiver/` (and `internal/unusualwhales/` if Tier B), each a thin REST client. Wire into the production reader in `portfoliowire/wire.go`; extend the `PortfolioReader` interface. Keys via config/env (Alpaca News reuses Alpaca keys; others their own).

---

## Build sequence
1. **Schema + slot plumbing** (no external deps, reversible): add `slot` to sessions/decisions, per-slot transcript kinds, migrations. Keep single-session behavior working behind the slot default.
2. **Three crons + slot-aware session entry**: 3 configurable schedules, pass slot into `runPortfolioSession`/agent.
3. **Slot-aware prompt**: update mandate per slot, intraday cadence framing.
4. **Half-day pre-close** dynamic gate (or accept v1 skip) + fix `HalfDays` map.
5. **Alpaca News tool** (free — wire immediately): `internal/alpacanews/` + `get_ticker_news` + `get_market_movers`.
6. **Quiver hype tool** (if Tier A+): `internal/quiver/` + `get_social_hype`.
7. **Recap email**: aggregate the 3 slots.
8. Lint/build/test per CLAUDE.md; UI audit if dashboard changes (decisions now grouped by slot).

---

## Decisions needed
1. **Session times** — confirm 9:45 / 12:30 / 15:30 ET (or adjust).
2. **News/hype tier** — Tier A (~$30), Free-only ($0), or Tier B (~$300). Drives which clients I build + which API keys you sign up for.
3. **Half-day pre-close** — simple skip vs. dynamic timing (can default simple, add later).

## Sources
Alpaca News (free Benzinga): docs.alpaca.markets/us/docs/streaming-real-time-news · alpaca.markets/data · Quiver: api.quiverquant.com/pricing · Massive/Polygon: massive.com/docs · Unusual Whales: unusualwhales.com/developers · cron: robfig/cron/v3
