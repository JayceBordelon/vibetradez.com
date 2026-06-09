# CLAUDE.md

Operator notes for this repo. Read before touching the trading service.

## Architecture: autonomous portfolio manager

VibeTradez is an autonomous **portfolio manager** that runs a single personal brokerage account: one daily agent decides from scratch (buy equity, buy options, trim, sell, or hold cash), sizes within hard caps, and holds positions across days. Mandate: grow the account, benchmarked to SPY. (The original three-pick options bot has been removed.)

- **It's one account, not a multi-user product.** UI and copy treat it as a single book; subscribers watch and get the daily recap email, they don't trade.
- **Live-only, runs whenever enabled.** The manager runs whenever `TRADING_ENABLED=true` and trades live with real money against the Schwab Trader API. There is no paper mode (the paper trader was removed), so enabling it is the deliberate action. Read the daily transcript to see exactly what it did.
- **Where it lives:** `internal/portfolio` (cap sheet = the security boundary in `caps.go`, tool layer, agent), `internal/portfoliowire` (adapters to Schwab + exec + store), `internal/exec` (broker: equity/options orders, positions, the portfolio entry points), the `portfolio_*` store tables, the three crons in `cmd/scanner` (daily session ~9:45 ET, intraday risk sweep, 16:00 EOD equity-curve snapshot), the `/api/portfolio*` endpoints, and `client/components/portfolio` (the dashboard).
- **The cap sheet is two sleeves**: option premium <= 50% of live equity and stock value <= 50% of live equity. That's the whole risk policy: no per-name cap, no per-order cap, no drawdown breaker, no liquidity floor, no session pacing. The settled-cash rule (broker T+1 compliance) also gates buys. Enforced at the **tool layer only** (`caps.go` `Check*`); the broker entry point re-checks only a flat $25k absolute fat-finger ceiling (`exec.MaxPortfolioOrderCostCeiling`), so the tool layer is the sole enforcement point. Don't add caps in the prompt, the policy lives in code.
- The intraday cron's remaining duty is reconciling decision-log order statuses against the broker (fills show on the dashboard within ~15 min).

## Development rules

### Lint + build + test before pushing

Run on every change to the server:

```bash
cd server
gofmt -w . && go vet ./...
go build ./...
go test -race -timeout 120s ./...
golangci-lint run ./...
```

`golangci-lint` is what production CI runs; `go vet` alone misses unused-function and similar dead-code findings. Don't rely on `go vet` as the only gate. If `golangci-lint` isn't installed locally: `brew install golangci-lint`.

For the client:

```bash
cd client
npx biome check --write .   # MUST use --write
npx next build
```

`biome check` without `--write` is not enough — it reports format errors but doesn't apply them, so the working tree still has CI-failing files when you commit. CI runs Biome with format-as-error semantics.

### Always use feature branches

Never push directly to `main`. Open a PR, let CI run, merge.

### Build + e2e locally before every push

The trading server runs against a live broker. A broken cron in production can mean missed fills, mis-sized orders, or worse. Before every push:

1. Local Docker stack boots cleanly (`cd local && docker compose -f docker-compose.local.yml up --build`)
2. Server `go test -race` green
3. Client `next build` green
4. Manual smoke: hit the local `/health` and the local portfolio dashboard

## UX/UI audit on every client PR

Any PR that touches `client/` (anything a user can see) gets a
**model-driven UI audit**: boot the seeded local stack, look at the
rendered app (full-page captures via `scripts/ui-audit/run.sh`, or drive
Playwright directly for behavior like animations and live updates), fix
what you find in the same PR, and summarize findings + fixes in the PR
description. Screenshots are throwaway analysis input in the gitignored
`scripts/ui-audit/out/`: never commit them. The PR description is the
audit record.

```bash
# fresh seeded local stack
cd local
python3 generate-seed.py > seed.sql
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml down -v
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml up --build -d
# capture analysis input (the 4 theme x viewport combos run concurrently)
cd ../scripts/ui-audit && npm install && ./run.sh
```

- Cover both viewports and both themes on every page the change touches,
  plus a regression glance at the rest; for behavior stills can't show,
  write a short throwaway Playwright probe against the running stack.
- The runner resolves dynamic routes (latest transcript date/kind, one
  holding and one closed trade) from the live stack, forces themes via
  `localStorage["theme"]`, and auto-scrolls each page so scroll-reveal
  sections are not caught at `opacity:0`. New page? Add it to
  `scripts/ui-audit/routes.mjs` and the route list in `run.sh`.
- The [PR template](.github/pull_request_template.md) carries the
  checklist; the convention lives in
  [`docs/ui-audits/README.md`](docs/ui-audits/README.md). The dated
  folders under `docs/ui-audits/` are frozen history from the old
  commit-the-screenshots convention.

## Schema migrations live inline

`internal/store/store.go`'s `migrate()` function runs on server boot with `CREATE TABLE IF NOT EXISTS` and `ALTER TABLE ... IF NOT EXISTS` for additive columns. Schema changes go there; no separate migration directory.

## Model version refresh policy

The portfolio agent's model is configured via the `ANTHROPIC_MODEL` env var with the default in `server/internal/config/config.go` as `DefaultAnthropicModel`.

Any time you touch the portfolio agent or this default, fetch the official Anthropic Go SDK documentation and refresh the default to the current latest production model. Anthropic publishes new model versions regularly, and a stale default degrades decision quality silently.

- Anthropic Go SDK: <https://platform.claude.com/docs/en/api/sdks/go>

When updating, also bump the `ANTHROPIC_MODEL` default baked into `local/docker-compose.local.yml` so the local dev stack matches.

