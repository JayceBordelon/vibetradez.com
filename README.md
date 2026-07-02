# vibetradez.com

An AI runs a single real brokerage account. Each trading day Claude (we call her Claudia) reads the book, the tape, and the news, then decides from scratch what to do with the money: buy options, trim, sell, or hold cash. It trades options only — calls and puts — and liquidates any leftover stock to fund them. Positions are held across days, sizing is the model's call within code-enforced caps (no single contract over ~10% of equity, no single name over ~25%), and the benchmark is buy-and-hold SPY. Subscribers watch and get a recap email only when a trade closes — a session that just buys or holds emails nobody.

> **This is live.** The account trades real money against the Schwab Trader API whenever `TRADING_ENABLED=true`. There is no paper mode. Every number on the site comes from one real account.

## The daily loop

Three sessions per trading day — at the open (~9:45 ET, once the opening spreads settle), midday (~12:30 ET, the main read), and before the close (~15:30 ET) — each a fresh automated pass with the full tool surface. The open reacts to the overnight tape, midday sets the core positioning, and the pre-close positions the book for overnight. Each session reads back the synopsis and action items the previous one left, so the three chain through the day.

```
        ┌── observes ──────────────────────────────────────┐
        │   the live book · quotes · option chains ·       │
        │   price history · news · settled cash ·          │
        │   its own realized track record vs SPY           │
        ▼                                                  │
   ┌─────────┐   buy · sell · trim · hold   ┌──────────┐   │
   │ Claudia │ ───────────────────────────▶ │  Schwab  │ ──┘
   └─────────┘    every buy passes the      └──────────┘
        │         settled-cash rule (in code) first
        │
        └── writes a synopsis + action items, read back
            as the starting point of the next session
```

Five crons drive the day (America/New_York, weekends and holidays skipped):

```
45 9  * * MON-FRI    session: open       reads everything, then trades (post-auction)
30 12 * * MON-FRI    session: midday     the primary read of the book
30 15 * * MON-FRI    session: pre-close  positions for overnight (skipped on half-days)
*/15 .. * * MON-FRI  sweep               reconciles fills against the broker
00 16 * * MON-FRI    EOD                 snapshots equity vs SPY
```

Orders are fire-and-forget LIMITs that fill asynchronously. Alongside the broker feed, the model pulls free, no-account real-time signal to stay current on hot names — headlines (Yahoo Finance + Google News RSS) and retail hype (StockTwits trending + bull/bear sentiment) — on top of its built-in web search. The full tool-by-tool reasoning of all three daily sessions is merged into one public transcript per day at `/transcripts/<date>`. The recap email is model-authored and goes out only when the session closed a position (a sell, including legacy-stock liquidation) — buy-only and hold-only sessions send nothing.

## The guardrails

The model trades options only, and the code keeps it from putting the whole account on one bet. Equity buys are disabled (any stock held from before is liquidated to cash); beyond that the model sizes and picks however it judges best.

The rules enforced in code, none of them a market view:

| Rule | Why it exists |
|---|---|
| Options only — equity buys disabled | The mandate; legacy stock is liquidated via `sell_equity` |
| No single option position over ~10% of equity | Per-position concentration cap |
| No single underlying over ~25% of equity | Per-name concentration cap |
| Spends settled cash only | Broker T+1 compliance for a cash account |
| Flat absolute per-order ceiling at the broker entry | A fat-finger backstop against code bugs |

The model concentrates and sizes however it judges best within those caps, and it can absolutely lose the money. That is the show.

## Architecture

```
   visitors ──▶ traefik ─┬─ /api /auth /health ──▶ trading-server (Go)
                         └─ everything else ─────▶ trading-frontend (Next.js)

   trading-server owns every outbound dependency:
     postgres    the book, sessions, transcripts, subscribers
     schwab      market data + the Trader API (orders, positions, streams)
     anthropic   Claudia, the portfolio agent
     resend      the model-authored recap email
```

Sign-in is in-process Google OAuth on the trading-server. Live quotes stream from Schwab's WebSocket and fan out to browsers as SSE. The frontend talks to nothing outside the droplet.

## Repo layout

```
server/    Go API: portfolio agent + tools + gates (internal/portfolio),
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
