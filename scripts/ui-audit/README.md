# UX/UI audit pipeline

Headless-Chromium screenshot capture for VibeTradez. Drives every user-facing
page against the **local Docker stack** and writes full-page JPEGs at two
viewports (desktop 1440×900, mobile 390×844) in **both light and dark themes**:
four shots per page, named `<slug>__<viewport>__<theme>.jpg`. The four
theme × viewport combos capture concurrently.

Output lands in the gitignored `scripts/ui-audit/out/<date>/`: it is
THROWAWAY analysis input for the auditing model, never committed. The model
reads the captures (or drives Playwright directly for behavior), fixes what
it finds, and the PR description carries the findings. See
[`docs/ui-audits/README.md`](../../docs/ui-audits/README.md) for the
convention.

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

`run.sh` waits for `/health`, resolves the dynamic routes from the live
stack (latest transcript date/kind, one holding and one closed trade), prunes
stale captures, then captures into `scripts/ui-audit/out/<today>/`.

## Knobs

All optional, set as env vars before `./run.sh`:

| Var | Default | Purpose |
|-----|---------|---------|
| `BASE_URL` | `http://localhost:3005` | Frontend URL (the override maps 3001→3005). |
| `HEALTH_URL` | `http://localhost:8080/health` | Server health gate. |
| `PGURL` | local stack `:5433` | Only used if you query directly; `run.sh` execs into the container. |
| `AUDIT_DATE` | today (`YYYY-MM-DD`) | Output subdirectory stamp. |
| `OUT_DIR` | `./out/<AUDIT_DATE>` | Output directory (gitignored). |
| `SETTLE_MS` | `1800` | Extra wait after `networkidle` so charts finish rendering. |
| `JPEG_QUALITY` | `82` | JPEG quality for the full-page shots. |
| `SCALE` | `1` | Device scale factor. Bump to `2` for a one-off high-DPI capture when pixel-peeping a glitch. |

## Files

| File | Purpose |
|------|---------|
| `run.sh` | Orchestrator: health gate → resolve dynamic routes → capture. |
| `capture.mjs` | Playwright capture (theme × viewport combos concurrent, routes serial within each). |
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