## Recharts v3

The client is pinned at Recharts ^3.8.0 wrapped by the shadcn `ChartContainer` primitive at `components/ui/chart.tsx`. Recharts 3 was a hard break from 2 — read the migration guide before touching any chart code.

- v2 → v3 migration: <https://github.com/recharts/recharts/wiki/3.0-migration-guide>
- Release notes: <https://github.com/recharts/recharts/releases>

Breaking changes that bite us in this codebase:

- `CategoricalChartState` is gone. Anything that used to read internal chart state via `Customized` or props now uses hooks (`useActiveTooltipLabel`, etc.).
- Many "internal" cloned props are gone: `Scatter.points`, `Area.points`, `Legend.payload`, `activeIndex`. If you see code reading any of these, it's broken on v3.
- `<Customized />` no longer receives extra props.
- `ref.current.current` on `ResponsiveContainer` is gone.
- Multiple `YAxis` instances render in alphabetical order of `yAxisId`, not render order.
- `CartesianGrid` requires explicit `xAxisId` / `yAxisId` to match the axes it pairs with.
- SVG z-order is the JSX render order — to put a series on top, render it last.
- `Area`'s `connectNulls=true` now treats null datapoints as zero instead of skipping them.
- `Pie.blendStroke` is removed; use `stroke="none"`.
- `<Cell>` is deprecated as of v3.7 and will be removed in v4. Per-bar colors come from a `fill` field on each datum, which Recharts reads directly on the `<Bar>`. Don't reach for `Cell` in new code.
- Tooltip custom-content prop type is now `TooltipContentProps`, not `TooltipProps`.

**Project-specific rules:**

- Always render charts through `ChartContainer` from `@/components/ui/chart` — it owns the `ResponsiveContainer`, the `--color-*` CSS variable injection, and the tooltip context.
- Never call `.map()` directly on a `data` prop without a fallback. Any boundary that produces JSON arrays must initialize them as empty slices server-side (Go nil slice → JSON `null`), and any client function that consumes them must `?? []` them defensively.
- When passing data into Recharts components, the data prop must be an array, not null/undefined. Guard with `data && data.length > 0 && <BarChart data={data} ...>`.

## API protection

All `/api/*` routes on the trading server require the `X-VT-Source` header. Without it, requests return 403. The Next.js frontend includes this header on every fetch via its server-side API proxy. CI tests use a helper that sets the header; an external caller without the header is locked out.

## Latest documentation, not recalled syntax

When working with Next.js, Tailwind CSS, shadcn/ui, Recharts, Anthropic SDK, Schwab API, or any external library: fetch and read the current documentation before writing code. Recalled syntax may be outdated. Anthropic and Schwab in particular publish updates frequently. Incorrect API assumptions cause more rework than the time saved by skipping docs.

## Auth is in-process

Google OAuth is handled by the trading-server binary itself. The package lives at `internal/auth/` and serves `/auth/google/start` + `/auth/google/callback`. Sessions are validated against a dedicated Postgres pool (`AUTH_DATABASE_URL`) on every `/api/*` request via the `AttachUser` middleware.

The Google Cloud Console redirect URI is `https://vibetradez.com/auth/google/callback`. Changing the hostname or the callback path requires updating the Console allowlist or sign-in 400s with `redirect_uri_mismatch`.

Required env vars at boot: `AUTH_DATABASE_URL`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`. Optional `GOOGLE_CALLBACK_URL` overrides the default.

## Related repos

- [jaycebordelon.com](https://github.com/JayceBordelon/jaycebordelon.com) — personal portfolio and blog. Self-contained stack, planned to live on its own droplet.

## Common operations

### Re-authorize Schwab OAuth

Visit `https://vibetradez.com/auth/schwab` in a browser. Tokens are stored in the `oauth_tokens` table and auto-refresh.

### Check server health

```bash
curl https://vibetradez.com/health | jq
```

Returns per-service status for `database`, `anthropic`, `schwab_market_data`, `market_signals`, `api`, `morning_picks` with latencies. The Anthropic check goes through the official SDK and warns (instead of fails) when a stub local key is detected.

### Docker commands on production

```bash
ssh jayce@<server>
cd <project-path>
docker compose logs trading-server --tail 50    # Go server logs
docker compose logs trading-frontend --tail 50  # Next.js logs
docker compose restart trading-server           # Restart Go server
docker compose up -d --force-recreate trading-server  # Full recreate (env reload)
```

## Where the load-bearing logic lives

| Concern | Path |
|---|---|
| Cap sheet (the security boundary) + pure cap checks | `server/internal/portfolio/caps.go` |
| Portfolio tool layer (buy/sell equity + option, hold, reads) | `server/internal/portfolio/tools.go` |
| Portfolio agent loop + final-JSON stance parse | `server/internal/portfolio/agent.go` |
| Mandate prompt | `server/internal/portfolio/prompt.go` |
| Adapters to Schwab + exec + store | `server/internal/portfoliowire/wire.go` |
| Broker layer (equity + options orders, positions, account, portfolio entry points) | `server/internal/exec/` |
| Schwab market cap (instruments fundamentals) | `server/internal/schwab/instruments.go` |
| Portfolio crons (daily session, risk sweep, EOD snapshot) + email send | `server/cmd/scanner/portfolio.go` + `main.go` |
| Portfolio store tables + methods | `server/internal/store/portfolio.go` |
| Portfolio API endpoints | `server/internal/server/portfolio.go` |
| Daily recap email template | `server/internal/templates/portfolio_update.html` |
| Market open / holiday gating | `server/internal/calendar/calendar.go` |
| In-process Google OAuth | `server/internal/auth/` |
