# VibeTradez — Local Development Runtime

A self-contained Docker stack for testing VibeTradez locally with realistic seeded data. No production credentials, no external API calls, no Traefik.

## What's included

- **Postgres 16** — auto-seeded for the portfolio manager: ~1 year of daily equity-curve points (account vs SPY), the last few daily sessions with stance notes, the recent moves, a few subscribers, and one session transcript
- **Go API server** — runs against the local Postgres with `TRADING_ENABLED=true` but **no broker wired**: the stub Schwab/Anthropic/Resend keys mean no real Schwab client is built, so the manager never places orders or makes real API calls. There is no paper mode — the dashboard is driven entirely by the seeded tables
- **Next.js frontend** — proxies `/api`, `/auth`, `/admin`, and `/health` to the Go server via `next.config.ts` rewrites

## Prerequisites

- Docker Desktop (or Docker Engine + Compose v2)

## Usage

From the repo root:

```bash
cd vibetradez.com/local
docker compose -f docker-compose.local.yml up --build
```

First boot takes ~1–2 minutes (Postgres init + Go build + Next.js build). Subsequent boots are much faster thanks to Docker's layer cache.

Then open:

- **Frontend**: http://localhost:3001
- **API health**: http://localhost:8080/health
- **Postgres**: `localhost:5433` (user `vibetradez`, password `vibetradez`, db `vibetradez`)

## Tear down

To stop the stack but keep the database volume:

```bash
docker compose -f docker-compose.local.yml down
```

To stop and **wipe the database** (fresh seed on next boot):

```bash
docker compose -f docker-compose.local.yml down -v
```

## Inspecting the seeded data

Connect to the local Postgres directly:

```bash
psql postgresql://vibetradez:vibetradez@localhost:5433/vibetradez
```

Or via Docker:

```bash
docker exec -it vt-local-postgres psql -U vibetradez
```

Useful queries:

```sql
-- The equity curve (account vs SPY), most recent days
SELECT date, account_equity, spy_close FROM portfolio_equity_curve
ORDER BY date DESC LIMIT 10;

-- The recent daily stances
SELECT date, stance FROM portfolio_sessions ORDER BY date DESC;

-- The recent moves
SELECT date, action, symbol, notional, rationale
FROM portfolio_decisions ORDER BY created_at DESC, id DESC;
```

## What you can test

- **Dashboard at `/dashboard`** — the portfolio view: equity/cash summary, the equity curve vs SPY, the seeded holdings table, today's moves with rationale, and the stance. Refresh the seed (below) if "today's moves" is empty because the seed date has gone stale.
- **Session transcript** — the "view the full session reasoning" link under today's moves opens `/transcript/<date>/portfolio`.
- **Subscribe modal** — opens via the top bar button. Submitting writes to the local subscribers table.
- **Terms & FAQ** — `/terms` and `/faq` pages.
- **API protection** — `curl http://localhost:8080/api/portfolio` returns 403 (missing `X-VT-Source`); the frontend includes the header automatically.

## Refreshing the seed

The seed dates are relative to the day `generate-seed.py` was run, so "today's moves" and the end of the curve drift over time. Regenerate and reboot with a clean volume:

```bash
python3 generate-seed.py > seed.sql
docker compose -f docker-compose.local.yml down -v
docker compose -f docker-compose.local.yml up --build
```

## Disabled in local mode

- **Cron jobs** — the portfolio session/risk/EOD crons are pushed to Sunday so they never fire (no real broker or model to call).
- **Anthropic / Schwab / Resend** — stub keys; the server starts but never makes real calls. Live trading stays off because no Schwab broker is wired (not a paper-mode flag — there is no paper mode).

## Files in this directory

| File | Purpose |
|------|---------|
| `docker-compose.local.yml` | Local stack definition |
| `generate-seed.py` | Regenerates `seed.sql` (portfolio curve + sessions + moves + subscribers + transcript) |
| `seed.sql` | Generated schema + seed data |
| `README.md` | This file |
