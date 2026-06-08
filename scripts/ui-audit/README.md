# UX/UI audit pipeline

Headless-Chromium screenshot capture for VibeTradez. Drives every user-facing
page against the **local Docker stack** and writes full-page PNGs at two
viewports (desktop 1440×900, mobile 390×844) in **both light and dark themes**:
four shots per page, named `<slug>__<viewport>__<theme>.jpg`.

Output lands in `docs/ui-audits/<date>/`, where it is committed alongside the
written audit (`audit.md`) and embedded in the PR. See
[`docs/ui-audits/README.md`](../../docs/ui-audits/README.md) for the audit
convention and the per-PR requirement.

## Run it

```bash
# 1. Boot the local stack with fresh seeded data (from repo root):
cd local
python3 generate-seed.py > seed.sql
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml down -v
docker compose -f docker-compose.local.yml -f docker-compose.local.override.yml up --build -d

# 2. Capture (first run downloads Chromium):
cd ../scripts/ui-audit
npm install
./run.sh
```

`run.sh` waits for `/health`, resolves the dynamic transcript routes from the
seeded Postgres (latest `portfolio_sessions.date` + `transcripts.kind`), then
captures into `docs/ui-audits/<today>/`.

## Knobs

All optional, set as env vars before `./run.sh`:

| Var | Default | Purpose |
|-----|---------|---------|
| `BASE_URL` | `http://localhost:3005` | Frontend URL (the override maps 3001→3005). |
| `HEALTH_URL` | `http://localhost:8080/health` | Server health gate. |
| `PGURL` | local stack `:5433` | Only used if you query directly; `run.sh` execs into the container. |
| `AUDIT_DATE` | today (`YYYY-MM-DD`) | Output subdirectory stamp. |
| `SETTLE_MS` | `1800` | Extra wait after `networkidle` so charts finish rendering. |
| `JPEG_QUALITY` | `82` | JPEG quality for the full-page shots. |
| `SCALE` | `1` | Device scale factor. Bump to `2` for a one-off high-DPI capture when pixel-peeping a glitch. |

## Files

| File | Purpose |
|------|---------|
| `run.sh` | Orchestrator: health gate → resolve dynamic routes → capture. |
| `capture.mjs` | Playwright capture loop (viewport × theme × route). |
| `routes.mjs` | The route manifest, viewport list, theme list. |
| `package.json` | Pins `playwright`; `postinstall` fetches Chromium. |

## Notes

- The `/seo/[slug]` routes are **excluded** — they are noindex 1200×630 OG-card
  capture targets, not browsable pages.
- Themes are forced by seeding `localStorage["theme"]` before app scripts run
  (next-themes, `attribute="class"`), plus matching `prefers-color-scheme`
  emulation, so there is no light→dark flash in the captured frame.
- Motion is reduced (`reducedMotion: "reduce"`) so captures land on the settled
  layout instead of mid-animation.
