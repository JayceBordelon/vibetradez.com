# CLAUDE.md

Operator notes for this repo. Read before touching the trading service.

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
4. Manual smoke: hit the local `/health`, the local dashboard, the local `/trade/[symbol]` page

## Schema migrations live inline

`internal/store/store.go`'s `migrate()` function runs on server boot with `CREATE TABLE IF NOT EXISTS` and `ALTER TABLE ... IF NOT EXISTS` for additive columns. Schema changes go there; no separate migration directory.

## Picker / agent: two-stage by design

This is the load-bearing architectural invariant. Read it before touching any picker, agent, or chain code.

**Pre-bell (9:25 ET) does ticker selection only.** Claude reads sentiment + overnight news + live Schwab equity spot. Output: exactly **3 ranked candidates**, each `{symbol, contract_type, score, thesis, target_otm_pct, min_dte}`. Specific strikes / expirations / estimated prices are NOT chosen because US listed options don't trade pre-market and Schwab's `/chains` endpoint serves yesterday's 4:00 PM close.

**At market open (9:30:00 ET) the agent picks contracts and fires orders.** Same model, re-invoked via `internal/execagent` with a locked-down tool surface: live Schwab quotes, live option chain restricted to the candidate symbols, account funds, web search, and `place_options_order` (the only side-effectful tool — submits one BUY_TO_OPEN LIMIT for a single contract per call). For each candidate the agent reads the now-live chain, picks the strike + expiration + limit price, and either calls `place_options_order` (which writes the execution row + stamps the contract spec on the trade row + submits the order in one atomic step) or declines with a written reason. Skipped candidates land in `executions` with `status='skipped'` and the reason in `error_message`.

**One contract per pick, no sizing up.** The agent buys at most one contract of each of the up-to-3 picks whose setup still holds. Conviction is expressed by which picks it takes, not by how many contracts of one. There is no budget to deploy, and the combined price of the three contracts is never a reason to drop a pick (buy one of each you believe in regardless of the consolidated cost). The agent still skips freely on a deteriorated setup.

Hard caps live at the tool layer in `internal/execagent/tools.go`, enforced even if the system prompt is bypassed:

- `MaxContractsPerOrder = 1` — every order is exactly one contract; the tool refuses any `quantity` above 1. This is the rule that keeps the day to one contract per pick
- `MaxOrdersPerRun = 3` — mirrors candidate basket size, so at most one order per pick
- `MaxToolPremiumPerShare = $10.00` — per-share LIMIT cap (sanity ceiling on a single share price)
- One order per rank (one order per candidate; the tool refuses a second order for the same rank)
- Symbol allowlist (only the 3 morning candidates can be ordered)
- `MaxDailyExposure = $3,000.00` — a defensive outer ceiling on cumulative `limit_price × 100 × quantity`. With one contract per pick, three picks max, and the $10/share cap, the day can never exceed `3 × $10 × 100 = $3,000`, so this never blocks buying one of each; it only matters if the per-share or per-order caps were ever loosened
- `exec.MaxContractPremium = $10.00` (per-share) and `exec.MaxOrderCost = $1,000.00` (per-order total cost) re-validate at the broker entry point so a buggy tool-layer change can't widen a single order

**Don't try to "improve" the picker by giving it the chain pre-bell.** It was that way before and produced wrong orders on overnight-gap names (INTU 2026-05-21: $1.48 stale LIMIT vs $13+ true post-open ask, +$1,180 floor missed). The split exists because pre-bell chain data is fossilized; the picker prompt now explicitly tells Claude this.

The 9:30 agent run is fire-and-forget. The per-minute reconcile cron flips fills through the morning, the 9:35 cron cancels any still-WORKING LIMITs and sends ONE consolidated execution-summary email covering every candidate (buys with fill prices, skips with reasons).

## Model version refresh policy

The picker model is configured via `ANTHROPIC_MODEL` env var with the default in `server/internal/config/config.go` as `DefaultAnthropicModel`.

Any time you touch the trade picker or this default, fetch the official Anthropic Go SDK documentation and refresh the default to the current latest production model. Anthropic publishes new model versions regularly; a stale default degrades trade quality silently.

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
| Cron registration (9:25, 9:30:00, 9:35, 15:55, 16:00, 16:30 Fri) | `server/cmd/scanner/main.go` |
| Market open / holiday / half-day gating | `server/internal/calendar/calendar.go` |
| Signal scraping (4 sources) | `server/internal/sentiment/scraper.go` |
| 9:25 picker tool loop + Anthropic SDK wiring | `server/internal/trades/picker.go` |
| Picker prompts (AnalysisPrompt, EndOfDayPrompt) | `server/internal/trades/prompt.go` |
| 9:30 at-open agent (conversation loop, final-JSON reconciliation) | `server/internal/execagent/agent.go` |
| Agent tool schemas + validation + hard caps | `server/internal/execagent/tools.go` |
| Agent system prompt | `server/internal/execagent/prompt.go` |
| Broker entry points (`PlaceBuyToOpenAgent`, `InsertSkippedExecutionAgent`, `AvailableFundsAgent`) + reconcile + cancel-dangling + close-all + summary emails | `server/internal/exec/service.go` |
| Email templates (morning, basket summary, close summary, EOD, weekly, error) | `server/internal/templates/` |
| Rollout v8 announcement (agent execution) | `server/internal/rollouts/v8_agent_executes.go` + `server/internal/templates/rollout_agent_executes.html` |
