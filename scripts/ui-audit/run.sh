#!/usr/bin/env bash
#
# One-shot UX/UI audit capture against the local Docker stack.
#
# Boots nothing on its own — assumes the local stack is already running on
# $BASE_URL (default http://localhost:3005). It:
#   1. waits for /health to be green,
#   2. resolves the dynamic routes (latest transcript date + kind) from the
#      seeded Postgres so /transcripts/<date> and /transcript/<date>/<kind>
#      point at real data,
#   3. runs capture.mjs to write desktop+mobile, light+dark PNGs into
#      docs/ui-audits/<date>/.
#
# Usage:
#   cd vibetradez.com/local && docker compose -f docker-compose.local.yml \
#       -f docker-compose.local.override.yml up --build -d
#   cd ../scripts/ui-audit && npm install && ./run.sh
#
# Env overrides:
#   BASE_URL    frontend URL              (default http://localhost:3005)
#   HEALTH_URL  server health URL         (default http://localhost:8080/health)
#   PGURL       seeded Postgres conn      (default local stack on :5433)
#   AUDIT_DATE  output dir date stamp     (default: today, YYYY-MM-DD)
set -euo pipefail
cd "$(dirname "$0")"

BASE_URL="${BASE_URL:-http://localhost:3005}"
HEALTH_URL="${HEALTH_URL:-http://localhost:8080/health}"
PGURL="${PGURL:-postgresql://vibetradez:vibetradez@localhost:5433/vibetradez}"
AUDIT_DATE="${AUDIT_DATE:-$(date +%F)}"
OUT_DIR="$(cd ../.. && pwd)/docs/ui-audits/${AUDIT_DATE}"

echo "==> waiting for server health at ${HEALTH_URL}"
for i in $(seq 1 40); do
  if curl -fsS "${HEALTH_URL}" >/dev/null 2>&1; then echo "    healthy"; break; fi
  if [ "$i" = 40 ]; then echo "    server never became healthy" >&2; exit 1; fi
  sleep 2
done

echo "==> resolving dynamic routes from seeded Postgres"
# psql via the running postgres container so a local psql client isn't required.
# `date` is stored as ISO text ('YYYY-MM-DD'), so max() orders it correctly and
# no to_char cast is needed. `|| true` keeps a transient miss from tripping set -e.
q() { docker exec vt-local-postgres psql -U vibetradez -d vibetradez -tAc "$1" 2>/dev/null | tr -d '[:space:]' || true; }
TX_DATE="$(q "SELECT max(date) FROM portfolio_sessions;")"
TX_KIND="portfolio"
if [ -z "${TX_DATE}" ]; then
  echo "    WARN: no portfolio_sessions rows; transcript routes will be skipped" >&2
fi
echo "    transcript date=${TX_DATE:-<none>} kind=${TX_KIND}"

# Build the resolved route list as JSON for capture.mjs.
AUDIT_ROUTES="$(
  TX_DATE="${TX_DATE}" TX_KIND="${TX_KIND}" node -e '
    const d = process.env.TX_DATE, k = process.env.TX_KIND;
    const routes = [
      { slug: "landing",     label: "Landing",           path: "/" },
      { slug: "dashboard",   label: "Dashboard",          path: "/dashboard" },
      { slug: "holdings",    label: "Holdings",           path: "/holdings" },
      { slug: "closed",      label: "Closed",             path: "/closed" },
      d && { slug: "transcripts", label: "Transcripts index", path: `/transcripts/${d}` },
      d && { slug: "transcript",  label: "Session transcript", path: `/transcript/${d}/${k}` },
      { slug: "faq",         label: "FAQ",                path: "/faq" },
      { slug: "terms",       label: "Terms",              path: "/terms" },
      { slug: "privacy",     label: "Privacy",            path: "/privacy" },
    ].filter(Boolean);
    process.stdout.write(JSON.stringify(routes));
  '
)"

echo "==> capturing screenshots into ${OUT_DIR}"
BASE_URL="${BASE_URL}" OUT_DIR="${OUT_DIR}" AUDIT_ROUTES="${AUDIT_ROUTES}" node capture.mjs

echo "==> done. PNGs in ${OUT_DIR}"
