# VibeTradez — Local Development Runtime

A self-contained Docker stack for testing VibeTradez locally with realistic seeded data. No production credentials, no external API calls, no Traefik.

## What's included

- **Postgres 16** — auto-seeded with ~10 trading days of trades + EOD summaries (the most recent day is left in "morning picks" mode without summaries so you can see both UI states)
- **Go API server** — runs against the local Postgres, with stub env vars for OpenAI/Resend/Schwab so it never makes real API calls
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
-- Count of trades and summaries per day
SELECT date, COUNT(*) FROM trades GROUP BY date ORDER BY date DESC;
SELECT date, COUNT(*) FROM summaries GROUP BY date ORDER BY date DESC;

-- See the latest day's picks (should have NO summary entries — morning mode)
SELECT symbol, contract_type, rank, estimated_price FROM trades
WHERE date = (SELECT MAX(date) FROM trades) ORDER BY rank;

-- See the previous day with EOD results
SELECT t.symbol, t.rank, s.entry_price, s.closing_price,
       ROUND(((s.closing_price - s.entry_price) * 100)::numeric, 0) AS pnl_per_contract
FROM trades t
JOIN summaries s ON t.date = s.date AND t.symbol = s.symbol
WHERE t.date = (SELECT MAX(date) FROM summaries)
ORDER BY t.rank;
```

## What you can test

- **Dashboard at `/`** — most recent date is in "morning picks" mode (no summaries). The previous day will switch to "EOD results" mode with stats grid, P&L chart, and trade table.
- **Date navigation** — prev/next arrows to walk through 10 days of seeded history
- **Top N filter** — toggle between Top 1, 3, 5, and 10 picks; localStorage persistence works
- **Historical analytics at `/history`** — full equity curve, daily P&L bars, exposure vs returns charts, capital efficiency panel, daily breakdown with expandable rows
- **Mode toggle** — Week / Month / Year / All time on the history page
- **Subscribe modal** — opens via the top bar button. Submitting writes to the local subscribers table.
- **Terms & FAQ** — `/terms` and `/faq` pages
- **API protection** — `curl http://localhost:8080/api/trades/today` returns 403 (missing `X-VT-Source`); the frontend includes the header automatically

## Disabled in local mode

- **Cron jobs** — pushed to Sunday so they never fire
- **OpenAI / Schwab / Resend** — stub keys; the server starts but never makes real calls
- **Live quotes (`/api/quotes/live`)** — returns `connected: false` since Schwab is unauthorized; the frontend gracefully degrades to "market closed" freshness
- **Stock chart (`/api/chart/{symbol}`)** — returns 503; the chart panel shows "Chart unavailable"

## Files in this directory

| File | Purpose |
|------|---------|
| `docker-compose.local.yml` | Local stack definition |
| `seed.sql` | Schema + ~100 seeded trades + ~90 EOD summaries + 3 subscribers |
| `README.md` | This file |
