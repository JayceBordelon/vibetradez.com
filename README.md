# vibetradez.com

An AI runs a single real brokerage account. Each trading day Claude (we call her Claudia) reads the book, the tape, and the news, then decides from scratch what to do with the money: buy equity, buy options, trim, sell, or hold cash. Positions are held across days, every move is sized inside hard caps enforced in code, and the benchmark is buy-and-hold SPY. Subscribers watch and get one recap email a day.

> **This is live.** The account trades real money against the Schwab Trader API whenever `TRADING_ENABLED=true`. There is no paper mode. Every number on the site comes from one real account.

## The daily loop

One session per trading day, around midday, when spreads have settled and the morning's information is already on the tape.

```
        ┌── observes ──────────────────────────────────────┐
        │   the live book · quotes · option chains ·       │
        │   price history · news · cap headroom ·          │
        │   its own realized track record vs SPY           │
        ▼                                                  │
   ┌─────────┐   buy · sell · trim · hold   ┌──────────┐   │
   │ Claudia │ ───────────────────────────▶ │  Schwab  │ ──┘
   └─────────┘    every order passes the    └──────────┘
        │         cap sheet (in code) first
        │
        └── writes a synopsis + action items, read back
            as the starting point of the next session
```

Three crons drive the day (America/New_York, weekends and holidays skipped):

```
30 12 * * MON-FRI    the session   reads everything, then trades
*/15 .. * * MON-FRI  the sweep     reconciles fills against the broker
00 16 * * MON-FRI    the close     snapshots equity vs SPY, sends the recap
```

Orders are fire-and-forget LIMITs that fill over the afternoon. The full tool-by-tool reasoning of every session is public at `/transcripts/<date>`.

## The guardrails

The entire risk policy is two sleeve caps, enforced at the tool layer where the model cannot talk its way past them:

| Cap | Value |
|---|---|
| Options sleeve | premium at risk ≤ 50% of live equity |
| Equity sleeve | stock value ≤ 50% of live equity |

Buys also spend settled cash only (broker T+1 compliance, not a risk preference), and a flat absolute order ceiling at the broker entry backstops code bugs, not trades. There is no per-name cap, no stop loss, no drawdown breaker. The model concentrates and sizes however it judges best, and it can absolutely lose the money. That is the show.

## Architecture

```
   visitors ──▶ traefik ─┬─ /api /auth /health ──▶ trading-server (Go)
                         └─ everything else ─────▶ trading-frontend (Next.js)

   trading-server owns every outbound dependency:
     postgres    the book, sessions, transcripts, subscribers
     schwab      market data + the Trader API (orders, positions, streams)
     anthropic   Claudia, the portfolio agent
     resend      the daily recap email
```

Sign-in is in-process Google OAuth on the trading-server. Live quotes stream from Schwab's WebSocket and fan out to browsers as SSE. The frontend talks to nothing outside the droplet.

## Repo layout

```
server/    Go API: portfolio agent + tools + cap sheet (internal/portfolio),
           Schwab client, store, crons, in-process OAuth, email
client/    Next.js 16 frontend: dashboard, holdings, closed trades,
           session transcripts, in a CRT terminal aesthetic
local/     Self-contained Docker stack with seeded Postgres for offline dev
```

| Component | Choice |
|---|---|
| Server | Go 1.25, PostgreSQL, Anthropic Claude, Schwab Market Data + Trader API, Resend |
| Client | Next.js 16, React 19, Tailwind v4, shadcn/ui, Recharts v3 |
| Infra | Docker Compose + Traefik on a Digital Ocean droplet |

## Run it locally

```bash
cd local
docker compose -f docker-compose.local.yml up --build
# http://localhost:3001
```

The local stack boots Postgres, the server, and the frontend with seeded data and stub keys, so nothing real is called and no cron ever fires. For a server-only loop: `cd server && go test ./... && go build ./...`. For client-only dev: `cd client && npm run dev`.

Schema migrations run inline at server boot (`internal/store/store.go`). Required env vars are listed in `server/.env.example`, and a missing one fails the boot loudly.

## CI and deploys

Every PR runs Biome + `next build` on the client and gofmt / vet / build / `go test -race` on the server. Merges to `main` deploy via the operator's pipeline. Every PR that touches the client gets a model-driven UI audit (`scripts/ui-audit/`, convention in `docs/ui-audits/README.md`).

## Related

[jaycebordelon.com](https://github.com/JayceBordelon/jaycebordelon.com) is the builder's portfolio and blog, a separate stack with no shared infra.

## License

MIT. See [LICENSE](LICENSE). Use it, fork it, run your own. Contributions are welcome by PR, and opening one means you agree to license your contribution under the same terms.
